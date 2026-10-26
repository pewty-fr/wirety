import { screen, waitFor, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { server } from '../../test/setup';
import { db, requestsTo, seedNetwork, seedPeer, seedUser } from '../../test/fakeApi';
import { findDialog, pick } from '../../test/helpers';
import { renderApp } from '../../test/renderApp';

const API = '*/api/v1';

function seedNetworks() {
  const corp = seedNetwork({ name: 'corp', cidr: '10.42.0.0/24' });
  const lab = seedNetwork({ name: 'lab', cidr: '10.43.0.0/24' });
  const bob = seedUser({ name: 'Bob', email: 'bob@example.com' });
  const jump = seedPeer(corp.id, { name: 'jump-paris', is_jump: true, use_agent: true, address: '10.42.0.1' });
  const alice = seedPeer(corp.id, { name: 'alice-laptop', address: '10.42.0.2', address_v6: 'fd00::2', owner_id: 'admin' });
  const server1 = seedPeer(lab.id, { name: 'build-server', address: '10.43.0.2', use_agent: true, owner_id: bob.id });
  return { corp, lab, bob, jump, alice, server1 };
}

const rows = () => within(screen.getByRole('table')).getAllByRole('row').slice(1);
const rowOf = (name: string) => screen.getByRole('row', { name: new RegExp(name) });

describe('Peers page', () => {
  it('lists the peers of every network with their network, address, owner and type', async () => {
    seedNetworks();
    renderApp('/peers');

    const table = await screen.findByRole('table');
    expect(within(table).getAllByRole('row')).toHaveLength(4);
    const alice = within(rowOf('alice-laptop'));
    expect(alice.getByText('corp')).toBeInTheDocument();
    expect(alice.getByText('10.42.0.2')).toBeInTheDocument();
    expect(alice.getByText('fd00::2')).toBeInTheDocument();
    expect(alice.getByText('Administrator')).toBeInTheDocument();
    expect(alice.getByText('Regular')).toBeInTheDocument();
    expect(within(rowOf('build-server')).getByText('Bob')).toBeInTheDocument();
    expect(within(rowOf('build-server')).getByText('lab')).toBeInTheDocument();
    expect(within(rowOf('jump-paris')).getByText('Jump')).toBeInTheDocument();
    expect(screen.getByText(/^3 peers/)).toBeInTheDocument();
  });

  it('shows the status of each peer and the peers blocked by the ACL', async () => {
    const corp = seedNetwork({ name: 'corp' });
    const recent = new Date().toISOString();
    const states: Record<string, { has_active_agent?: boolean; captive_portal_state?: string; last_seen?: string }> = {};
    const peer = (name: string, over: object, state: (typeof states)[string]) => {
      const p = seedPeer(corp.id, { name, ...over });
      states[p.id] = state;
      return p;
    };
    peer('jump-paris', { is_jump: true, use_agent: true }, { has_active_agent: true });
    peer('alice-laptop', {}, { captive_portal_state: 'pending_auth' });
    const bob = peer('bob-laptop', {}, { captive_portal_state: 'quarantined' });
    peer('carol-server', { use_agent: true }, {});
    peer('dave-laptop', {}, { last_seen: recent });
    peer('erin-laptop', {}, { last_seen: recent, captive_portal_state: 'authenticated' });
    peer('frank-laptop', {}, {});
    server.use(
      http.get(`${API}/networks/:id/peers/:peerId/session`, ({ params }) => {
        const s = states[String(params.peerId)] ?? {};
        return HttpResponse.json({
          peer_id: params.peerId,
          has_active_agent: Boolean(s.has_active_agent),
          last_checked: recent,
          captive_portal_state: s.captive_portal_state ?? '',
          ...(s.last_seen ? { current_session: { last_seen: s.last_seen } } : {}),
        });
      }),
      http.get(`${API}/networks/:id/acl`, () =>
        HttpResponse.json({ id: 'acl', name: 'acl', enabled: true, blocked_peers: { [bob.id]: true }, rules: [] }),
      ),
    );
    renderApp('/peers');

    await screen.findByRole('table');
    expect(within(rowOf('jump-paris')).getByText('Connected')).toBeInTheDocument();
    expect(within(rowOf('alice-laptop')).getByText('Pending sign-in')).toBeInTheDocument();
    expect(within(rowOf('bob-laptop')).getByText('Quarantined')).toBeInTheDocument();
    expect(within(rowOf('carol-server')).getByText('Agent offline')).toBeInTheDocument();
    expect(within(rowOf('dave-laptop')).getByText('Not authenticated')).toBeInTheDocument();
    expect(within(rowOf('erin-laptop')).getByText('Connected')).toBeInTheDocument();
    expect(within(rowOf('frank-laptop')).getByText('Offline')).toBeInTheDocument();
    expect(await within(rowOf('bob-laptop')).findByText('Blocked by ACL')).toBeInTheDocument();
    expect(within(rowOf('alice-laptop')).queryByText('Blocked by ACL')).not.toBeInTheDocument();
  });

  it('filters the peers by network, type, agent and IP address', async () => {
    seedNetworks();
    const { user } = renderApp('/peers');
    await screen.findByRole('table');

    await pick(user, document.body, 'Network', 'corp');
    await waitFor(() => expect(rows()).toHaveLength(2));
    expect(screen.queryByText('build-server')).not.toBeInTheDocument();

    await user.selectOptions(screen.getByLabelText('Type'), 'Regular');
    await waitFor(() => expect(rows()).toHaveLength(1));
    expect(screen.getByText('alice-laptop')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Clear filters' }));
    await waitFor(() => expect(rows()).toHaveLength(3));

    await user.selectOptions(screen.getByLabelText('Agent'), 'Agent disconnected');
    await waitFor(() => expect(rows()).toHaveLength(2));
    expect(screen.getByText('jump-paris')).toBeInTheDocument();
    expect(screen.getByText('build-server')).toBeInTheDocument();
    await user.selectOptions(screen.getByLabelText('Agent'), 'No agent');
    await waitFor(() => expect(rows()).toHaveLength(1));
    expect(screen.getByText('alice-laptop')).toBeInTheDocument();
    await user.selectOptions(screen.getByLabelText('Agent'), 'Any');

    await user.type(screen.getByLabelText('IP address'), 'fd00::');
    await waitFor(() => expect(rows()).toHaveLength(1));
    expect(screen.getByText('alice-laptop')).toBeInTheDocument();
    await user.clear(screen.getByLabelText('IP address'));
    await user.type(screen.getByLabelText('IP address'), '10.99.');
    expect(await screen.findByText('No peer matches these filters')).toBeInTheDocument();
  });

  it('paginates the peers, after filtering', async () => {
    const corp = seedNetwork({ name: 'corp' });
    const lab = seedNetwork({ name: 'lab' });
    for (let i = 1; i <= 25; i++) seedPeer(corp.id, { name: `corp-${String(i).padStart(2, '0')}` });
    seedPeer(lab.id, { name: 'lab-01' });
    const { user } = renderApp('/peers');

    await screen.findByRole('table');
    expect(rows()).toHaveLength(20);
    const pagination = screen.getByRole('navigation', { name: 'Pagination' });
    expect(pagination).toHaveTextContent('1–20 of 26');
    await user.click(within(pagination).getByRole('button', { name: 'Next' }));
    expect(rows()).toHaveLength(6);
    expect(screen.getByText('lab-01')).toBeInTheDocument();

    // Filtering starts again from the first page, over every peer.
    await pick(user, document.body, 'Network', 'lab');
    await waitFor(() => expect(rows()).toHaveLength(1));
    expect(screen.queryByRole('navigation', { name: 'Pagination' })).not.toBeInTheDocument();
  });

  it('explains that a network is needed before creating a peer', async () => {
    renderApp('/peers');

    expect(await screen.findByText('No network yet')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'New peer' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'New jump peer' })).toBeDisabled();
    expect(screen.getByRole('link', { name: 'Go to networks' })).toHaveAttribute('href', '/networks');
  });

  it('invites to create the first peer of an empty network', async () => {
    seedNetwork({ name: 'corp' });
    const { user } = renderApp('/peers');

    expect(await screen.findByText('No peer yet')).toBeInTheDocument();
    const [jumpButton] = screen.getAllByRole('button', { name: 'New jump peer' });
    expect(jumpButton).toBeEnabled();
    await user.click(jumpButton);
    const dialog = await findDialog('New jump peer');
    // The only network is picked already.
    expect(within(dialog).getByRole('combobox', { name: 'Network' })).toHaveValue('corp');
  });

  it('deletes a peer after confirmation, then refreshes the list', async () => {
    const { corp, alice } = seedNetworks();
    const { user } = renderApp('/peers');

    await user.click(await screen.findByRole('button', { name: 'Delete alice-laptop' }));
    let confirm = await findDialog('Delete peer “alice-laptop”?');
    await user.click(within(confirm).getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(requestsTo('DELETE', /\/peers\//)).toHaveLength(0);

    await user.click(screen.getByRole('button', { name: 'Delete alice-laptop' }));
    confirm = await findDialog('Delete peer “alice-laptop”?');
    await user.click(within(confirm).getByRole('button', { name: 'Delete peer' }));

    expect(await screen.findByText('Peer “alice-laptop” deleted')).toBeInTheDocument();
    expect(requestsTo('DELETE', /\/peers\//)[0].path).toBe(`/networks/${corp.id}/peers/${alice.id}`);
    await waitFor(() => expect(screen.queryByText('alice-laptop')).not.toBeInTheDocument());
    expect(rows()).toHaveLength(2);
  });

  it('shows the server error when a deletion fails', async () => {
    seedNetworks();
    server.use(
      http.delete(`${API}/networks/:id/peers/:peerId`, () =>
        HttpResponse.json({ error: 'peer is the gateway of 2 routes' }, { status: 409 }),
      ),
    );
    const { user } = renderApp('/peers');

    await user.click(await screen.findByRole('button', { name: 'Delete jump-paris' }));
    await user.click(within(await findDialog('Delete peer “jump-paris”?')).getByRole('button', { name: 'Delete peer' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('peer is the gateway of 2 routes');
    expect(screen.getByText('jump-paris')).toBeInTheDocument();
  });

  it('lets a user edit and delete only the peers they own', async () => {
    const { corp } = seedNetworks();
    db.me = { ...db.me, id: 'carol', name: 'Carol', role: 'user', authorized_networks: [corp.id] };
    seedPeer(corp.id, { name: 'carol-phone', owner_id: 'carol' });
    renderApp('/peers');

    await screen.findByText('carol-phone');
    expect(screen.getByRole('button', { name: 'Edit carol-phone' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Delete carol-phone' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Edit alice-laptop' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Delete alice-laptop' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Delete jump-paris' })).not.toBeInTheDocument();
    // Owners are only shown to administrators.
    expect(screen.queryByRole('columnheader', { name: 'Owner' })).not.toBeInTheDocument();
  });
});
