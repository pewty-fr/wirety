import { screen, waitFor, within } from '@testing-library/react';
import type { UserEvent } from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { server } from '../../test/setup';
import { db, requestsTo, seedNetwork, seedPeer, seedRoute } from '../../test/fakeApi';
import { findDialog, pick } from '../../test/helpers';
import { renderApp } from '../../test/renderApp';
import type { DNSMapping } from '../../types';

function seedRecord(routeId: string, over: Partial<DNSMapping> = {}): DNSMapping {
  const record: DNSMapping = {
    id: `rec-${db.dns.length + 1}`,
    route_id: routeId,
    name: 'nas',
    created_at: '2026-10-01T10:00:00Z',
    updated_at: '2026-10-01T10:00:00Z',
    ...over,
  };
  db.dns.push(record);
  return record;
}

function seedCorp() {
  const net = seedNetwork({ name: 'corp', domain_suffix: 'internal' });
  const jump = seedPeer(net.id, { name: 'jump-paris', is_jump: true });
  const dc = seedRoute(net.id, {
    name: 'datacenter',
    destination_cidr: '10.0.0.0/16',
    destination_cidr_v6: 'fd00:10::/64',
    jump_peer_id: jump.id,
  });
  const office = seedRoute(net.id, { name: 'office', destination_cidr: '192.168.1.0/24', jump_peer_id: jump.id });
  return { net, jump, dc, office };
}

const creations = () => requestsTo('POST', /\/dns$/);

async function openNewRecord(user: UserEvent) {
  // Enabled once the routes of the network are known.
  const button = (await screen.findAllByRole('button', { name: 'New record' }))[0];
  await waitFor(() => expect(button).toBeEnabled());
  await user.click(button);
  return findDialog('New DNS record');
}

describe('DNS records', () => {
  it('lists the records of the first network with their full name', async () => {
    const { dc, office } = seedCorp();
    seedRecord(dc.id, { name: 'nas', ip_address: '10.0.0.50', ip_address_v6: 'fd00:10::50' });
    seedRecord(office.id, { name: 'printer', ip_address: '192.168.1.20' });
    renderApp('/dns');

    const table = await screen.findByRole('table');
    expect(screen.getByRole('combobox', { name: 'Network' })).toHaveValue('corp');
    expect(within(table).getByText('nas.corp.internal')).toBeInTheDocument();
    expect(within(table).getByText('10.0.0.50')).toBeInTheDocument();
    expect(within(table).getByText('fd00:10::50')).toBeInTheDocument();
    expect(within(table).getByText('datacenter')).toBeInTheDocument();
    expect(within(table).getByText('printer.corp.internal')).toBeInTheDocument();
    expect(within(table).getByText('office')).toBeInTheDocument();
  });

  it('shows the records of the network picked', async () => {
    const { dc } = seedCorp();
    seedRecord(dc.id, { name: 'nas', ip_address: '10.0.0.50' });
    const lab = seedNetwork({ name: 'lab', domain_suffix: 'test' });
    const labRoute = seedRoute(lab.id, { name: 'bench', destination_cidr: '10.9.0.0/24' });
    seedRecord(labRoute.id, { name: 'scope', ip_address: '10.9.0.7' });
    const { user } = renderApp('/dns');
    await screen.findByText('nas.corp.internal');

    await pick(user, document.body, 'Network', 'lab');

    expect(await screen.findByText('scope.lab.test')).toBeInTheDocument();
    expect(screen.queryByText('nas.corp.internal')).not.toBeInTheDocument();
  });

  it('creates a record', async () => {
    const { net, dc } = seedCorp();
    const { user } = renderApp('/dns');

    const dialog = await openNewRecord(user);
    await pick(user, dialog, 'Route', 'datacenter');
    await user.type(within(dialog).getByLabelText(/^Name/), 'nas');
    expect(within(dialog).getByText('nas.corp.internal')).toBeInTheDocument();
    await user.type(within(dialog).getByLabelText('IPv4 address'), '10.0.0.50');
    await user.type(within(dialog).getByLabelText('IPv6 address'), 'fd00:10::50');

    await user.click(within(dialog).getByRole('button', { name: 'Create record' }));

    expect(await screen.findByText('DNS record “nas.corp.internal” created')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New DNS record' })).not.toBeInTheDocument());
    expect(creations()).toEqual([
      {
        method: 'POST',
        path: `/networks/${net.id}/routes/${dc.id}/dns`,
        body: { name: 'nas', ip_address: '10.0.0.50', ip_address_v6: 'fd00:10::50' },
      },
    ]);
    const table = await screen.findByRole('table');
    expect(within(table).getByText('nas.corp.internal')).toBeInTheDocument();
  });

  it('creates a wildcard record with a single address', async () => {
    const { net, office } = seedCorp();
    seedRecord(office.id, { name: 'printer', ip_address: '192.168.1.20' });
    const { user } = renderApp('/dns');

    const dialog = await openNewRecord(user);
    await pick(user, dialog, 'Route', 'office');
    await user.type(within(dialog).getByLabelText(/^Name/), '*.apps');
    expect(within(dialog).getByText('*.apps.corp.internal')).toBeInTheDocument();
    await user.type(within(dialog).getByLabelText('IPv4 address'), '192.168.1.80');
    await user.click(within(dialog).getByRole('button', { name: 'Create record' }));

    await screen.findByText('DNS record “*.apps.corp.internal” created');
    expect(creations()[0].path).toBe(`/networks/${net.id}/routes/${office.id}/dns`);
    expect(creations()[0].body).toEqual({ name: '*.apps', ip_address: '192.168.1.80' });
  });

  it('checks the form before sending anything', async () => {
    seedCorp();
    const { user } = renderApp('/dns');
    const dialog = await openNewRecord(user);

    await user.click(within(dialog).getByRole('button', { name: 'Create record' }));

    expect(within(dialog).getByText('Choose the route the addresses are behind.')).toBeInTheDocument();
    expect(within(dialog).getByText('Give the record a name.')).toBeInTheDocument();
    expect(within(dialog).getByText('Enter an IPv4 or an IPv6 address, or both.')).toBeInTheDocument();
    expect(within(dialog).getByLabelText(/^Name/)).toHaveAttribute('aria-invalid', 'true');

    await pick(user, dialog, 'Route', 'office');
    await user.type(within(dialog).getByLabelText(/^Name/), 'my_nas');
    await user.type(within(dialog).getByLabelText('IPv4 address'), '10.0.0.300');
    await user.type(within(dialog).getByLabelText('IPv6 address'), 'fd00::50');
    await user.click(within(dialog).getByRole('button', { name: 'Create record' }));

    expect(
      within(dialog).getByText('Use letters, digits and hyphens (not at the start or end), or * for a wildcard.'),
    ).toBeInTheDocument();
    expect(within(dialog).getByText('Enter an IPv4 address, e.g. 10.0.0.50.')).toBeInTheDocument();
    // The office route has no IPv6 destination.
    expect(within(dialog).getByText('Route “office” has no IPv6 destination.')).toBeInTheDocument();

    await user.clear(within(dialog).getByLabelText('IPv6 address'));
    await user.type(within(dialog).getByLabelText('IPv6 address'), 'fd00::zz');
    await user.click(within(dialog).getByRole('button', { name: 'Create record' }));
    expect(within(dialog).getByText('Enter an IPv6 address, e.g. fd00::50.')).toBeInTheDocument();
    expect(creations()).toHaveLength(0);
  });

  it('shows the server error in the dialog', async () => {
    seedCorp();
    server.use(
      http.post('*/api/v1/networks/:id/routes/:routeId/dns', () =>
        HttpResponse.json({ error: 'ip_address validation failed: IP 10.9.0.1 is not within CIDR 10.0.0.0/16' }, { status: 400 }),
      ),
    );
    const { user } = renderApp('/dns');
    const dialog = await openNewRecord(user);
    await pick(user, dialog, 'Route', 'datacenter');
    await user.type(within(dialog).getByLabelText(/^Name/), 'nas');
    await user.type(within(dialog).getByLabelText('IPv4 address'), '10.9.0.1');

    await user.click(within(dialog).getByRole('button', { name: 'Create record' }));

    expect(await within(dialog).findByRole('alert')).toHaveTextContent('IP 10.9.0.1 is not within CIDR 10.0.0.0/16');
    expect(screen.getByRole('dialog', { name: 'New DNS record' })).toBeInTheDocument();
  });

  it('closes the form without sending anything on cancel', async () => {
    seedCorp();
    const { user } = renderApp('/dns');
    const dialog = await openNewRecord(user);
    await pick(user, dialog, 'Route', 'datacenter');
    await user.type(within(dialog).getByLabelText(/^Name/), 'nas');

    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));

    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New DNS record' })).not.toBeInTheDocument());
    expect(creations()).toHaveLength(0);
  });

  it('edits a record, which stays on its route', async () => {
    const { net, dc } = seedCorp();
    const record = seedRecord(dc.id, { name: 'nas', ip_address: '10.0.0.50', ip_address_v6: 'fd00:10::50' });
    const { user } = renderApp('/dns');

    await user.click(await screen.findByRole('button', { name: 'Edit nas.corp.internal' }));
    const dialog = await findDialog('Edit DNS record');
    const route = within(dialog).getByRole('combobox', { name: 'Route' });
    expect(route).toBeDisabled();
    expect(route).toHaveValue('datacenter');
    await user.clear(within(dialog).getByLabelText(/^Name/));
    await user.type(within(dialog).getByLabelText(/^Name/), 'storage');
    expect(within(dialog).getByText('storage.corp.internal')).toBeInTheDocument();
    await user.clear(within(dialog).getByLabelText('IPv4 address'));
    await user.type(within(dialog).getByLabelText('IPv4 address'), '10.0.0.51');

    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

    await screen.findByText('DNS record “storage.corp.internal” saved');
    expect(requestsTo('PUT', /\/dns\//)).toEqual([
      {
        method: 'PUT',
        path: `/networks/${net.id}/routes/${dc.id}/dns/${record.id}`,
        body: { name: 'storage', ip_address: '10.0.0.51', ip_address_v6: 'fd00:10::50' },
      },
    ]);
    expect(within(await screen.findByRole('table')).getByText('storage.corp.internal')).toBeInTheDocument();
  });

  it('removes an address from a record', async () => {
    const { dc } = seedCorp();
    const record = seedRecord(dc.id, { name: 'nas', ip_address: '10.0.0.50', ip_address_v6: 'fd00:10::50' });
    const { user } = renderApp('/dns');

    await user.click(await screen.findByRole('button', { name: 'Edit nas.corp.internal' }));
    const dialog = await findDialog('Edit DNS record');
    await user.clear(within(dialog).getByLabelText('IPv6 address'));
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

    await screen.findByText('DNS record “nas.corp.internal” saved');
    // An address sent empty is removed by the server.
    expect(requestsTo('PUT', /\/dns\//)[0].body).toEqual({ name: 'nas', ip_address: '10.0.0.50', ip_address_v6: '' });
    expect(db.dns.find((d) => d.id === record.id)).toMatchObject({ ip_address: '10.0.0.50', ip_address_v6: '' });
  });

  it('shows the server error of an edit in the dialog', async () => {
    const { dc } = seedCorp();
    seedRecord(dc.id, { name: 'nas', ip_address: '10.0.0.50' });
    server.use(
      http.put('*/api/v1/networks/:id/routes/:routeId/dns/:dnsId', () =>
        HttpResponse.json({ error: 'a record with this name already exists' }, { status: 409 }),
      ),
    );
    const { user } = renderApp('/dns');

    await user.click(await screen.findByRole('button', { name: 'Edit nas.corp.internal' }));
    const dialog = await findDialog('Edit DNS record');
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

    expect(await within(dialog).findByRole('alert')).toHaveTextContent('a record with this name already exists');
  });

  it('deletes a record after confirmation', async () => {
    const { net, dc } = seedCorp();
    const record = seedRecord(dc.id, { name: 'nas', ip_address: '10.0.0.50', ip_address_v6: 'fd00:10::50' });
    const { user } = renderApp('/dns');

    await user.click(await screen.findByRole('button', { name: 'Delete nas.corp.internal' }));
    let confirm = await findDialog('Delete DNS record “nas.corp.internal”?');
    expect(within(confirm).getByText('nas.corp.internal → 10.0.0.50, fd00:10::50')).toBeInTheDocument();
    await user.click(within(confirm).getByRole('button', { name: 'Cancel' }));
    expect(requestsTo('DELETE', /\/dns\//)).toHaveLength(0);

    await user.click(screen.getByRole('button', { name: 'Delete nas.corp.internal' }));
    confirm = await findDialog('Delete DNS record “nas.corp.internal”?');
    await user.click(within(confirm).getByRole('button', { name: 'Delete record' }));

    await screen.findByText('DNS record “nas.corp.internal” deleted');
    expect(requestsTo('DELETE', /\/dns\//)[0].path).toBe(`/networks/${net.id}/routes/${dc.id}/dns/${record.id}`);
    expect(await screen.findByText('No DNS record yet')).toBeInTheDocument();
  });

  it('reports a failed deletion', async () => {
    const { dc } = seedCorp();
    seedRecord(dc.id, { name: 'nas', ip_address: '10.0.0.50' });
    server.use(
      http.delete('*/api/v1/networks/:id/routes/:routeId/dns/:dnsId', () =>
        HttpResponse.json({ error: 'DNS mapping not found' }, { status: 404 }),
      ),
    );
    const { user } = renderApp('/dns');

    await user.click(await screen.findByRole('button', { name: 'Delete nas.corp.internal' }));
    await user.click(
      within(await findDialog('Delete DNS record “nas.corp.internal”?')).getByRole('button', { name: 'Delete record' }),
    );

    expect(await screen.findByText('DNS mapping not found')).toBeInTheDocument();
  });

  it('invites to create the first record', async () => {
    seedCorp();
    renderApp('/dns');
    expect(await screen.findByText('No DNS record yet')).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'New record' })[0]).toBeEnabled();
  });

  it('explains that a route is needed first', async () => {
    seedNetwork({ name: 'corp' });
    renderApp('/dns');
    expect(await screen.findByText('No route in corp')).toBeInTheDocument();
    expect(screen.getByText(/create a route in this network first/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'New record' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Go to routes' })).toBeInTheDocument();
  });

  it('explains that a network is needed first', async () => {
    renderApp('/dns');
    expect(await screen.findByText('No network yet')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'New record' })).toBeDisabled();
  });

  it('is reserved to administrators', async () => {
    db.me = { ...db.me, role: 'user' };
    renderApp('/dns');
    expect(await screen.findByText('Only administrators can manage DNS records.')).toBeInTheDocument();
  });
});
