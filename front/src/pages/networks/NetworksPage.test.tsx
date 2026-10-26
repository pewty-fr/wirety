import { screen, waitFor, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { server } from '../../test/setup';
import { db, requests, requestsTo, seedNetwork, seedPeer } from '../../test/fakeApi';
import { findDialog } from '../../test/helpers';
import { renderApp } from '../../test/renderApp';

const NETWORKS = /^\/networks$/;
const ONE_NETWORK = /^\/networks\/[^/]+$/;

async function openNewNetwork(user: ReturnType<typeof renderApp>['user']) {
  await user.click(await screen.findByRole('button', { name: 'New network' }));
  return findDialog('New network');
}

describe('Networks', () => {
  it('lists the networks with their ranges, domain, peers and free addresses', async () => {
    const corp = seedNetwork({ name: 'corp', cidr: '10.42.0.0/24', cidr_v6: 'fd12:3456:789a:1::/64' });
    seedPeer(corp.id);
    seedPeer(corp.id);
    seedNetwork({ name: 'lab', cidr: '', cidr_v6: 'fd00:1::/64', domain_suffix: 'example.com' });
    renderApp('/networks');

    const table = await screen.findByRole('table');
    const corpRow = within(table).getByRole('row', { name: /^corp\b/ });
    expect(within(corpRow).getByText('10.42.0.0/24')).toBeInTheDocument();
    expect(within(corpRow).getByText('fd12:3456:789a:1::/64')).toBeInTheDocument();
    expect(within(corpRow).getByText('corp.internal')).toBeInTheDocument();
    expect(within(corpRow).getByText('2')).toBeInTheDocument();
    expect(within(corpRow).getByText('252 / 254')).toBeInTheDocument();
    const labRow = within(table).getByRole('row', { name: /^lab\b/ });
    expect(within(labRow).getByText('lab.example.com')).toBeInTheDocument();
    expect(within(labRow).getByText('—')).toBeInTheDocument();
  });

  it('searches the networks', async () => {
    seedNetwork({ name: 'corp' });
    seedNetwork({ name: 'lab', cidr: '10.43.0.0/24' });
    const { user } = renderApp('/networks');
    await screen.findByText('corp');

    await user.type(screen.getByLabelText('Search'), 'la');

    await waitFor(() => expect(screen.queryByText('corp')).not.toBeInTheDocument());
    expect(screen.getByText('lab')).toBeInTheDocument();

    await user.type(screen.getByLabelText('Search'), 'zzz');
    expect(await screen.findByText('No network matches “lazzz”')).toBeInTheDocument();
  });

  it('pages through the networks', async () => {
    for (let i = 1; i <= 25; i++) seedNetwork({ name: `net-${String(i).padStart(2, '0')}`, cidr: `10.${i}.0.0/24` });
    const { user } = renderApp('/networks');

    expect(await screen.findByText('net-01')).toBeInTheDocument();
    expect(screen.queryByText('net-21')).not.toBeInTheDocument();
    const pagination = screen.getByRole('navigation', { name: 'Pagination' });
    expect(pagination).toHaveTextContent('1–20 of 25');

    await user.click(within(pagination).getByRole('button', { name: 'Next' }));

    expect(await screen.findByText('net-21')).toBeInTheDocument();
    expect(screen.queryByText('net-01')).not.toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Pagination' })).toHaveTextContent('21–25 of 25');
  });

  it('creates a network with its DNS servers and domain suffix', async () => {
    const { user } = renderApp('/networks');
    const dialog = await openNewNetwork(user);

    await user.type(within(dialog).getByLabelText(/^Name/), 'prod.eu');
    await user.type(within(dialog).getByLabelText('IPv4 CIDR'), '10.10.0.0/16');
    // Enter adds a server (and does not submit the form); so does Add.
    await user.type(within(dialog).getByLabelText('DNS servers'), '1.1.1.1{Enter}');
    expect(screen.getByRole('dialog', { name: 'New network' })).toBeInTheDocument();
    await user.type(within(dialog).getByLabelText('DNS servers'), '2606:4700:4700::1111');
    await user.click(within(dialog).getByRole('button', { name: 'Add' }));
    await user.type(within(dialog).getByLabelText('DNS servers'), '9.9.9.9{Enter}');
    await user.click(within(dialog).getByRole('button', { name: 'Remove 9.9.9.9' }));
    expect(within(within(dialog).getByRole('list', { name: 'Added DNS servers' })).getAllByRole('listitem')).toHaveLength(2);
    await user.clear(within(dialog).getByLabelText(/^Domain suffix/));
    await user.type(within(dialog).getByLabelText(/^Domain suffix/), 'corp.example.com');

    await user.click(within(dialog).getByRole('button', { name: 'Create network' }));

    expect(await screen.findByText('Network “prod.eu” created')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New network' })).not.toBeInTheDocument());
    expect(requestsTo('POST', NETWORKS)).toHaveLength(1);
    expect(requestsTo('POST', NETWORKS)[0].body).toEqual({
      name: 'prod.eu',
      cidr: '10.10.0.0/16',
      dns: ['1.1.1.1', '2606:4700:4700::1111'],
      domain_suffix: 'corp.example.com',
    });
    expect(within(await screen.findByRole('table')).getByText('prod.eu')).toBeInTheDocument();
  });

  it('suggests free IPv4 ranges for the expected number of peers', async () => {
    let asked: URLSearchParams | undefined;
    server.use(
      http.get('*/api/v1/ipam/available-cidrs', ({ request }) => {
        asked = new URL(request.url).searchParams;
        return HttpResponse.json({
          base_cidr: '10.0.0.0/8',
          requested_max_peers: 50,
          suggested_prefix: 26,
          usable_hosts: 62,
          cidrs: ['10.1.0.0/26', '10.1.0.64/26'],
        });
      }),
    );
    const { user } = renderApp('/networks');
    const dialog = await openNewNetwork(user);

    await user.clear(within(dialog).getByLabelText('Expected max peers'));
    await user.type(within(dialog).getByLabelText('Expected max peers'), '50');
    await user.click(within(dialog).getByRole('button', { name: 'Suggest' }));

    const ranges = await within(dialog).findByRole('group', { name: 'Suggested IPv4 ranges' });
    expect(asked?.get('max_peers')).toBe('50');
    expect(within(dialog).getByText('Free /26 ranges (62 addresses each). Pick one:')).toBeInTheDocument();
    await user.click(within(ranges).getByRole('button', { name: '10.1.0.64/26' }));
    expect(within(dialog).getByLabelText('IPv4 CIDR')).toHaveValue('10.1.0.64/26');
    expect(within(dialog).queryByRole('group', { name: 'Suggested IPv4 ranges' })).not.toBeInTheDocument();

    await user.type(within(dialog).getByLabelText(/^Name/), 'branch');
    await user.click(within(dialog).getByRole('button', { name: 'Create network' }));

    await screen.findByText('Network “branch” created');
    expect(requestsTo('POST', NETWORKS)[0].body).toEqual({ name: 'branch', cidr: '10.1.0.64/26', domain_suffix: 'internal' });
  });

  it('checks the number of peers before suggesting ranges, and shows why it failed', async () => {
    server.use(
      http.get('*/api/v1/ipam/available-cidrs', () => HttpResponse.json({ error: 'no free range left' }, { status: 409 })),
    );
    const { user } = renderApp('/networks');
    const dialog = await openNewNetwork(user);

    await user.clear(within(dialog).getByLabelText('Expected max peers'));
    await user.click(within(dialog).getByRole('button', { name: 'Suggest' }));
    expect(within(dialog).getByText('Enter a whole number of peers, at least 1.')).toBeInTheDocument();

    await user.type(within(dialog).getByLabelText('Expected max peers'), '5000');
    await user.click(within(dialog).getByRole('button', { name: 'Suggest' }));
    expect(await within(dialog).findByText('no free range left')).toBeInTheDocument();
  });

  it('suggests IPv6 unique local prefixes for an IPv6-only network', async () => {
    const { user } = renderApp('/networks');
    const dialog = await openNewNetwork(user);

    await user.click(within(dialog).getByRole('button', { name: 'Suggest ULA prefixes' }));
    const prefixes = within(dialog).getByRole('group', { name: 'Suggested IPv6 prefixes' });
    const chips = within(prefixes).getAllByRole('button');
    expect(chips).toHaveLength(5);
    const ula = chips[2].textContent ?? '';
    expect(ula).toMatch(/^fd[0-9a-f]{2}:[0-9a-f]{4}:[0-9a-f]{4}:[0-9a-f]{4}::\/64$/);
    await user.click(chips[2]);
    expect(within(dialog).getByLabelText('IPv6 CIDR')).toHaveValue(ula);
    expect(within(dialog).queryByRole('group', { name: 'Suggested IPv6 prefixes' })).not.toBeInTheDocument();

    await user.type(within(dialog).getByLabelText(/^Name/), 'v6only');
    await user.click(within(dialog).getByRole('button', { name: 'Create network' }));

    await screen.findByText('Network “v6only” created');
    expect(requestsTo('POST', NETWORKS)[0].body).toEqual({ name: 'v6only', cidr_v6: ula, domain_suffix: 'internal' });
  });

  it('checks the form before sending anything', async () => {
    const { user } = renderApp('/networks');
    const dialog = await openNewNetwork(user);

    await user.clear(within(dialog).getByLabelText(/^Domain suffix/));
    await user.click(within(dialog).getByRole('button', { name: 'Create network' }));

    expect(within(dialog).getByText('Give the network a name.')).toBeInTheDocument();
    expect(within(dialog).getByText('Enter an IPv4 CIDR, an IPv6 CIDR, or both.')).toBeInTheDocument();
    expect(within(dialog).getByText('Enter a domain suffix, e.g. internal.')).toBeInTheDocument();
    expect(within(dialog).getByLabelText(/^Name/)).toHaveAttribute('aria-invalid', 'true');

    // Wrong formats; the CIDRs are checked as they are typed.
    await user.type(within(dialog).getByLabelText(/^Name/), 'Prod_EU');
    await user.type(within(dialog).getByLabelText('IPv4 CIDR'), '10.10.1.0/16');
    expect(within(dialog).getByText('10.10.1.0 has host bits set — did you mean 10.10.0.0/16?')).toBeInTheDocument();
    await user.type(within(dialog).getByLabelText('IPv6 CIDR'), '10.20.0.0/16');
    expect(within(dialog).getByText('This is an IPv4 range: enter it in the IPv4 CIDR field.')).toBeInTheDocument();
    await user.type(within(dialog).getByLabelText('DNS servers'), '8.8.8{Enter}');
    expect(within(dialog).getByText('Enter an IPv4 or IPv6 address, e.g. 1.1.1.1.')).toBeInTheDocument();
    expect(within(dialog).queryByRole('list', { name: 'Added DNS servers' })).not.toBeInTheDocument();
    await user.type(within(dialog).getByLabelText(/^Domain suffix/), '-corp');

    await user.click(within(dialog).getByRole('button', { name: 'Create network' }));

    expect(within(dialog).getByText('Use lowercase letters, digits, hyphens and dots, e.g. prod or prod.eu.')).toBeInTheDocument();
    expect(within(dialog).getByText('10.10.1.0 has host bits set — did you mean 10.10.0.0/16?')).toBeInTheDocument();
    expect(within(dialog).getByText('This is an IPv4 range: enter it in the IPv4 CIDR field.')).toBeInTheDocument();
    expect(within(dialog).getByText('Enter an IPv4 or IPv6 address, e.g. 1.1.1.1.')).toBeInTheDocument();
    expect(
      within(dialog).getByText('Use lowercase letters, digits, hyphens and dots, e.g. internal or corp.example.com.'),
    ).toBeInTheDocument();
    expect(requestsTo('POST', NETWORKS)).toHaveLength(0);
  });

  it('shows the server error in the dialog', async () => {
    seedNetwork({ name: 'corp' });
    const { user } = renderApp('/networks');
    const dialog = await openNewNetwork(user);
    await user.type(within(dialog).getByLabelText(/^Name/), 'corp');
    await user.type(within(dialog).getByLabelText('IPv4 CIDR'), '10.50.0.0/24');

    await user.click(within(dialog).getByRole('button', { name: 'Create network' }));

    expect(await within(dialog).findByRole('alert')).toHaveTextContent('a network with this name already exists');
    expect(screen.getByRole('dialog', { name: 'New network' })).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Create network' })).toBeEnabled();
  });

  it('starts each form afresh and sends nothing on cancel', async () => {
    seedNetwork({ name: 'corp', cidr: '10.42.0.0/24' });
    const { user } = renderApp('/networks');
    let dialog = await openNewNetwork(user);
    await user.type(within(dialog).getByLabelText(/^Name/), 'draft');
    await user.type(within(dialog).getByLabelText('IPv4 CIDR'), '10.0.0.1/8');
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New network' })).not.toBeInTheDocument());

    dialog = await openNewNetwork(user);
    expect(within(dialog).getByLabelText(/^Name/)).toHaveValue('');
    expect(within(dialog).getByLabelText('IPv4 CIDR')).toHaveValue('');
    expect(within(dialog).queryByText(/has host bits set/)).not.toBeInTheDocument();
    expect(within(dialog).getByLabelText(/^Domain suffix/)).toHaveValue('internal');
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));

    await user.click(screen.getByRole('button', { name: 'Edit corp' }));
    dialog = await findDialog('Edit network');
    await user.clear(within(dialog).getByLabelText(/^Name/));
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Edit network' })).not.toBeInTheDocument());
    await user.click(screen.getByRole('button', { name: 'Edit corp' }));
    dialog = await findDialog('Edit network');
    expect(within(dialog).getByLabelText(/^Name/)).toHaveValue('corp');

    expect(requests).toHaveLength(0);
  });

  it('edits a network', async () => {
    const corp = seedNetwork({
      name: 'corp',
      cidr: '10.42.0.0/24',
      dns: ['1.1.1.1', '9.9.9.9'],
      domain_suffix: 'internal',
      default_group_ids: ['grp-default'],
    });
    const { user } = renderApp('/networks');

    await user.click(await screen.findByRole('button', { name: 'Edit corp' }));
    const dialog = await findDialog('Edit network');
    expect(within(dialog).getByLabelText(/^Name/)).toHaveValue('corp');
    expect(within(dialog).getByLabelText('IPv4 CIDR')).toHaveValue('10.42.0.0/24');
    expect(within(dialog).getByLabelText(/^Domain suffix/)).toHaveValue('internal');
    expect(within(dialog).queryByLabelText('Expected max peers')).not.toBeInTheDocument();
    await user.clear(within(dialog).getByLabelText(/^Name/));
    await user.type(within(dialog).getByLabelText(/^Name/), 'corp-eu');
    await user.click(within(dialog).getByRole('button', { name: 'Remove 1.1.1.1' }));
    // A server typed but not added is saved too.
    await user.type(within(dialog).getByLabelText('DNS servers'), '8.8.8.8');

    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

    expect(await screen.findByText('Network “corp-eu” saved')).toBeInTheDocument();
    expect(requestsTo('PUT', ONE_NETWORK)).toEqual([
      {
        method: 'PUT',
        path: `/networks/${corp.id}`,
        body: {
          name: 'corp-eu',
          cidr: '10.42.0.0/24',
          dns: ['9.9.9.9', '8.8.8.8'],
          domain_suffix: 'internal',
          default_group_ids: ['grp-default'],
        },
      },
    ]);
    expect(within(await screen.findByRole('table')).getByText('corp-eu')).toBeInTheDocument();
  });

  it('can change the range of a network but not remove it', async () => {
    seedNetwork({ name: 'corp', cidr: '10.42.0.0/24', cidr_v6: 'fd00:42::/64' });
    const { user } = renderApp('/networks');
    await user.click(await screen.findByRole('button', { name: 'Edit corp' }));
    const dialog = await findDialog('Edit network');

    await user.clear(within(dialog).getByLabelText('IPv4 CIDR'));
    await user.type(within(dialog).getByLabelText('IPv4 CIDR'), '10.43.0.0/24');
    expect(within(dialog).getByText('Saving gives every peer a new address in this range.')).toBeInTheDocument();
    await user.clear(within(dialog).getByLabelText('IPv6 CIDR'));
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

    expect(within(dialog).getByText('The IPv6 range of a network can be changed, not removed.')).toBeInTheDocument();
    expect(requestsTo('PUT', ONE_NETWORK)).toHaveLength(0);
  });

  it('shows the details of a network, fresh after an edit', async () => {
    const corp = seedNetwork({ name: 'corp', cidr: '10.42.0.0/24', dns: ['1.1.1.1'] });
    seedPeer(corp.id);
    seedPeer(corp.id);
    seedPeer(corp.id);
    const { user } = renderApp('/networks');

    await user.click(await screen.findByRole('button', { name: 'corp' }));
    const details = await findDialog('corp');
    expect(within(details).getByText('10.42.0.0/24')).toBeInTheDocument();
    expect(within(details).getByText('corp.internal')).toBeInTheDocument();
    expect(within(details).getByText('1.1.1.1')).toBeInTheDocument();
    expect(within(details).getByText('Addresses').parentElement).toHaveTextContent('254');
    expect(within(details).getByText('Used').parentElement).toHaveTextContent('3');
    expect(within(details).getByText('Free').parentElement).toHaveTextContent('251');
    expect(within(details).getByRole('progressbar', { name: 'IPv4 utilization' })).toHaveAttribute('aria-valuenow', '1');

    await user.click(within(details).getByRole('button', { name: 'Edit network' }));
    const edit = await findDialog('Edit network');
    await user.clear(within(edit).getByLabelText(/^Name/));
    await user.type(within(edit).getByLabelText(/^Name/), 'corp-eu');
    await user.clear(within(edit).getByLabelText(/^Domain suffix/));
    await user.type(within(edit).getByLabelText(/^Domain suffix/), 'example.com');
    await user.click(within(edit).getByRole('button', { name: 'Save changes' }));

    await screen.findByText('Network “corp-eu” saved');
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Edit network' })).not.toBeInTheDocument());
    const fresh = await findDialog('corp-eu');
    expect(within(fresh).getByText('corp-eu.example.com')).toBeInTheDocument();
    expect(within(fresh).getByText('Used').parentElement).toHaveTextContent('3');
  });

  it('deletes a network after confirmation', async () => {
    const corp = seedNetwork({ name: 'corp' });
    const { user } = renderApp('/networks');

    await user.click(await screen.findByRole('button', { name: 'Delete corp' }));
    let confirm = await findDialog('Delete network “corp”?');
    expect(confirm).toHaveTextContent('This also deletes all its peers');
    await user.click(within(confirm).getByRole('button', { name: 'Cancel' }));
    expect(requestsTo('DELETE', ONE_NETWORK)).toHaveLength(0);

    await user.click(screen.getByRole('button', { name: 'Delete corp' }));
    confirm = await findDialog('Delete network “corp”?');
    await user.click(within(confirm).getByRole('button', { name: 'Delete network' }));

    await screen.findByText('Network “corp” deleted');
    expect(requestsTo('DELETE', ONE_NETWORK)[0].path).toBe(`/networks/${corp.id}`);
    expect(await screen.findByText('No network yet')).toBeInTheDocument();
  });

  it('deletes a network from its details', async () => {
    const corp = seedNetwork({ name: 'corp' });
    seedNetwork({ name: 'lab', cidr: '10.43.0.0/24' });
    const { user } = renderApp('/networks');

    await user.click(await screen.findByRole('button', { name: 'corp' }));
    const details = await findDialog('corp');
    await user.click(within(details).getByRole('button', { name: 'Delete network' }));
    const confirm = await findDialog('Delete network “corp”?');
    await user.click(within(confirm).getByRole('button', { name: 'Delete network' }));

    await screen.findByText('Network “corp” deleted');
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'corp' })).not.toBeInTheDocument());
    expect(requestsTo('DELETE', ONE_NETWORK).map((r) => r.path)).toEqual([`/networks/${corp.id}`]);
    await waitFor(() => expect(screen.queryByRole('button', { name: 'corp' })).not.toBeInTheDocument());
    expect(screen.getByRole('button', { name: 'lab' })).toBeInTheDocument();
  });

  it('tells why a network could not be deleted', async () => {
    seedNetwork({ name: 'corp' });
    server.use(
      http.delete('*/api/v1/networks/:id', () => HttpResponse.json({ error: 'network is in use' }, { status: 409 })),
    );
    const { user } = renderApp('/networks');

    await user.click(await screen.findByRole('button', { name: 'Delete corp' }));
    const confirm = await findDialog('Delete network “corp”?');
    await user.click(within(confirm).getByRole('button', { name: 'Delete network' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('network is in use');
    expect(screen.getByRole('button', { name: 'corp' })).toBeInTheDocument();
  });

  it('lets users read the networks but not change them', async () => {
    db.me = { ...db.me, role: 'user' };
    seedNetwork({ name: 'corp', cidr: '10.42.0.0/24' });
    const { user } = renderApp('/networks');

    await screen.findByRole('table');
    expect(screen.queryByRole('button', { name: 'New network' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Edit corp' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Delete corp' })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'corp' }));
    const details = await findDialog('corp');
    expect(within(details).getByText('10.42.0.0/24')).toBeInTheDocument();
    expect(within(details).queryByRole('button', { name: 'Edit network' })).not.toBeInTheDocument();
    expect(within(details).queryByRole('button', { name: 'Delete network' })).not.toBeInTheDocument();
    await user.click(within(details).getByRole('button', { name: 'Close' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'corp' })).not.toBeInTheDocument());
  });
});
