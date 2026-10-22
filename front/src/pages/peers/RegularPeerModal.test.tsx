import { screen, waitFor, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { server } from '../../test/setup';
import { db, requests, requestsTo, seedGroup, seedNetwork, seedPeer, seedUser } from '../../test/fakeApi';
import { findDialog, pick } from '../../test/helpers';
import { renderApp } from '../../test/renderApp';

const API = '*/api/v1';

function seed() {
  const corp = seedNetwork({ name: 'corp', cidr: '10.42.0.0/24' });
  const lab = seedNetwork({ name: 'lab', cidr: '10.43.0.0/24' });
  const bob = seedUser({ name: 'Bob', email: 'bob@example.com' });
  seedPeer(corp.id, { name: 'jump-paris', is_jump: true });
  const labDefault = seedGroup(lab.id, { name: 'default' });
  const corpDefault = seedGroup(corp.id, { name: 'default' });
  return { corp, lab, bob, labDefault, corpDefault };
}

async function openNewPeer() {
  const app = renderApp('/peers');
  await app.user.click(await screen.findByRole('button', { name: 'New peer' }));
  return { ...app, dialog: await findDialog('New peer') };
}

describe('New peer / edit peer', () => {
  it('creates a peer in the filtered network, owned by the current user, and adds it to that network’s default group', async () => {
    const { lab, labDefault, corpDefault } = seed();
    const { user } = renderApp('/peers');
    await screen.findByRole('table');
    await pick(user, document.body, 'Network', 'lab');

    await user.click(screen.getByRole('button', { name: 'New peer' }));
    const dialog = await findDialog('New peer');
    expect(within(dialog).getByRole('combobox', { name: 'Network' })).toHaveValue('lab');
    expect(within(dialog).getByRole('combobox', { name: 'Owner' })).toHaveValue('Administrator');
    expect(within(dialog).getByLabelText('Use the agent')).not.toBeChecked();
    await user.type(within(dialog).getByLabelText(/^Name/), 'alice-laptop');
    await user.click(within(dialog).getByLabelText('Use the agent'));

    await user.click(within(dialog).getByRole('button', { name: 'Create peer' }));

    expect(await screen.findByText('Peer “alice-laptop” created')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New peer' })).not.toBeInTheDocument());
    expect(requestsTo('POST', /\/peers$/)).toEqual([
      {
        method: 'POST',
        path: `/networks/${lab.id}/peers`,
        body: { name: 'alice-laptop', is_jump: false, use_agent: true, owner_id: 'admin' },
      },
    ]);
    const peer = db.peers.find((p) => p.name === 'alice-laptop')!;
    expect(requestsTo('POST', /\/groups\//).map((r) => r.path)).toEqual([`/networks/${lab.id}/groups/${labDefault.id}/peers/${peer.id}`]);
    expect(db.groups.find((g) => g.id === labDefault.id)!.peer_ids).toEqual([peer.id]);
    expect(db.groups.find((g) => g.id === corpDefault.id)!.peer_ids).toEqual([]);
    expect(await screen.findByRole('row', { name: /alice-laptop/ })).toBeInTheDocument();
  });

  it('lets an administrator give the peer another owner, or none', async () => {
    const { corp, bob } = seed();
    const { user, dialog } = await openNewPeer();
    await pick(user, dialog, 'Network', 'corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'bob-laptop');
    await pick(user, dialog, 'Owner', 'Bob');
    await user.click(within(dialog).getByRole('button', { name: 'Create peer' }));
    await screen.findByText('Peer “bob-laptop” created');

    await user.click(screen.getByRole('button', { name: 'New peer' }));
    const second = await findDialog('New peer');
    await pick(user, second, 'Network', 'corp');
    await user.type(within(second).getByLabelText(/^Name/), 'kiosk');
    await pick(user, second, 'Owner', 'No owner');
    await user.click(within(second).getByRole('button', { name: 'Create peer' }));
    await screen.findByText('Peer “kiosk” created');

    expect(requestsTo('POST', /\/peers$/).map((r) => r.body)).toEqual([
      { name: 'bob-laptop', is_jump: false, use_agent: false, owner_id: bob.id },
      { name: 'kiosk', is_jump: false, use_agent: false },
    ]);
    expect(requestsTo('POST', /\/peers$/)[1].path).toBe(`/networks/${corp.id}/peers`);
  });

  it('checks the form before sending anything', async () => {
    seed();
    const { user, dialog } = await openNewPeer();
    await user.type(within(dialog).getByLabelText(/^Name/), '   ');

    await user.click(within(dialog).getByRole('button', { name: 'Create peer' }));

    expect(within(dialog).getByText('Choose the network of the peer.')).toBeInTheDocument();
    expect(within(dialog).getByText('Give the peer a name.')).toBeInTheDocument();
    expect(within(dialog).getByLabelText(/^Name/)).toHaveAttribute('aria-invalid', 'true');
    expect(requests).toHaveLength(0);
  });

  it('shows the server error in the dialog', async () => {
    const { corp } = seed();
    seedPeer(corp.id, { name: 'alice-laptop' });
    const { user, dialog } = await openNewPeer();
    await pick(user, dialog, 'Network', 'corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'alice-laptop');

    await user.click(within(dialog).getByRole('button', { name: 'Create peer' }));

    expect(await within(dialog).findByRole('alert')).toHaveTextContent('a peer with this name already exists in the network');
    expect(screen.getByRole('dialog', { name: 'New peer' })).toBeInTheDocument();
    expect(requestsTo('POST', /\/groups\//)).toHaveLength(0);
  });

  it('keeps editing the created peer when it cannot join the default group, instead of creating it twice', async () => {
    const { corp } = seed();
    server.use(
      http.post(`${API}/networks/:id/groups/:groupId/peers/:peerId`, () =>
        HttpResponse.json({ error: 'group is locked' }, { status: 409 }),
      ),
    );
    const { user, dialog } = await openNewPeer();
    await pick(user, dialog, 'Network', 'corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'alice-laptop');

    await user.click(within(dialog).getByRole('button', { name: 'Create peer' }));

    const edit = await findDialog('Edit peer');
    expect(within(edit).getByRole('alert')).toHaveTextContent(
      'The peer was created, but it could not be added to the “default” group: group is locked',
    );
    server.resetHandlers();
    await user.clear(within(edit).getByLabelText(/^Name/));
    await user.type(within(edit).getByLabelText(/^Name/), 'alice-macbook');
    await user.click(within(edit).getByRole('button', { name: 'Save changes' }));

    await screen.findByText('Peer “alice-macbook” saved');
    expect(requestsTo('POST', /\/peers$/)).toHaveLength(1);
    const peer = db.peers.find((p) => p.name === 'alice-macbook')!;
    expect(requestsTo('PUT', /\/peers\//)[0]).toEqual({
      method: 'PUT',
      path: `/networks/${corp.id}/peers/${peer.id}`,
      body: { name: 'alice-macbook' },
    });
  });

  it('sends nothing when cancelled', async () => {
    seed();
    const { user, dialog } = await openNewPeer();
    await pick(user, dialog, 'Network', 'corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'alice-laptop');

    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(requests).toHaveLength(0);
  });

  it('edits the name and the owner of a peer', async () => {
    const { corp, bob } = seed();
    const alice = seedPeer(corp.id, { name: 'alice-laptop', owner_id: 'admin', use_agent: true });
    const { user } = renderApp('/peers');

    await user.click(await screen.findByRole('button', { name: 'Edit alice-laptop' }));
    const dialog = await findDialog('Edit peer');
    expect(within(dialog).queryByRole('combobox', { name: 'Network' })).not.toBeInTheDocument();
    expect(within(dialog).queryByLabelText('Use the agent')).not.toBeInTheDocument();
    expect(within(dialog).getByLabelText(/^Name/)).toHaveValue('alice-laptop');
    await user.clear(within(dialog).getByLabelText(/^Name/));
    await user.type(within(dialog).getByLabelText(/^Name/), 'alice-macbook');
    await pick(user, dialog, 'Owner', 'Bob');

    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

    await screen.findByText('Peer “alice-macbook” saved');
    expect(requestsTo('PUT', /\/peers\//)).toEqual([
      { method: 'PUT', path: `/networks/${corp.id}/peers/${alice.id}`, body: { name: 'alice-macbook', owner_id: bob.id } },
    ]);
    expect(await screen.findByRole('row', { name: /alice-macbook/ })).toHaveTextContent('Bob');
  });

  it('lets a user create a peer they own, without touching groups (reserved to administrators)', async () => {
    const { corp } = seed();
    db.me = { ...db.me, id: 'carol', name: 'Carol', role: 'user', authorized_networks: [corp.id] };
    const { user, dialog } = await openNewPeer();

    expect(within(dialog).queryByRole('combobox', { name: 'Owner' })).not.toBeInTheDocument();
    expect(within(dialog).getByText(/It will belong to you/)).toBeInTheDocument();
    await pick(user, dialog, 'Network', 'corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'carol-phone');
    await user.click(within(dialog).getByRole('button', { name: 'Create peer' }));

    await screen.findByText('Peer “carol-phone” created');
    expect(requests.map((r) => [r.method, r.path, r.body])).toEqual([
      ['POST', `/networks/${corp.id}/peers`, { name: 'carol-phone', is_jump: false, use_agent: false }],
    ]);
  });
});
