import { screen, waitFor, within } from '@testing-library/react';
import type { UserEvent } from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { server } from '../../test/setup';
import { db, requestsTo, seedGroup, seedNetwork, seedPeer, seedRoute } from '../../test/fakeApi';
import { findDialog, openTab, pick } from '../../test/helpers';
import { renderApp } from '../../test/renderApp';

function seedCorp() {
  const net = seedNetwork({ name: 'corp' });
  const paris = seedPeer(net.id, { name: 'jump-paris', is_jump: true });
  const lyon = seedPeer(net.id, { name: 'jump-lyon', is_jump: true });
  const alice = seedPeer(net.id, { name: 'alice-laptop' });
  const engineering = seedGroup(net.id, { name: 'engineering', peer_ids: [alice.id] });
  const ops = seedGroup(net.id, { name: 'ops' });
  // jump-paris is a member: a route through it cannot be given to this group.
  const gateways = seedGroup(net.id, { name: 'gateways', peer_ids: [paris.id] });
  return { net, paris, lyon, alice, engineering, ops, gateways };
}

async function openNewRoute(user: UserEvent) {
  // The header and the empty state both offer it.
  await user.click((await screen.findAllByRole('button', { name: 'New route' }))[0]);
  return findDialog('New route');
}

const routeCreations = () => requestsTo('POST', /\/routes$/);
const groupAttachments = () => requestsTo('POST', /\/groups\/[^/]+\/routes\//);

describe('Routes', () => {
  it('lists the routes of every network with the name of their jump peer', async () => {
    const { net, paris } = seedCorp();
    const lab = seedNetwork({ name: 'lab', cidr: '10.43.0.0/24' });
    const labJump = seedPeer(lab.id, { name: 'jump-lab', is_jump: true });
    seedRoute(net.id, {
      name: 'datacenter',
      description: 'Paris datacenter',
      destination_cidr: '10.0.0.0/16',
      destination_cidr_v6: 'fd00:10::/64',
      jump_peer_id: paris.id,
    });
    seedRoute(lab.id, { name: 'office', destination_cidr: '192.168.1.0/24', jump_peer_id: labJump.id });
    renderApp('/routes');

    const table = await screen.findByRole('table');
    expect(within(table).getByText('datacenter')).toBeInTheDocument();
    expect(within(table).getByText('Paris datacenter')).toBeInTheDocument();
    expect(within(table).getByText('10.0.0.0/16')).toBeInTheDocument();
    expect(within(table).getByText('fd00:10::/64')).toBeInTheDocument();
    expect(within(table).getByText('office')).toBeInTheDocument();
    expect(within(table).getByText('lab')).toBeInTheDocument();
    // Jump peers are resolved in each route's own network.
    expect(await within(table).findByText('jump-paris')).toBeInTheDocument();
    expect(await within(table).findByText('jump-lab')).toBeInTheDocument();
  });

  it('filters the list by network', async () => {
    const { net, paris } = seedCorp();
    const lab = seedNetwork({ name: 'lab' });
    seedRoute(net.id, { name: 'datacenter', jump_peer_id: paris.id });
    seedRoute(lab.id, { name: 'office' });
    const { user } = renderApp('/routes');
    await screen.findByText('office');

    await pick(user, document.body, 'Network', 'lab');

    await waitFor(() => expect(screen.queryByText('datacenter')).not.toBeInTheDocument());
    expect(screen.getByText('office')).toBeInTheDocument();
  });

  it('creates a route attached to its groups in one go', async () => {
    const { net, paris, engineering, ops } = seedCorp();
    const { user } = renderApp('/routes');

    const dialog = await openNewRoute(user);
    await pick(user, dialog, 'Network', 'corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'datacenter');
    await user.type(within(dialog).getByLabelText('Description'), 'Paris datacenter');
    await user.type(within(dialog).getByLabelText('IPv4 destination'), '10.0.0.0/16');
    await user.type(within(dialog).getByLabelText('IPv6 destination'), 'fd00:10::/64');
    await pick(user, dialog, 'Jump peer', 'jump-paris');
    expect(within(dialog).getByLabelText('Domain suffix')).toHaveValue('internal');
    await openTab(user, dialog, 'Groups');
    await pick(user, dialog, 'Add a group', 'engineering');
    await pick(user, dialog, 'Add a group', 'ops');
    expect(within(dialog).getByRole('tab', { name: /^Groups/ })).toHaveTextContent('2');

    await user.click(within(dialog).getByRole('button', { name: 'Create route' }));

    expect(await screen.findByText('Route “datacenter” created')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New route' })).not.toBeInTheDocument());
    expect(routeCreations()).toEqual([
      {
        method: 'POST',
        path: `/networks/${net.id}/routes`,
        body: {
          name: 'datacenter',
          description: 'Paris datacenter',
          destination_cidr: '10.0.0.0/16',
          destination_cidr_v6: 'fd00:10::/64',
          jump_peer_id: paris.id,
          domain_suffix: 'internal',
        },
      },
    ]);
    const route = db.routes.find((r) => r.name === 'datacenter')!;
    expect(groupAttachments().map((r) => r.path)).toEqual([
      `/networks/${net.id}/groups/${engineering.id}/routes/${route.id}`,
      `/networks/${net.id}/groups/${ops.id}/routes/${route.id}`,
    ]);
    expect(db.groups.find((g) => g.id === engineering.id)!.route_ids).toEqual([route.id]);
    const table = await screen.findByRole('table');
    expect(within(table).getByText('datacenter')).toBeInTheDocument();
    expect(await within(table).findByText('jump-paris')).toBeInTheDocument();
  });

  it('creates an IPv6-only route with its own domain suffix', async () => {
    const { net, lyon } = seedCorp();
    const { user } = renderApp('/routes');

    const dialog = await openNewRoute(user);
    await pick(user, dialog, 'Network', 'corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'lab-v6');
    await user.type(within(dialog).getByLabelText('IPv6 destination'), 'fd00:20::/48');
    await pick(user, dialog, 'Jump peer', 'jump-lyon');
    await user.clear(within(dialog).getByLabelText('Domain suffix'));
    await user.type(within(dialog).getByLabelText('Domain suffix'), 'lab.corp');
    await user.click(within(dialog).getByRole('button', { name: 'Create route' }));

    await screen.findByText('Route “lab-v6” created');
    expect(routeCreations()[0].body).toEqual({
      name: 'lab-v6',
      description: '',
      destination_cidr_v6: 'fd00:20::/48',
      jump_peer_id: lyon.id,
      domain_suffix: 'lab.corp',
    });
    expect(routeCreations()[0].path).toBe(`/networks/${net.id}/routes`);
    expect(groupAttachments()).toHaveLength(0);
  });

  it('checks the form before sending anything', async () => {
    seedCorp();
    const { user } = renderApp('/routes');
    const dialog = await openNewRoute(user);

    await user.click(within(dialog).getByRole('button', { name: 'Create route' }));

    expect(within(dialog).getByText('Choose the network of the route.')).toBeInTheDocument();
    expect(within(dialog).getByText('Give the route a name.')).toBeInTheDocument();
    expect(within(dialog).getByText('Enter an IPv4 or an IPv6 CIDR, or both.')).toBeInTheDocument();
    expect(within(dialog).getByText('Choose the jump peer the traffic goes through.')).toBeInTheDocument();
    expect(within(dialog).getByLabelText(/^Name/)).toHaveAttribute('aria-invalid', 'true');
    expect(routeCreations()).toHaveLength(0);
  });

  it('checks the format of the destinations and of the domain suffix', async () => {
    seedCorp();
    const { user } = renderApp('/routes');
    const dialog = await openNewRoute(user);
    await pick(user, dialog, 'Network', 'corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'datacenter');
    await pick(user, dialog, 'Jump peer', 'jump-paris');
    await user.type(within(dialog).getByLabelText('IPv4 destination'), '10.0.0.1/16');
    await user.type(within(dialog).getByLabelText('IPv6 destination'), '10.1.0.0/16');
    await user.clear(within(dialog).getByLabelText('Domain suffix'));
    await user.type(within(dialog).getByLabelText('Domain suffix'), 'my_domain');

    await user.click(within(dialog).getByRole('button', { name: 'Create route' }));

    expect(within(dialog).getByText('10.0.0.1 has host bits set — did you mean 10.0.0.0/16?')).toBeInTheDocument();
    expect(within(dialog).getByText('This is an IPv4 CIDR: enter it in the IPv4 field.')).toBeInTheDocument();
    expect(
      within(dialog).getByText('Use letters, digits, dots and hyphens, not at the start or end.'),
    ).toBeInTheDocument();
    expect(within(dialog).getByLabelText('IPv4 destination')).toHaveAttribute('aria-invalid', 'true');
    expect(routeCreations()).toHaveLength(0);

    // Fixed, the same form goes through.
    await user.clear(within(dialog).getByLabelText('IPv4 destination'));
    await user.type(within(dialog).getByLabelText('IPv4 destination'), '10.0.0.0/16');
    await user.clear(within(dialog).getByLabelText('IPv6 destination'));
    await user.clear(within(dialog).getByLabelText('Domain suffix'));
    await user.click(within(dialog).getByRole('button', { name: 'Create route' }));
    await screen.findByText('Route “datacenter” created');
    expect(routeCreations()[0].body).toMatchObject({ destination_cidr: '10.0.0.0/16', domain_suffix: 'internal' });
  });

  it('refuses to attach a route to a group its jump peer belongs to', async () => {
    seedCorp();
    const { user } = renderApp('/routes');
    const dialog = await openNewRoute(user);
    await pick(user, dialog, 'Network', 'corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'datacenter');
    await user.type(within(dialog).getByLabelText('IPv4 destination'), '10.0.0.0/16');
    await pick(user, dialog, 'Jump peer', 'jump-paris');
    await openTab(user, dialog, 'Groups');
    await pick(user, dialog, 'Add a group', 'gateways');
    expect(within(dialog).getByText('Has the jump peer')).toBeInTheDocument();

    await user.click(within(dialog).getByRole('button', { name: 'Create route' }));

    expect(await within(dialog).findByRole('alert')).toHaveTextContent(
      'A route cannot be attached to a group its jump peer belongs to: “jump-paris” is a member of “gateways”.',
    );
    expect(routeCreations()).toHaveLength(0);
  });

  it('refuses to move the jump peer of a route into one of its groups', async () => {
    const { net, lyon, gateways } = seedCorp();
    const route = seedRoute(net.id, { name: 'datacenter', jump_peer_id: lyon.id });
    gateways.route_ids.push(route.id);
    const { user } = renderApp('/routes');

    await user.click(await screen.findByRole('button', { name: 'Edit datacenter' }));
    const dialog = await findDialog('Edit route datacenter');
    await pick(user, dialog, 'Jump peer', 'jump-paris');
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

    expect(await within(dialog).findByRole('alert')).toHaveTextContent('“jump-paris” is a member of “gateways”');
    expect(requestsTo('PUT', /\/routes\//)).toHaveLength(0);
  });

  it('shows the server error in the dialog', async () => {
    seedCorp();
    server.use(
      http.post('*/api/v1/networks/:id/routes', () =>
        HttpResponse.json({ error: 'a route with this name already exists' }, { status: 409 }),
      ),
    );
    const { user } = renderApp('/routes');
    const dialog = await openNewRoute(user);
    await pick(user, dialog, 'Network', 'corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'datacenter');
    await user.type(within(dialog).getByLabelText('IPv4 destination'), '10.0.0.0/16');
    await pick(user, dialog, 'Jump peer', 'jump-paris');

    await user.click(within(dialog).getByRole('button', { name: 'Create route' }));

    expect(await within(dialog).findByRole('alert')).toHaveTextContent('a route with this name already exists');
    expect(screen.getByRole('dialog', { name: 'New route' })).toBeInTheDocument();
  });

  it('keeps editing the created route when a group attachment fails, instead of creating it twice', async () => {
    const { engineering } = seedCorp();
    server.use(
      http.post('*/api/v1/networks/:id/groups/:groupId/routes/:routeId', () =>
        HttpResponse.json({ error: 'group is locked' }, { status: 409 }),
      ),
    );
    const { user } = renderApp('/routes');
    let dialog = await openNewRoute(user);
    await pick(user, dialog, 'Network', 'corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'datacenter');
    await user.type(within(dialog).getByLabelText('IPv4 destination'), '10.0.0.0/16');
    await pick(user, dialog, 'Jump peer', 'jump-paris');
    await openTab(user, dialog, 'Groups');
    await pick(user, dialog, 'Add a group', 'engineering');

    await user.click(within(dialog).getByRole('button', { name: 'Create route' }));

    dialog = await findDialog('Edit route datacenter');
    expect(within(dialog).getByRole('alert')).toHaveTextContent(
      'The route was created, but not attached to all its groups: group is locked',
    );
    // A second submit saves the same route.
    server.resetHandlers();
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));
    await screen.findByText('Route “datacenter” saved');
    expect(routeCreations()).toHaveLength(1);
    expect(requestsTo('PUT', /\/routes\//)).toHaveLength(0);
    const route = db.routes.find((r) => r.name === 'datacenter')!;
    expect(db.groups.find((g) => g.id === engineering.id)!.route_ids).toEqual([route.id]);
  });

  it('edits a route: details and groups are saved together', async () => {
    const { net, paris, lyon, engineering, ops } = seedCorp();
    const route = seedRoute(net.id, {
      name: 'datacenter',
      description: 'Paris datacenter',
      destination_cidr: '10.0.0.0/16',
      jump_peer_id: paris.id,
    });
    engineering.route_ids.push(route.id);
    const { user } = renderApp('/routes');

    await user.click(await screen.findByRole('button', { name: 'Edit datacenter' }));
    const dialog = await findDialog('Edit route datacenter');
    expect(within(dialog).queryByRole('combobox', { name: 'Network' })).not.toBeInTheDocument();
    expect(within(dialog).getByLabelText('Description')).toHaveValue('Paris datacenter');
    await user.clear(within(dialog).getByLabelText(/^Name/));
    await user.type(within(dialog).getByLabelText(/^Name/), 'dc-lyon');
    await user.clear(within(dialog).getByLabelText('IPv4 destination'));
    await user.type(within(dialog).getByLabelText('IPv4 destination'), '10.1.0.0/16');
    await user.type(within(dialog).getByLabelText('IPv6 destination'), 'fd00:10::/64');
    await pick(user, dialog, 'Jump peer', 'jump-lyon');
    await openTab(user, dialog, 'Groups');
    await user.click(await within(dialog).findByRole('button', { name: 'Remove engineering' }));
    await pick(user, dialog, 'Add a group', 'ops');

    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

    await screen.findByText('Route “dc-lyon” saved');
    // Only what changed is sent.
    expect(requestsTo('PUT', /\/routes\//)).toEqual([
      {
        method: 'PUT',
        path: `/networks/${net.id}/routes/${route.id}`,
        body: { name: 'dc-lyon', destination_cidr: '10.1.0.0/16', destination_cidr_v6: 'fd00:10::/64', jump_peer_id: lyon.id },
      },
    ]);
    expect(requestsTo('DELETE', /\/groups\/[^/]+\/routes\//).map((r) => r.path)).toEqual([
      `/networks/${net.id}/groups/${engineering.id}/routes/${route.id}`,
    ]);
    expect(groupAttachments().map((r) => r.path)).toEqual([`/networks/${net.id}/groups/${ops.id}/routes/${route.id}`]);
    expect(db.groups.find((g) => g.id === engineering.id)!.route_ids).toEqual([]);
    expect(db.groups.find((g) => g.id === ops.id)!.route_ids).toEqual([route.id]);
  });

  it('sends nothing when an edited route is saved unchanged', async () => {
    const { net, paris, engineering } = seedCorp();
    const route = seedRoute(net.id, { name: 'datacenter', jump_peer_id: paris.id });
    engineering.route_ids.push(route.id);
    const { user } = renderApp('/routes');

    await user.click(await screen.findByRole('button', { name: 'Edit datacenter' }));
    const dialog = await findDialog('Edit route datacenter');
    expect(await within(dialog).findByRole('tab', { name: /^Groups/ })).toHaveTextContent('1');
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

    await screen.findByText('Route “datacenter” saved');
    expect(requestsTo('PUT', /\/routes\//)).toHaveLength(0);
    expect(requestsTo('POST', /\/groups\//)).toHaveLength(0);
    expect(requestsTo('DELETE', /\/groups\//)).toHaveLength(0);
  });

  it('removes a destination and clears the description of a route', async () => {
    const { net, paris } = seedCorp();
    const route = seedRoute(net.id, {
      name: 'datacenter',
      description: 'Paris DC',
      destination_cidr: '10.0.0.0/16',
      destination_cidr_v6: 'fd00:10::/64',
      jump_peer_id: paris.id,
    });
    const { user } = renderApp('/routes');

    await user.click(await screen.findByRole('button', { name: 'Edit datacenter' }));
    const dialog = await findDialog('Edit route datacenter');
    await user.clear(within(dialog).getByLabelText('IPv6 destination'));
    await user.clear(within(dialog).getByLabelText('Description'));
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

    await screen.findByText('Route “datacenter” saved');
    // Fields sent empty are cleared by the server.
    expect(requestsTo('PUT', /\/routes\//)[0].body).toEqual({ description: '', destination_cidr_v6: '' });
    const saved = db.routes.find((r) => r.id === route.id)!;
    expect(saved).toMatchObject({ description: '', destination_cidr: '10.0.0.0/16', destination_cidr_v6: '' });
  });

  it('closes the form without sending anything on cancel', async () => {
    seedCorp();
    const { user } = renderApp('/routes');
    const dialog = await openNewRoute(user);
    await pick(user, dialog, 'Network', 'corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'datacenter');

    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));

    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New route' })).not.toBeInTheDocument());
    expect(routeCreations()).toHaveLength(0);
  });

  it('deletes a route after confirmation, in its own network', async () => {
    const { net, paris } = seedCorp();
    const route = seedRoute(net.id, { name: 'datacenter', jump_peer_id: paris.id });
    const { user } = renderApp('/routes');

    await user.click(await screen.findByRole('button', { name: 'Delete datacenter' }));
    let confirm = await findDialog('Delete route “datacenter”?');
    await user.click(within(confirm).getByRole('button', { name: 'Cancel' }));
    expect(requestsTo('DELETE', /\/routes\//)).toHaveLength(0);

    await user.click(screen.getByRole('button', { name: 'Delete datacenter' }));
    confirm = await findDialog('Delete route “datacenter”?');
    await user.click(within(confirm).getByRole('button', { name: 'Delete route' }));

    await screen.findByText('Route “datacenter” deleted');
    expect(requestsTo('DELETE', /\/routes\//)[0].path).toBe(`/networks/${net.id}/routes/${route.id}`);
    await waitFor(() => expect(screen.queryByRole('table')).not.toBeInTheDocument());
  });

  it('reports a failed deletion', async () => {
    const { net, paris } = seedCorp();
    seedRoute(net.id, { name: 'datacenter', jump_peer_id: paris.id });
    server.use(
      http.delete('*/api/v1/networks/:id/routes/:routeId', () =>
        HttpResponse.json({ error: 'route is still in use' }, { status: 409 }),
      ),
    );
    const { user } = renderApp('/routes');

    await user.click(await screen.findByRole('button', { name: 'Delete datacenter' }));
    await user.click(within(await findDialog('Delete route “datacenter”?')).getByRole('button', { name: 'Delete route' }));

    expect(await screen.findByText('route is still in use')).toBeInTheDocument();
    expect(within(screen.getByRole('table')).getByText('datacenter')).toBeInTheDocument();
  });

  it('is reserved to administrators', async () => {
    db.me = { ...db.me, role: 'user' };
    renderApp('/routes');
    expect(await screen.findByText('Only administrators can manage routes.')).toBeInTheDocument();
  });
});
