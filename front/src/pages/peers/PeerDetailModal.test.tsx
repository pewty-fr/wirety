import { screen, waitFor, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { server } from '../../test/setup';
import { db, requests, requestsTo, seedGroup, seedNetwork, seedPeer, seedPolicy, seedRoute, seedUser } from '../../test/fakeApi';
import { findDialog, openTab, pick } from '../../test/helpers';
import { renderApp } from '../../test/renderApp';

const API = '*/api/v1';

function seed() {
  const corp = seedNetwork({ name: 'corp', cidr: '10.42.0.0/24', domain_suffix: 'internal' });
  const jump = seedPeer(corp.id, { name: 'jump-paris', is_jump: true, use_agent: true, endpoint: 'vpn.example.com', listen_port: 51820 });
  const alice = seedPeer(corp.id, { name: 'alice-laptop', address: '10.42.0.2', address_v6: 'fd00::2', owner_id: 'admin' });
  return { corp, jump, alice };
}

/** Renders the peers page and opens the details of `name`. */
async function openDetails(name: string, title = name) {
  const app = renderApp('/peers');
  await app.user.click(await screen.findByText(name));
  return { ...app, dialog: await findDialog(title) };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('Peer details', () => {
  it('shows the configuration and status of a peer', async () => {
    seed();
    const { dialog } = await openDetails('alice-laptop');

    expect(within(dialog).getByText('Peer in network corp')).toBeInTheDocument();
    expect(within(dialog).getByRole('tab', { name: 'Configuration', selected: true })).toBeInTheDocument();
    expect(within(dialog).getByText('alice-laptop.corp.internal')).toBeInTheDocument();
    expect(within(dialog).getByText('10.42.0.2')).toBeInTheDocument();
    expect(within(dialog).getByText('fd00::2')).toBeInTheDocument();
    expect(within(dialog).getByText('Administrator')).toBeInTheDocument();
    expect(within(dialog).getByText('No agent')).toBeInTheDocument();
    expect(within(dialog).getByText('Not authenticated')).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Reset authentication' })).toBeInTheDocument();
  });

  it('shows the endpoint and port of a jump peer, without captive portal', async () => {
    seed();
    const { dialog } = await openDetails('jump-paris');

    expect(within(dialog).getByText('Jump peer in network corp')).toBeInTheDocument();
    expect(within(dialog).getByText('vpn.example.com')).toBeInTheDocument();
    expect(within(dialog).getByText('51820')).toBeInTheDocument();
    expect(within(dialog).getByText('Agent disconnected')).toBeInTheDocument();
    expect(within(dialog).queryByText('Captive portal')).not.toBeInTheDocument();
    expect(within(dialog).queryByRole('button', { name: 'Reset authentication' })).not.toBeInTheDocument();
  });

  it('copies and downloads the WireGuard configuration of a peer', async () => {
    seed();
    const { user, dialog } = await openDetails('alice-laptop');
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue();
    const downloads: string[] = [];
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      downloads.push(this.download);
    });
    Object.assign(URL, { createObjectURL: vi.fn(() => 'blob:config'), revokeObjectURL: vi.fn() });

    await user.click(within(dialog).getByRole('button', { name: 'Copy configuration' }));
    expect(await screen.findByText('Configuration copied to the clipboard')).toBeInTheDocument();
    expect(writeText).toHaveBeenCalledWith('[Interface]\nPrivateKey = test\nAddress = 10.42.0.2/32\n');

    await user.click(within(dialog).getByRole('button', { name: 'Download .conf' }));
    await waitFor(() => expect(downloads).toEqual(['alice-laptop.corp.internal.conf']));
  });

  it('disables the configuration of a peer without owner', async () => {
    const { corp } = seed();
    seedPeer(corp.id, { name: 'kiosk' });
    const { dialog } = await openDetails('kiosk');

    expect(within(dialog).getByText(/this peer has no owner/)).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Copy configuration' })).toBeDisabled();
    expect(within(dialog).getByRole('button', { name: 'Download .conf' })).toBeDisabled();
  });

  it('copies the agent token of a peer using the agent', async () => {
    const { corp } = seed();
    seedPeer(corp.id, { name: 'build-server', use_agent: true, token: 'enroll-token', owner_id: 'admin' });
    const { user, dialog } = await openDetails('build-server');
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue();

    expect(within(dialog).queryByRole('button', { name: 'Copy configuration' })).not.toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Copy token' }));

    expect(await screen.findByText('Token copied to the clipboard')).toBeInTheDocument();
    expect(writeText).toHaveBeenCalledWith('enroll-token');
  });

  it('lists the groups of the peer and the rules and routes they give it, in evaluation order', async () => {
    const { corp, jump, alice } = seed();
    const web = seedPolicy(corp.id, {
      name: 'allow-web',
      rules: [{ id: 'r1', direction: 'output', action: 'allow', target: '0.0.0.0/0', target_type: 'cidr', description: 'internet' }],
    });
    const db1 = seedPolicy(corp.id, {
      name: 'deny-db',
      rules: [{ id: 'r2', direction: 'output', action: 'deny', target: '10.42.0.9/32', target_type: 'cidr', description: 'database' }],
    });
    const dc = seedRoute(corp.id, { name: 'datacenter', destination_cidr: '10.0.0.0/16', jump_peer_id: jump.id });
    seedGroup(corp.id, { name: 'default', priority: 100, peer_ids: [alice.id], policy_ids: [web.id] });
    seedGroup(corp.id, { name: 'engineering', priority: 50, peer_ids: [alice.id], policy_ids: [db1.id, web.id], route_ids: [dc.id] });
    const { user, dialog } = await openDetails('alice-laptop');

    await openTab(user, dialog, 'Access control');

    const groups = await within(dialog).findByRole('list', { name: 'Groups' });
    expect(within(groups).getAllByRole('listitem').map((li) => li.textContent)).toEqual([
      'engineeringPriority 50',
      'defaultPriority 100',
    ]);
    const rules = within(dialog).getByRole('list', { name: 'Effective rules' });
    const items = within(rules).getAllByRole('listitem');
    expect(items).toHaveLength(2);
    expect(items[0]).toHaveTextContent('DenyOutputcidr: 10.42.0.9/32');
    expect(items[0]).toHaveTextContent('From policy “deny-db” in group “engineering” (priority 50)');
    expect(items[1]).toHaveTextContent('From policy “allow-web” in group “engineering” (priority 50)');
    const routes = within(dialog).getByRole('list', { name: 'Effective routes' });
    expect(routes).toHaveTextContent('datacenter10.0.0.0/16');
  });

  it('shows what the peer can reach', async () => {
    const { jump, alice } = seed();
    server.use(
      http.get(`${API}/networks/:id/peers/:peerId/reachability`, () =>
        HttpResponse.json({
          peer_id: alice.id,
          peer_name: 'alice-laptop',
          peer_address: '10.42.0.2',
          is_jump: false,
          peer_access: [
            { peer_id: jump.id, peer_name: 'jump-paris', address: '10.42.0.1', is_jump: true, allowed: true, reason: 'allow_rule' },
            { peer_id: 'peer-x', peer_name: 'bob-laptop', address: '10.42.0.3', is_jump: false, allowed: false, reason: 'blocked' },
          ],
          rules: [
            {
              direction: 'output',
              action: 'deny',
              target_type: 'group',
              target: 'databases',
              addresses: ['10.42.0.9'],
              policy_name: 'deny-db',
              group_name: 'engineering',
            },
          ],
          routes: [
            {
              route_id: 'rt-x',
              route_name: 'datacenter',
              destination_cidr: '10.0.0.0/16',
              jump_peer_id: jump.id,
              jump_peer_name: 'jump-paris',
              group_name: 'engineering',
            },
          ],
        }),
      ),
    );
    const { user, dialog } = await openDetails('alice-laptop');

    await openTab(user, dialog, 'Reachability');

    const access = await within(dialog).findByRole('list', { name: 'Peer access' });
    expect(within(dialog).getByText('1 allowed, 1 denied')).toBeInTheDocument();
    const [jumpRow, bobRow] = within(access).getAllByRole('listitem');
    expect(jumpRow).toHaveTextContent('jump-paris');
    expect(jumpRow).toHaveTextContent('Allow rule');
    expect(bobRow).toHaveTextContent('bob-laptop');
    expect(bobRow).toHaveTextContent('Blocked');
    expect(within(dialog).getByRole('list', { name: 'Firewall rules' })).toHaveTextContent('10.42.0.9');
    expect(within(dialog).getByRole('list', { name: 'External routes' })).toHaveTextContent('via jump-paris');
  });

  it('deletes the peer after confirmation and closes the details', async () => {
    const { corp, alice } = seed();
    const { user, dialog } = await openDetails('alice-laptop');

    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));
    let confirm = await findDialog('Delete peer “alice-laptop”?');
    await user.click(within(confirm).getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Delete peer “alice-laptop”?' })).not.toBeInTheDocument());
    expect(requests).toHaveLength(0);
    expect(screen.getByRole('dialog', { name: 'alice-laptop' })).toBeInTheDocument();

    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));
    confirm = await findDialog('Delete peer “alice-laptop”?');
    await user.click(within(confirm).getByRole('button', { name: 'Delete peer' }));

    expect(await screen.findByText('Peer “alice-laptop” deleted')).toBeInTheDocument();
    expect(requests).toEqual([{ method: 'DELETE', path: `/networks/${corp.id}/peers/${alice.id}`, body: undefined }]);
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    await waitFor(() => expect(screen.queryByRole('row', { name: /alice-laptop/ })).not.toBeInTheDocument());
  });

  it('resets the captive-portal authentication of the peer after confirmation', async () => {
    const { corp, alice } = seed();
    const { user, dialog } = await openDetails('alice-laptop');

    await user.click(within(dialog).getByRole('button', { name: 'Reset authentication' }));
    let confirm = await findDialog('Reset the authentication of “alice-laptop”?');
    expect(confirm).toHaveTextContent('lifts any quarantine');
    await user.click(within(confirm).getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: /Reset the authentication/ })).not.toBeInTheDocument());
    expect(requests).toHaveLength(0);

    await user.click(within(dialog).getByRole('button', { name: 'Reset authentication' }));
    confirm = await findDialog('Reset the authentication of “alice-laptop”?');
    await user.click(within(confirm).getByRole('button', { name: 'Reset authentication' }));

    expect(await screen.findByText('Authentication of “alice-laptop” reset')).toBeInTheDocument();
    expect(requestsTo('POST', /\/revoke-auth$/)).toEqual([
      { method: 'POST', path: `/networks/${corp.id}/peers/${alice.id}/revoke-auth`, body: undefined },
    ]);
    // The details stay open.
    expect(screen.getByRole('dialog', { name: 'alice-laptop' })).toBeInTheDocument();
  });

  it('shows the server error when the reset fails', async () => {
    seed();
    server.use(
      http.post(`${API}/networks/:id/peers/:peerId/revoke-auth`, () =>
        HttpResponse.json({ error: 'captive portal is disabled' }, { status: 500 }),
      ),
    );
    const { user, dialog } = await openDetails('alice-laptop');

    await user.click(within(dialog).getByRole('button', { name: 'Reset authentication' }));
    const confirm = await findDialog('Reset the authentication of “alice-laptop”?');
    await user.click(within(confirm).getByRole('button', { name: 'Reset authentication' }));

    expect(await screen.findByText('captive portal is disabled')).toBeInTheDocument();
  });

  it('edits the peer from its latest data, then shows the changes', async () => {
    const { corp, alice } = seed();
    const bob = seedUser({ name: 'Bob', email: 'bob@example.com' });
    const app = renderApp('/peers');
    await screen.findByText('alice-laptop');
    // Changed since the list was loaded.
    Object.assign(db.peers.find((p) => p.id === alice.id)!, { name: 'alice-macbook', owner_id: bob.id });
    await app.user.click(screen.getByText('alice-laptop'));
    const dialog = await findDialog('alice-macbook');

    await app.user.click(within(dialog).getByRole('button', { name: 'Edit' }));
    const edit = await findDialog('Edit peer');
    expect(within(edit).getByLabelText(/^Name/)).toHaveValue('alice-macbook');
    expect(within(edit).getByRole('combobox', { name: 'Owner' })).toHaveValue('Bob');
    await app.user.clear(within(edit).getByLabelText(/^Name/));
    await app.user.type(within(edit).getByLabelText(/^Name/), 'alice-mbp');
    await pick(app.user, edit, 'Owner', 'Administrator');
    await app.user.click(within(edit).getByRole('button', { name: 'Save changes' }));

    await screen.findByText('Peer “alice-mbp” saved');
    expect(requests).toEqual([
      { method: 'PUT', path: `/networks/${corp.id}/peers/${alice.id}`, body: { name: 'alice-mbp', owner_id: 'admin' } },
    ]);
    const details = await findDialog('alice-mbp');
    await app.user.click(within(details).getByRole('button', { name: 'Close' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(await screen.findByRole('row', { name: /alice-mbp/ })).toHaveTextContent('Administrator');
  });

  it('opens on the configuration again after being closed', async () => {
    seed();
    const { user, dialog } = await openDetails('alice-laptop');
    await openTab(user, dialog, 'Reachability');
    await within(dialog).findByText('No other peer in this network.');

    await user.click(within(dialog).getByRole('button', { name: 'Close' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    await user.click(screen.getByText('jump-paris'));

    const next = await findDialog('jump-paris');
    expect(within(next).getByRole('tab', { name: 'Configuration', selected: true })).toBeInTheDocument();
  });

  it('only lets the owner or an administrator manage a peer', async () => {
    const { corp } = seed();
    db.me = { ...db.me, id: 'carol', name: 'Carol', role: 'user', authorized_networks: [corp.id] };
    seedPeer(corp.id, { name: 'carol-phone', owner_id: 'carol' });
    const app = renderApp('/peers');

    await app.user.click(await screen.findByText('jump-paris'));
    let dialog = await findDialog('jump-paris');
    expect(within(dialog).queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument();
    expect(within(dialog).queryByRole('button', { name: 'Delete' })).not.toBeInTheDocument();
    await openTab(app.user, dialog, 'Access control');
    expect(within(dialog).getByText(/Only administrators can see the groups/)).toBeInTheDocument();
    await app.user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

    await app.user.click(screen.getByText('carol-phone'));
    dialog = await findDialog('carol-phone');
    expect(within(dialog).getByRole('button', { name: 'Edit' })).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Delete' })).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Reset authentication' })).toBeInTheDocument();
  });
});
