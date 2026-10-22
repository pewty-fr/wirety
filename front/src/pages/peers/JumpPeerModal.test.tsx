import { screen, waitFor, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { server } from '../../test/setup';
import { db, requests, requestsTo, seedNetwork, seedPeer, seedUser } from '../../test/fakeApi';
import { findDialog, pick } from '../../test/helpers';
import { renderApp } from '../../test/renderApp';

const API = '*/api/v1';

async function openNewJumpPeer() {
  const app = renderApp('/peers');
  await app.user.click((await screen.findAllByRole('button', { name: 'New jump peer' }))[0]);
  return { ...app, dialog: await findDialog('New jump peer') };
}

const rule = (direction: string, target: string, description: string) => ({
  id: expect.any(String),
  direction,
  action: 'allow',
  target,
  target_type: 'cidr',
  description,
});

describe('New jump peer / edit jump peer', () => {
  it('creates the first jump peer of the chosen network and sets up its default access there, step by step', async () => {
    // corp comes first: everything must happen in lab, the chosen network.
    seedNetwork({ name: 'corp', cidr: '10.42.0.0/24' });
    const lab = seedNetwork({ name: 'lab', cidr: '10.43.0.0/24', cidr_v6: 'fd00:43::/64' });
    const alice = seedPeer(lab.id, { name: 'alice-laptop' });
    const bob = seedPeer(lab.id, { name: 'bob-laptop' });
    const { user, dialog } = await openNewJumpPeer();

    expect(within(dialog).getByLabelText('Use the agent')).toBeChecked();
    expect(within(dialog).getByLabelText('Listen port')).toHaveValue(51820);
    await pick(user, dialog, 'Network', 'lab');
    await user.type(within(dialog).getByLabelText(/^Name/), 'jump-paris');
    await user.type(within(dialog).getByLabelText('Endpoint'), 'vpn.example.com');
    await user.click(within(dialog).getByRole('button', { name: 'Create jump peer' }));

    expect(
      await screen.findByText('Jump peer “jump-paris” created, with the network\'s default group, policy and route'),
    ).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New jump peer' })).not.toBeInTheDocument());

    const jump = db.peers.find((p) => p.name === 'jump-paris')!;
    const group = db.groups.find((g) => g.name === 'default')!;
    const policy = db.policies.find((p) => p.name === 'default')!;
    const route = db.routes.find((r) => r.name === 'lab')!;
    const n = `/networks/${lab.id}`;
    expect(requests.map((r) => `${r.method} ${r.path}`)).toEqual([
      `POST ${n}/peers`,
      `POST ${n}/groups`,
      `POST ${n}/policies`,
      `POST ${n}/groups/${group.id}/policies/${policy.id}`,
      `POST ${n}/routes`,
      `POST ${n}/groups/${group.id}/routes/${route.id}`,
      `POST ${n}/groups/${group.id}/peers/${alice.id}`,
      `POST ${n}/groups/${group.id}/peers/${bob.id}`,
    ]);
    expect(requests[0].body).toEqual({
      name: 'jump-paris',
      endpoint: 'vpn.example.com',
      listen_port: 51820,
      is_jump: true,
      use_agent: true,
      owner_id: 'admin',
    });
    expect(requests[1].body).toEqual({ name: 'default', description: 'default group generated at network creation', priority: 100 });
    expect(requests[2].body).toEqual({
      name: 'default',
      description: 'default policy for network access',
      rules: [
        rule('input', '10.43.0.0/24', 'allow network IPv4 CIDR resources to access a peer'),
        rule('output', '10.43.0.0/24', 'allow a peer to access network IPv4 CIDR resources'),
        rule('output', '0.0.0.0/0', 'allow a peer to access the IPv4 internet'),
        rule('input', 'fd00:43::/64', 'allow network IPv6 CIDR resources to access a peer'),
        rule('output', 'fd00:43::/64', 'allow a peer to access network IPv6 CIDR resources'),
        rule('output', '::/0', 'allow a peer to access the IPv6 internet'),
      ],
    });
    expect(requests[4].body).toEqual({
      name: 'lab',
      description: 'default route for network',
      destination_cidr: '10.43.0.0/24',
      destination_cidr_v6: 'fd00:43::/64',
      jump_peer_id: jump.id,
    });
    expect(group).toMatchObject({ network_id: lab.id, peer_ids: [alice.id, bob.id], policy_ids: [policy.id], route_ids: [route.id] });
  });

  it('gives an IPv4-only network IPv4 rules and route only', async () => {
    const corp = seedNetwork({ name: 'corp', cidr: '10.42.0.0/24' });
    const { user, dialog } = await openNewJumpPeer();
    expect(within(dialog).getByRole('combobox', { name: 'Network' })).toHaveValue('corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'jump-paris');
    await user.click(within(dialog).getByRole('button', { name: 'Create jump peer' }));
    await screen.findByText(/Jump peer “jump-paris” created/);

    const [policy] = requestsTo('POST', /\/policies$/);
    expect((policy.body as { rules: { target: string }[] }).rules.map((r) => r.target)).toEqual([
      '10.42.0.0/24',
      '10.42.0.0/24',
      '0.0.0.0/0',
    ]);
    expect(requestsTo('POST', /\/routes$/)[0]).toMatchObject({
      path: `/networks/${corp.id}/routes`,
      body: { name: 'corp', description: 'default route for network', destination_cidr: '10.42.0.0/24' },
    });
    expect(requestsTo('POST', /\/routes$/)[0].body).not.toHaveProperty('destination_cidr_v6');
  });

  it('only creates the peer when the network already has a jump peer', async () => {
    const corp = seedNetwork({ name: 'corp' });
    seedPeer(corp.id, { name: 'jump-paris', is_jump: true });
    const { user } = renderApp('/peers');
    await screen.findByRole('table');
    await pick(user, document.body, 'Network', 'corp');
    await user.click(screen.getByRole('button', { name: 'New jump peer' }));
    const dialog = await findDialog('New jump peer');
    expect(within(dialog).getByRole('combobox', { name: 'Network' })).toHaveValue('corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'jump-lyon');
    await user.click(within(dialog).getByLabelText('Use the agent'));
    await pick(user, dialog, 'Owner', 'No owner');

    await user.click(within(dialog).getByRole('button', { name: 'Create jump peer' }));

    expect(await screen.findByText('Jump peer “jump-lyon” created')).toBeInTheDocument();
    expect(requests).toEqual([
      {
        method: 'POST',
        path: `/networks/${corp.id}/peers`,
        body: { name: 'jump-lyon', listen_port: 51820, is_jump: true, use_agent: false },
      },
    ]);
  });

  it('checks the form before sending anything', async () => {
    seedNetwork({ name: 'corp' });
    seedNetwork({ name: 'lab' });
    const { user, dialog } = await openNewJumpPeer();
    await user.clear(within(dialog).getByLabelText('Listen port'));
    await user.type(within(dialog).getByLabelText('Listen port'), '70000');

    await user.click(within(dialog).getByRole('button', { name: 'Create jump peer' }));

    expect(within(dialog).getByText('Choose the network of the jump peer.')).toBeInTheDocument();
    expect(within(dialog).getByText('Give the jump peer a name.')).toBeInTheDocument();
    expect(within(dialog).getByText('Enter a port between 1 and 65535.')).toBeInTheDocument();
    expect(within(dialog).getByLabelText('Listen port')).toHaveAttribute('aria-invalid', 'true');
    expect(requests).toHaveLength(0);
  });

  it('lets the server pick the port when the field is cleared', async () => {
    const corp = seedNetwork({ name: 'corp' });
    seedPeer(corp.id, { name: 'jump-paris', is_jump: true });
    const { user, dialog } = await openNewJumpPeer();
    await user.type(within(dialog).getByLabelText(/^Name/), 'jump-lyon');
    await user.clear(within(dialog).getByLabelText('Listen port'));

    await user.click(within(dialog).getByRole('button', { name: 'Create jump peer' }));

    await screen.findByText('Jump peer “jump-lyon” created');
    expect(requestsTo('POST', /\/peers$/)[0].body).toEqual({ name: 'jump-lyon', is_jump: true, use_agent: true, owner_id: 'admin' });
  });

  it('shows the server error in the dialog', async () => {
    const corp = seedNetwork({ name: 'corp' });
    seedPeer(corp.id, { name: 'jump-paris', is_jump: true });
    const { user, dialog } = await openNewJumpPeer();
    await user.type(within(dialog).getByLabelText(/^Name/), 'jump-paris');

    await user.click(within(dialog).getByRole('button', { name: 'Create jump peer' }));

    expect(await within(dialog).findByRole('alert')).toHaveTextContent('a peer with this name already exists in the network');
    expect(screen.getByRole('dialog', { name: 'New jump peer' })).toBeInTheDocument();
  });

  it('reports a failed default access setup and keeps editing the created jump peer', async () => {
    const corp = seedNetwork({ name: 'corp', cidr: '10.42.0.0/24' });
    server.use(
      http.post(`${API}/networks/:id/policies`, () => HttpResponse.json({ error: 'database is locked' }, { status: 500 })),
    );
    const { user, dialog } = await openNewJumpPeer();
    await user.type(within(dialog).getByLabelText(/^Name/), 'jump-paris');

    await user.click(within(dialog).getByRole('button', { name: 'Create jump peer' }));

    const edit = await findDialog('Edit jump peer');
    expect(within(edit).getByRole('alert')).toHaveTextContent(
      "The jump peer was created, but the network's default access (group, policy and route) could not be set up: database is locked",
    );
    // No route nor group membership after the failed step.
    expect(requestsTo('POST', /\/routes$/)).toHaveLength(0);
    server.resetHandlers();
    await user.click(within(edit).getByRole('button', { name: 'Save changes' }));

    await screen.findByText('Jump peer “jump-paris” saved');
    expect(requestsTo('POST', /\/peers$/)).toHaveLength(1);
    const jump = db.peers.find((p) => p.name === 'jump-paris')!;
    expect(requestsTo('PUT', /\/peers\//)[0]).toEqual({
      method: 'PUT',
      path: `/networks/${corp.id}/peers/${jump.id}`,
      body: { name: 'jump-paris', listen_port: 51820 },
    });
  });

  it('sends nothing when cancelled', async () => {
    seedNetwork({ name: 'corp' });
    const { user, dialog } = await openNewJumpPeer();
    await user.type(within(dialog).getByLabelText(/^Name/), 'jump-paris');

    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(requests).toHaveLength(0);
  });

  it('edits the endpoint, port and owner of a jump peer', async () => {
    const corp = seedNetwork({ name: 'corp' });
    const bob = seedUser({ name: 'Bob', email: 'bob@example.com' });
    const jump = seedPeer(corp.id, { name: 'jump-paris', is_jump: true, endpoint: 'old.example.com', listen_port: 51820 });
    const { user } = renderApp('/peers');

    await user.click(await screen.findByRole('button', { name: 'Edit jump-paris' }));
    const dialog = await findDialog('Edit jump peer');
    expect(within(dialog).getByLabelText('Endpoint')).toHaveValue('old.example.com');
    expect(within(dialog).queryByLabelText('Use the agent')).not.toBeInTheDocument();
    await user.clear(within(dialog).getByLabelText('Endpoint'));
    await user.type(within(dialog).getByLabelText('Endpoint'), 'vpn.example.com');
    await user.clear(within(dialog).getByLabelText('Listen port'));
    await user.type(within(dialog).getByLabelText('Listen port'), '443');
    await pick(user, dialog, 'Owner', 'Bob');

    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

    await screen.findByText('Jump peer “jump-paris” saved');
    expect(requests).toEqual([
      {
        method: 'PUT',
        path: `/networks/${corp.id}/peers/${jump.id}`,
        body: { name: 'jump-paris', endpoint: 'vpn.example.com', listen_port: 443, owner_id: bob.id },
      },
    ]);
  });

  it('removes the endpoint and the owner of a jump peer', async () => {
    const corp = seedNetwork({ name: 'corp' });
    const bob = seedUser({ name: 'Bob', email: 'bob@example.com' });
    const jump = seedPeer(corp.id, { name: 'jump-paris', is_jump: true, endpoint: 'vpn.example.com', owner_id: bob.id });
    const { user } = renderApp('/peers');

    await user.click(await screen.findByRole('button', { name: 'Edit jump-paris' }));
    const dialog = await findDialog('Edit jump peer');
    await user.clear(within(dialog).getByLabelText('Endpoint'));
    await pick(user, dialog, 'Owner', 'No owner');
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

    await screen.findByText('Jump peer “jump-paris” saved');
    // Fields sent empty are cleared by the server.
    expect(requestsTo('PUT', /\/peers\//)[0].body).toEqual({ name: 'jump-paris', endpoint: '', owner_id: '' });
    expect(db.peers.find((p) => p.id === jump.id)).toMatchObject({ endpoint: '', owner_id: '' });
  });
});
