import { screen, waitFor, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { server } from '../../test/setup';
import { db, requestsTo, seedGroup, seedNetwork, seedPeer, seedPolicy, seedRoute } from '../../test/fakeApi';
import { findDialog, openTab, pick } from '../../test/helpers';
import { renderApp } from '../../test/renderApp';

function seedCorp() {
  const net = seedNetwork({ name: 'corp' });
  const jump = seedPeer(net.id, { name: 'jump-paris', is_jump: true });
  const alice = seedPeer(net.id, { name: 'alice-laptop' });
  const bob = seedPeer(net.id, { name: 'bob-laptop' });
  const k8s = seedPolicy(net.id, { name: 'allow-k8s' });
  const deny = seedPolicy(net.id, { name: 'deny-db' });
  const dc = seedRoute(net.id, { name: 'datacenter', jump_peer_id: jump.id });
  return { net, jump, alice, bob, k8s, deny, dc };
}

describe('Groups', () => {
  it('lists the groups of every network', async () => {
    const { net } = seedCorp();
    const lab = seedNetwork({ name: 'lab' });
    seedGroup(net.id, { name: 'engineering', description: 'Developers' });
    seedGroup(lab.id, { name: 'testers' });
    renderApp('/groups');

    const table = await screen.findByRole('table');
    expect(within(table).getByText('engineering')).toBeInTheDocument();
    expect(within(table).getByText('testers')).toBeInTheDocument();
    expect(within(table).getByText('Developers')).toBeInTheDocument();
  });

  it('filters the list by network', async () => {
    const { net } = seedCorp();
    const lab = seedNetwork({ name: 'lab' });
    seedGroup(net.id, { name: 'engineering' });
    seedGroup(lab.id, { name: 'testers' });
    const { user } = renderApp('/groups');
    await screen.findByText('testers');

    await pick(user, document.body, 'Network', 'lab');

    await waitFor(() => expect(screen.queryByText('engineering')).not.toBeInTheDocument());
    expect(screen.getByText('testers')).toBeInTheDocument();
  });

  it('creates a group with its peers, ordered policies and routes in one go', async () => {
    const { net, alice, k8s, deny, dc } = seedCorp();
    const { user } = renderApp('/groups');

    await user.click(await screen.findByRole('button', { name: 'New group' }));
    const dialog = await findDialog('New group');
    await pick(user, dialog, 'Network', 'corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'engineering');
    await user.clear(within(dialog).getByLabelText('Priority'));
    await user.type(within(dialog).getByLabelText('Priority'), '50');
    await user.type(within(dialog).getByLabelText('Description'), 'Developers');

    await openTab(user, dialog, 'Peers');
    await pick(user, dialog, 'Add a peer', /alice-laptop/);
    await openTab(user, dialog, 'Policies');
    await pick(user, dialog, 'Attach a policy', /allow-k8s/);
    await pick(user, dialog, 'Attach a policy', /deny-db/);
    // deny-db must be evaluated first.
    await user.click(within(dialog).getByRole('button', { name: 'Move deny-db up' }));
    await openTab(user, dialog, 'Routes');
    await pick(user, dialog, 'Attach a route', /datacenter/);

    await user.click(within(dialog).getByRole('button', { name: 'Create group' }));

    expect(await screen.findByText('Group “engineering” created')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New group' })).not.toBeInTheDocument());
    expect(requestsTo('POST', /\/groups$/)[0].body).toEqual({ name: 'engineering', description: 'Developers', priority: 50 });
    const group = db.groups.find((g) => g.name === 'engineering')!;
    expect(group.network_id).toBe(net.id);
    expect(group.peer_ids).toEqual([alice.id]);
    expect(group.policy_ids).toEqual([deny.id, k8s.id]);
    expect(group.route_ids).toEqual([dc.id]);
    expect(within(await screen.findByRole('table')).getByText('engineering')).toBeInTheDocument();
  });

  it('checks the form before sending anything', async () => {
    seedCorp();
    const { user } = renderApp('/groups');
    await user.click(await screen.findByRole('button', { name: 'New group' }));
    const dialog = await findDialog('New group');
    await user.clear(within(dialog).getByLabelText('Priority'));
    await user.type(within(dialog).getByLabelText('Priority'), '1200');

    await user.click(within(dialog).getByRole('button', { name: 'Create group' }));

    expect(within(dialog).getByText('Choose the network of the group.')).toBeInTheDocument();
    expect(within(dialog).getByText('Give the group a name.')).toBeInTheDocument();
    expect(within(dialog).getByText('Enter a whole number between 1 and 999.')).toBeInTheDocument();
    expect(within(dialog).getByLabelText(/^Name/)).toHaveAttribute('aria-invalid', 'true');
    expect(requestsTo('POST', /\/groups$/)).toHaveLength(0);
  });

  it('refuses a group routing through one of its own members', async () => {
    seedCorp();
    const { user } = renderApp('/groups');
    await user.click(await screen.findByRole('button', { name: 'New group' }));
    const dialog = await findDialog('New group');
    await pick(user, dialog, 'Network', 'corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'gateways');
    await openTab(user, dialog, 'Peers');
    await pick(user, dialog, 'Add a peer', /jump-paris/);
    await openTab(user, dialog, 'Routes');
    await pick(user, dialog, 'Attach a route', /datacenter/);

    await user.click(within(dialog).getByRole('button', { name: 'Create group' }));

    expect(await within(dialog).findByRole('alert')).toHaveTextContent(
      'route “datacenter” goes through “jump-paris”, a member of this group',
    );
    expect(requestsTo('POST', /\/groups$/)).toHaveLength(0);
  });

  it('shows the server error in the dialog', async () => {
    const { net } = seedCorp();
    seedGroup(net.id, { name: 'engineering' });
    const { user } = renderApp('/groups');
    await user.click(await screen.findByRole('button', { name: 'New group' }));
    const dialog = await findDialog('New group');
    await pick(user, dialog, 'Network', 'corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'engineering');

    await user.click(within(dialog).getByRole('button', { name: 'Create group' }));

    expect(await within(dialog).findByRole('alert')).toHaveTextContent('a group with this name already exists');
    expect(screen.getByRole('dialog', { name: 'New group' })).toBeInTheDocument();
  });

  it('keeps editing the created group when an attachment fails, instead of creating it twice', async () => {
    const { alice } = seedCorp();
    server.use(
      http.post('*/api/v1/networks/:id/groups/:groupId/peers/:peerId', () =>
        HttpResponse.json({ error: 'peer is quarantined' }, { status: 409 }),
      ),
    );
    const { user } = renderApp('/groups');
    await user.click(await screen.findByRole('button', { name: 'New group' }));
    let dialog = await findDialog('New group');
    await pick(user, dialog, 'Network', 'corp');
    await user.type(within(dialog).getByLabelText(/^Name/), 'engineering');
    await openTab(user, dialog, 'Peers');
    await pick(user, dialog, 'Add a peer', /alice-laptop/);

    await user.click(within(dialog).getByRole('button', { name: 'Create group' }));

    dialog = await findDialog('Edit group engineering');
    expect(within(dialog).getByRole('alert')).toHaveTextContent('The group was created, but not all its attachments: peer is quarantined');
    // A second submit saves the same group.
    server.resetHandlers();
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));
    await screen.findByText('Group “engineering” saved');
    expect(requestsTo('POST', /\/groups$/)).toHaveLength(1);
    expect(db.groups.find((g) => g.name === 'engineering')!.peer_ids).toEqual([alice.id]);
  });

  it('edits a group: details and attachments are saved together', async () => {
    const { net, alice, bob, k8s, deny } = seedCorp();
    const group = seedGroup(net.id, { name: 'engineering', peer_ids: [alice.id], policy_ids: [k8s.id, deny.id] });
    const { user } = renderApp('/groups');

    await user.click(await screen.findByRole('button', { name: 'Edit engineering' }));
    const dialog = await findDialog('Edit group engineering');
    await user.clear(within(dialog).getByLabelText(/^Name/));
    await user.type(within(dialog).getByLabelText(/^Name/), 'developers');
    await openTab(user, dialog, 'Peers');
    await user.click(within(dialog).getByRole('button', { name: 'Remove alice-laptop' }));
    await pick(user, dialog, 'Add a peer', /bob-laptop/);
    await openTab(user, dialog, 'Policies');
    await user.click(within(dialog).getByRole('button', { name: 'Move deny-db up' }));

    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

    await screen.findByText('Group “developers” saved');
    const saved = db.groups.find((g) => g.id === group.id)!;
    expect(saved.name).toBe('developers');
    expect(saved.peer_ids).toEqual([bob.id]);
    expect(saved.policy_ids).toEqual([deny.id, k8s.id]);
    expect(requestsTo('PUT', /\/policies\/order$/)[0].body).toEqual({ policy_ids: [deny.id, k8s.id] });
  });

  it('protects the name of the default group', async () => {
    const { net } = seedCorp();
    seedGroup(net.id, { name: 'default' });
    const { user } = renderApp('/groups');
    await user.click(await screen.findByRole('button', { name: 'Edit default' }));
    const dialog = await findDialog('Edit group default');
    expect(within(dialog).getByLabelText(/^Name/)).toBeDisabled();
    expect(within(dialog).getByText('Default group')).toBeInTheDocument();
  });

  it('deletes a group after confirmation, in its own network', async () => {
    const { net } = seedCorp();
    const group = seedGroup(net.id, { name: 'engineering' });
    const { user } = renderApp('/groups');

    await user.click(await screen.findByRole('button', { name: 'Delete engineering' }));
    let confirm = await findDialog('Delete group “engineering”?');
    await user.click(within(confirm).getByRole('button', { name: 'Cancel' }));
    expect(requestsTo('DELETE', /\/groups\//)).toHaveLength(0);

    await user.click(screen.getByRole('button', { name: 'Delete engineering' }));
    confirm = await findDialog('Delete group “engineering”?');
    await user.click(within(confirm).getByRole('button', { name: 'Delete group' }));

    await screen.findByText('Group “engineering” deleted');
    expect(requestsTo('DELETE', /\/groups\//)[0].path).toBe(`/networks/${net.id}/groups/${group.id}`);
    await waitFor(() => expect(screen.queryByRole('table')).not.toBeInTheDocument());
  });

  it('is reserved to administrators', async () => {
    db.me = { ...db.me, role: 'user' };
    renderApp('/groups');
    expect(await screen.findByText('Only administrators can manage groups.')).toBeInTheDocument();
  });
});
