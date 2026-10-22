import { screen, waitFor, within } from '@testing-library/react';
import type { UserEvent } from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { server } from '../../test/setup';
import { db, requests, requestsTo, seedGroup, seedNetwork, seedPeer, seedPolicy, seedRoute } from '../../test/fakeApi';
import { findDialog, openTab, pick } from '../../test/helpers';
import { renderApp } from '../../test/renderApp';
import type { PolicyRule } from '../../types';

function seedCorp() {
  const net = seedNetwork({ name: 'corp', cidr: '10.42.0.0/24' });
  const jump = seedPeer(net.id, { name: 'jump-paris', is_jump: true });
  const alice = seedPeer(net.id, { name: 'alice-laptop' });
  const eng = seedGroup(net.id, { name: 'engineering', peer_ids: [alice.id] });
  const ops = seedGroup(net.id, { name: 'ops' });
  const dc = seedRoute(net.id, { name: 'datacenter', destination_cidr: '10.0.0.0/16', jump_peer_id: jump.id });
  return { net, jump, alice, eng, ops, dc };
}

const rule = (over: Partial<PolicyRule>): PolicyRule => ({
  id: 'rule',
  direction: 'output',
  action: 'allow',
  target_type: 'cidr',
  target: '10.1.0.0/16',
  description: '',
  ...over,
});

/** Opens the "New policy" dialog and fills the network and name. */
async function startPolicy(user: UserEvent, name = 'allow-web') {
  await user.click(await screen.findByRole('button', { name: 'New policy' }));
  const dialog = await findDialog('New policy');
  await pick(user, dialog, 'Network', 'corp');
  await user.type(within(dialog).getByLabelText(/^Name/), name);
  return dialog;
}

/** Adds a rule with the inline editor of the Rules tab (open). */
async function addCidrRule(user: UserEvent, dialog: HTMLElement, cidr: string) {
  await user.type(within(dialog).getByRole('textbox', { name: 'Target' }), cidr);
  await user.click(within(dialog).getByRole('button', { name: 'Add rule' }));
}

const ruleList = (dialog: HTMLElement) => within(dialog).getByRole('list', { name: 'Rules' });

describe('Policies', () => {
  it('lists the policies of every network', async () => {
    const { net } = seedCorp();
    const lab = seedNetwork({ name: 'lab', cidr: '10.43.0.0/24' });
    seedPolicy(net.id, { name: 'allow-web', description: 'Web servers', rules: [rule({ id: 'r1' }), rule({ id: 'r2' })] });
    seedPolicy(lab.id, { name: 'deny-db' });
    renderApp('/policies');

    const table = await screen.findByRole('table');
    const web = within(table).getByRole('row', { name: /allow-web/ });
    expect(within(web).getByText('Web servers')).toBeInTheDocument();
    expect(within(web).getByText('corp')).toBeInTheDocument();
    expect(within(web).getByText('2')).toBeInTheDocument();
    const deny = within(table).getByRole('row', { name: /deny-db/ });
    expect(within(deny).getByText('lab')).toBeInTheDocument();
    expect(within(deny).getByText('0')).toBeInTheDocument();
  });

  it('filters the list by network', async () => {
    const { net } = seedCorp();
    const lab = seedNetwork({ name: 'lab', cidr: '10.43.0.0/24' });
    seedPolicy(net.id, { name: 'allow-web' });
    seedPolicy(lab.id, { name: 'deny-db' });
    const { user } = renderApp('/policies');
    await screen.findByText('deny-db');

    await pick(user, document.body, 'Network', 'lab');

    await waitFor(() => expect(screen.queryByText('allow-web')).not.toBeInTheDocument());
    expect(screen.getByText('deny-db')).toBeInTheDocument();
  });

  it('creates a policy with its ordered rules and its groups in one go', async () => {
    const { net, alice, eng, ops } = seedCorp();
    const { user } = renderApp('/policies');

    const dialog = await startPolicy(user);
    await user.type(within(dialog).getByLabelText('Description'), 'Web access');

    await openTab(user, dialog, 'Rules');
    expect(within(dialog).getByText('No rule yet: add one below.')).toBeInTheDocument();
    // 1. A CIDR, with the default direction (output) and action (allow).
    expect(within(dialog).getByLabelText('Direction')).toHaveValue('output');
    expect(within(dialog).getByLabelText('Action')).toHaveValue('allow');
    await user.type(within(dialog).getByRole('textbox', { name: 'Target' }), '192.168.1.0/24');
    await user.type(within(dialog).getByLabelText('Description'), 'Office LAN');
    await user.click(within(dialog).getByRole('button', { name: 'Add rule' }));
    // The editor is cleared and keeps the direction.
    expect(within(dialog).getByRole('textbox', { name: 'Target' })).toHaveValue('');
    expect(within(dialog).getByLabelText('Description')).toHaveValue('');
    expect(within(dialog).getByLabelText('Direction')).toHaveValue('output');
    // 2. The whole network, inbound, denied.
    await user.selectOptions(within(dialog).getByLabelText('Direction'), 'input');
    await user.selectOptions(within(dialog).getByLabelText('Action'), 'deny');
    await user.selectOptions(within(dialog).getByLabelText('Target type'), 'network');
    expect(within(dialog).getByRole('combobox', { name: 'Target' })).toHaveValue('10.42.0.0/24');
    await user.click(within(dialog).getByRole('button', { name: 'Add rule' }));
    // 3. A peer, 4. a group and 5. a route, picked by name.
    await user.selectOptions(within(dialog).getByLabelText('Direction'), 'output');
    await user.selectOptions(within(dialog).getByLabelText('Action'), 'allow');
    await user.selectOptions(within(dialog).getByLabelText('Target type'), 'peer');
    await pick(user, dialog, 'Target', 'alice-laptop');
    await user.click(within(dialog).getByRole('button', { name: 'Add rule' }));
    await user.selectOptions(within(dialog).getByLabelText('Target type'), 'group');
    await pick(user, dialog, 'Target', 'ops');
    await user.click(within(dialog).getByRole('button', { name: 'Add rule' }));
    await user.selectOptions(within(dialog).getByLabelText('Target type'), 'route');
    await pick(user, dialog, 'Target', 'datacenter');
    await user.click(within(dialog).getByRole('button', { name: 'Add rule' }));

    const rows = within(ruleList(dialog)).getAllByRole('listitem');
    expect(rows).toHaveLength(5);
    expect(rows[0]).toHaveTextContent('AllowOutput');
    expect(rows[0]).toHaveTextContent('Office LAN');
    expect(rows[1]).toHaveTextContent('DenyInput');
    expect(rows[1]).toHaveTextContent('network corp');
    expect(rows[2]).toHaveTextContent('alice-laptop');
    expect(rows[3]).toHaveTextContent('ops');
    expect(rows[4]).toHaveTextContent('route datacenter');
    expect(within(dialog).getByRole('tab', { name: /^Rules/ })).toHaveTextContent('5');

    await openTab(user, dialog, 'Groups');
    await pick(user, dialog, 'Attach to a group', 'engineering');
    await pick(user, dialog, 'Attach to a group', 'ops');

    await user.click(within(dialog).getByRole('button', { name: 'Create policy' }));

    expect(await screen.findByText('Policy “allow-web” created')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New policy' })).not.toBeInTheDocument());
    // The rules are sent with the policy, in order; 'network' and 'route' as CIDRs.
    expect(requestsTo('POST', /\/policies$/)).toEqual([
      {
        method: 'POST',
        path: `/networks/${net.id}/policies`,
        body: {
          name: 'allow-web',
          description: 'Web access',
          rules: [
            { direction: 'output', action: 'allow', target_type: 'cidr', target: '192.168.1.0/24', description: 'Office LAN' },
            { direction: 'input', action: 'deny', target_type: 'cidr', target: '10.42.0.0/24', description: '' },
            { direction: 'output', action: 'allow', target_type: 'peer', target: alice.id, description: '' },
            { direction: 'output', action: 'allow', target_type: 'group', target: ops.id, description: '' },
            { direction: 'output', action: 'allow', target_type: 'cidr', target: '10.0.0.0/16', description: '' },
          ],
        },
      },
    ]);
    expect(requestsTo('POST', /\/rules$/)).toHaveLength(0);
    const policy = db.policies.find((p) => p.name === 'allow-web')!;
    // Groups are attached one after the other.
    expect(requestsTo('POST', /\/groups\/[^/]+\/policies\//).map((r) => r.path)).toEqual([
      `/networks/${net.id}/groups/${eng.id}/policies/${policy.id}`,
      `/networks/${net.id}/groups/${ops.id}/policies/${policy.id}`,
    ]);
    expect(within(await screen.findByRole('table')).getByText('allow-web')).toBeInTheDocument();
  });

  it('offers both address families of a dual-stack network and route', async () => {
    const net = seedNetwork({ name: 'corp', cidr: '10.42.0.0/24', cidr_v6: 'fd42::/64' });
    seedRoute(net.id, { name: 'datacenter', destination_cidr: '10.0.0.0/16', destination_cidr_v6: 'fd00:10::/48' });
    const { user } = renderApp('/policies');
    const dialog = await startPolicy(user, 'dual-stack');
    await openTab(user, dialog, 'Rules');

    await user.selectOptions(within(dialog).getByLabelText('Target type'), 'network');
    const target = within(dialog).getByRole('combobox', { name: 'Target' });
    expect(within(target).getAllByRole('option').map((o) => o.textContent)).toEqual([
      '10.42.0.0/24 (IPv4)',
      'fd42::/64 (IPv6)',
    ]);
    await user.selectOptions(target, 'fd42::/64');
    await user.click(within(dialog).getByRole('button', { name: 'Add rule' }));
    await user.selectOptions(within(dialog).getByLabelText('Target type'), 'route');
    await pick(user, dialog, 'Target', 'datacenter (IPv6)');
    await user.click(within(dialog).getByRole('button', { name: 'Add rule' }));
    await user.click(within(dialog).getByRole('button', { name: 'Create policy' }));

    await screen.findByText('Policy “dual-stack” created');
    const body = requestsTo('POST', /\/policies$/)[0].body as { rules: PolicyRule[] };
    expect(body.rules.map((r) => [r.target_type, r.target])).toEqual([
      ['cidr', 'fd42::/64'],
      ['cidr', 'fd00:10::/48'],
    ]);
  });

  it('checks the policy before sending anything', async () => {
    seedCorp();
    const { user } = renderApp('/policies');
    await user.click(await screen.findByRole('button', { name: 'New policy' }));
    const dialog = await findDialog('New policy');

    await user.click(within(dialog).getByRole('button', { name: 'Create policy' }));

    expect(within(dialog).getByText('Choose the network of the policy.')).toBeInTheDocument();
    expect(within(dialog).getByText('Give the policy a name.')).toBeInTheDocument();
    expect(within(dialog).getByLabelText(/^Name/)).toHaveAttribute('aria-invalid', 'true');
    // Rules and groups need the network first.
    await openTab(user, dialog, 'Rules');
    expect(within(dialog).getByText('Choose the network of the policy first.')).toBeInTheDocument();
    expect(requests).toHaveLength(0);
  });

  it('checks a rule before adding it', async () => {
    seedCorp();
    const { user } = renderApp('/policies');
    const dialog = await startPolicy(user);
    await openTab(user, dialog, 'Rules');
    const add = () => user.click(within(dialog).getByRole('button', { name: 'Add rule' }));
    const target = () => within(dialog).getByRole('textbox', { name: 'Target' });

    await add();
    expect(within(dialog).getByText('Enter a CIDR, e.g. 10.0.0.0/24.')).toBeInTheDocument();
    expect(target()).toHaveAttribute('aria-invalid', 'true');

    await user.type(target(), '10.0.0.1/24');
    await add();
    expect(within(dialog).getByText('10.0.0.1 has host bits set — did you mean 10.0.0.0/24?')).toBeInTheDocument();

    await user.clear(target());
    await user.type(target(), 'intranet');
    await add();
    expect(within(dialog).getByText('CIDR must include a prefix (e.g., 192.168.1.0/24)')).toBeInTheDocument();

    // Enter adds the rule instead of submitting the policy.
    await user.clear(target());
    await user.type(target(), '10.0.0.0/24{Enter}');
    expect(within(ruleList(dialog)).getAllByRole('listitem')).toHaveLength(1);
    await addCidrRule(user, dialog, '10.0.0.0/24');
    expect(within(dialog).getByText('This rule is already in the list.')).toBeInTheDocument();

    await user.clear(target());
    await user.selectOptions(within(dialog).getByLabelText('Target type'), 'peer');
    await add();
    expect(within(dialog).getByText('Choose a peer.')).toBeInTheDocument();
    await user.selectOptions(within(dialog).getByLabelText('Target type'), 'group');
    await add();
    expect(within(dialog).getByText('Choose a group.')).toBeInTheDocument();
    await user.selectOptions(within(dialog).getByLabelText('Target type'), 'route');
    await add();
    expect(within(dialog).getByText('Choose a route.')).toBeInTheDocument();

    expect(within(ruleList(dialog)).getAllByRole('listitem')).toHaveLength(1);
    expect(requests).toHaveLength(0);
  });

  it('does not save while a rule is written but not added', async () => {
    const { net } = seedCorp();
    const { user } = renderApp('/policies');
    const dialog = await startPolicy(user);
    await openTab(user, dialog, 'Rules');
    await addCidrRule(user, dialog, '10.1.0.0/16');
    await user.type(within(dialog).getByRole('textbox', { name: 'Target' }), '10.2.0.0/16');
    await openTab(user, dialog, 'Details');

    await user.click(within(dialog).getByRole('button', { name: 'Create policy' }));

    // Back on the Rules tab, with the reason.
    expect(await within(dialog).findByText('Add this rule with “Add rule”, or clear it, before saving.')).toBeInTheDocument();
    expect(within(dialog).getByRole('tab', { name: /^Rules/ })).toHaveAttribute('aria-selected', 'true');
    expect(requests).toHaveLength(0);

    await user.click(within(dialog).getByRole('button', { name: 'Clear' }));
    await user.click(within(dialog).getByRole('button', { name: 'Create policy' }));

    await screen.findByText('Policy “allow-web” created');
    expect(requestsTo('POST', /\/policies$/)[0]).toEqual({
      method: 'POST',
      path: `/networks/${net.id}/policies`,
      body: {
        name: 'allow-web',
        description: '',
        rules: [{ direction: 'output', action: 'allow', target_type: 'cidr', target: '10.1.0.0/16', description: '' }],
      },
    });
  });

  it('shows the server error in the dialog', async () => {
    const { net } = seedCorp();
    seedPolicy(net.id, { name: 'allow-web' });
    const { user } = renderApp('/policies');
    const dialog = await startPolicy(user);

    await user.click(within(dialog).getByRole('button', { name: 'Create policy' }));

    expect(await within(dialog).findByRole('alert')).toHaveTextContent('a policy with this name already exists');
    expect(screen.getByRole('dialog', { name: 'New policy' })).toBeInTheDocument();
  });

  it('keeps editing the created policy when a group attachment fails, instead of creating it twice', async () => {
    const { eng } = seedCorp();
    server.use(
      http.post('*/api/v1/networks/:id/groups/:groupId/policies/:policyId', () =>
        HttpResponse.json({ error: 'group is locked' }, { status: 409 }),
      ),
    );
    const { user } = renderApp('/policies');
    let dialog = await startPolicy(user);
    await openTab(user, dialog, 'Rules');
    await addCidrRule(user, dialog, '10.1.0.0/16');
    await openTab(user, dialog, 'Groups');
    await pick(user, dialog, 'Attach to a group', 'engineering');

    await user.click(within(dialog).getByRole('button', { name: 'Create policy' }));

    dialog = await findDialog('Edit policy allow-web');
    expect(within(dialog).getByRole('alert')).toHaveTextContent(
      'The policy was created, but not attached to all its groups: group is locked',
    );
    // A second submit saves the same policy, without adding its rules again.
    server.resetHandlers();
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));
    await screen.findByText('Policy “allow-web” saved');
    expect(requestsTo('POST', /\/policies$/)).toHaveLength(1);
    expect(requestsTo('POST', /\/rules$/)).toHaveLength(0);
    const policy = db.policies.find((p) => p.name === 'allow-web')!;
    expect(policy.rules.map((r) => r.target)).toEqual(['10.1.0.0/16']);
    expect(db.groups.find((g) => g.id === eng.id)!.policy_ids).toEqual([policy.id]);
  });

  it('edits a policy from the all-networks view: details, rules and groups are saved together', async () => {
    const { net, alice, eng, ops } = seedCorp();
    const policy = seedPolicy(net.id, {
      name: 'allow-web',
      description: 'Web',
      rules: [
        rule({ id: 'rule-a', target: '10.1.0.0/16', description: 'Old range' }),
        rule({ id: 'rule-b', action: 'deny', target_type: 'peer', target: alice.id }),
      ],
    });
    eng.policy_ids.push(policy.id);
    const { user } = renderApp('/policies');

    await user.click(await screen.findByRole('button', { name: 'Edit allow-web' }));
    const dialog = await findDialog('Edit policy allow-web');
    expect(within(dialog).getByText('In network corp. Changes apply when you save.')).toBeInTheDocument();
    await user.clear(within(dialog).getByLabelText(/^Name/));
    await user.type(within(dialog).getByLabelText(/^Name/), 'allow-http');
    await user.clear(within(dialog).getByLabelText('Description'));
    await user.type(within(dialog).getByLabelText('Description'), 'HTTP only');

    await openTab(user, dialog, 'Rules');
    const rows = within(ruleList(dialog)).getAllByRole('listitem');
    expect(rows[0]).toHaveTextContent('10.1.0.0/16');
    expect(rows[1]).toHaveTextContent('DenyOutputPeeralice-laptop');
    await user.click(within(dialog).getByRole('button', { name: 'Remove rule 1' }));
    await addCidrRule(user, dialog, '10.2.0.0/16');

    await openTab(user, dialog, 'Groups');
    await user.click(await within(dialog).findByRole('button', { name: 'Remove engineering' }));
    await pick(user, dialog, 'Attach to a group', 'ops');

    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

    await screen.findByText('Policy “allow-http” saved');
    const base = `/networks/${net.id}/policies/${policy.id}`;
    expect(requestsTo('PUT', /\/policies\//)).toEqual([
      { method: 'PUT', path: base, body: { name: 'allow-http', description: 'HTTP only' } },
    ]);
    expect(requestsTo('DELETE', /\/rules\//).map((r) => r.path)).toEqual([`${base}/rules/rule-a`]);
    expect(requestsTo('POST', /\/rules$/)).toEqual([
      {
        method: 'POST',
        path: `${base}/rules`,
        body: { direction: 'output', action: 'allow', target_type: 'cidr', target: '10.2.0.0/16', description: '' },
      },
    ]);
    // The policy's own network is used, not the page filter (all networks).
    expect(requestsTo('DELETE', /\/groups\//).map((r) => r.path)).toEqual([
      `/networks/${net.id}/groups/${eng.id}/policies/${policy.id}`,
    ]);
    expect(requestsTo('POST', /\/groups\//).map((r) => r.path)).toEqual([
      `/networks/${net.id}/groups/${ops.id}/policies/${policy.id}`,
    ]);
    const saved = db.policies.find((p) => p.id === policy.id)!;
    expect(saved.rules.map((r) => r.target)).toEqual([alice.id, '10.2.0.0/16']);
    expect(db.groups.find((g) => g.id === eng.id)!.policy_ids).toEqual([]);
    expect(db.groups.find((g) => g.id === ops.id)!.policy_ids).toEqual([policy.id]);
  });

  it('shows a failed change in the dialog and does not repeat what was saved on retry', async () => {
    const { net } = seedCorp();
    const policy = seedPolicy(net.id, { name: 'allow-web', rules: [rule({ id: 'rule-a' })] });
    server.use(
      http.post('*/api/v1/networks/:id/policies/:policyId/rules', async ({ request }) => {
        const body = (await request.clone().json()) as PolicyRule;
        // Only the second new rule fails; the first one goes to the fake API.
        if (body.target === '10.9.0.0/16') return HttpResponse.json({ error: 'rule rejected' }, { status: 400 });
      }),
    );
    const { user } = renderApp('/policies');
    await user.click(await screen.findByRole('button', { name: 'Edit allow-web' }));
    const dialog = await findDialog('Edit policy allow-web');
    await openTab(user, dialog, 'Rules');
    await addCidrRule(user, dialog, '10.8.0.0/16');
    await addCidrRule(user, dialog, '10.9.0.0/16');

    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

    expect(await within(dialog).findByRole('alert')).toHaveTextContent('rule rejected');
    server.resetHandlers();
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

    await screen.findByText('Policy “allow-web” saved');
    expect(requestsTo('POST', /\/rules$/).map((r) => (r.body as PolicyRule).target)).toEqual(['10.8.0.0/16', '10.9.0.0/16']);
    expect(requestsTo('DELETE', /\/rules\//)).toHaveLength(0);
    expect(db.policies.find((p) => p.id === policy.id)!.rules.map((r) => r.target)).toEqual([
      '10.1.0.0/16',
      '10.8.0.0/16',
      '10.9.0.0/16',
    ]);
  });

  it('sends nothing when the edit is cancelled', async () => {
    const { net, eng } = seedCorp();
    const policy = seedPolicy(net.id, { name: 'allow-web', rules: [rule({ id: 'rule-a' })] });
    eng.policy_ids.push(policy.id);
    const { user } = renderApp('/policies');

    await user.click(await screen.findByRole('button', { name: 'Edit allow-web' }));
    let dialog = await findDialog('Edit policy allow-web');
    await user.clear(within(dialog).getByLabelText(/^Name/));
    await user.type(within(dialog).getByLabelText(/^Name/), 'renamed');
    await openTab(user, dialog, 'Rules');
    await user.click(within(dialog).getByRole('button', { name: 'Remove rule 1' }));
    await openTab(user, dialog, 'Groups');
    await user.click(await within(dialog).findByRole('button', { name: 'Remove engineering' }));
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(requests).toHaveLength(0);

    // Reopened, the form shows the saved policy again.
    await user.click(screen.getByRole('button', { name: 'Edit allow-web' }));
    dialog = await findDialog('Edit policy allow-web');
    expect(within(dialog).getByLabelText(/^Name/)).toHaveValue('allow-web');
    expect(within(dialog).getByRole('tab', { name: /^Rules/ })).toHaveTextContent('1');
  });

  it('deletes a policy after confirmation, in its own network', async () => {
    const { net } = seedCorp();
    const policy = seedPolicy(net.id, { name: 'allow-web' });
    const { user } = renderApp('/policies');

    await user.click(await screen.findByRole('button', { name: 'Delete allow-web' }));
    let confirm = await findDialog('Delete policy “allow-web”?');
    await user.click(within(confirm).getByRole('button', { name: 'Cancel' }));
    expect(requests).toHaveLength(0);

    await user.click(screen.getByRole('button', { name: 'Delete allow-web' }));
    confirm = await findDialog('Delete policy “allow-web”?');
    await user.click(within(confirm).getByRole('button', { name: 'Delete policy' }));

    await screen.findByText('Policy “allow-web” deleted');
    expect(requestsTo('DELETE', /\/policies\//)).toEqual([
      { method: 'DELETE', path: `/networks/${net.id}/policies/${policy.id}`, body: undefined },
    ]);
    await waitFor(() => expect(screen.queryByRole('table')).not.toBeInTheDocument());
    expect(screen.getByText('No policy yet')).toBeInTheDocument();
  });

  it('shows a failed deletion in a toast', async () => {
    const { net } = seedCorp();
    seedPolicy(net.id, { name: 'allow-web' });
    server.use(
      http.delete('*/api/v1/networks/:id/policies/:policyId', () =>
        HttpResponse.json({ error: 'policy is in use' }, { status: 409 }),
      ),
    );
    const { user } = renderApp('/policies');

    await user.click(await screen.findByRole('button', { name: 'Delete allow-web' }));
    const confirm = await findDialog('Delete policy “allow-web”?');
    await user.click(within(confirm).getByRole('button', { name: 'Delete policy' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('policy is in use');
    expect(screen.getByRole('row', { name: /allow-web/ })).toBeInTheDocument();
  });

  it('is reserved to administrators', async () => {
    db.me = { ...db.me, role: 'user' };
    renderApp('/policies');
    expect(await screen.findByText('Only administrators can manage policies.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'New policy' })).not.toBeInTheDocument();
  });
});
