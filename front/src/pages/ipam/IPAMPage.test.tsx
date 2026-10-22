import { screen, waitFor, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { server } from '../../test/setup';
import { renderApp } from '../../test/renderApp';
import type { IPAMAllocation } from '../../types';

/** Serves `all` from the IPAM endpoint, filtered and paginated like the server; returns the queries received. */
function serveAllocations(all: IPAMAllocation[]) {
  const asked: URLSearchParams[] = [];
  server.use(
    http.get('*/api/v1/ipam', ({ request }) => {
      const params = new URL(request.url).searchParams;
      asked.push(params);
      const filter = (params.get('filter') ?? '').toLowerCase();
      const list = all.filter(
        (a) => !filter || [a.network_name, a.ip, a.peer_name ?? ''].some((v) => v.toLowerCase().includes(filter)),
      );
      const page = Number(params.get('page'));
      const size = Number(params.get('page_size'));
      return HttpResponse.json({ data: list.slice((page - 1) * size, page * size), total: list.length, page, page_size: size });
    }),
  );
  return asked;
}

const corp = { network_id: 'net-1', network_name: 'corp', network_cidr: '10.42.0.0/24' };

describe('IP address management', () => {
  it('lists the addresses with their family, network, peer and status', async () => {
    serveAllocations([
      { ...corp, family: 'ipv4', ip: '10.42.0.2', peer_id: 'peer-1', peer_name: 'alice-laptop', allocated: true },
      { ...corp, family: 'ipv4', ip: '10.42.0.3', allocated: false },
      // Older servers send no family.
      { ...corp, network_cidr: 'fd00:42::/64', ip: 'fd00:42::2', peer_id: 'peer-1', peer_name: 'alice-laptop', allocated: true },
    ]);
    renderApp('/ipam');

    const table = await screen.findByRole('table');
    expect(screen.getByRole('heading', { level: 1, name: 'IP address management' })).toBeInTheDocument();
    expect(screen.getByText('3 addresses tracked across your networks.')).toBeInTheDocument();

    const allocated = within(table).getByRole('row', { name: /^10\.42\.0\.2\b/ });
    expect(within(allocated).getByText('IPv4')).toBeInTheDocument();
    expect(within(allocated).getByText('corp')).toBeInTheDocument();
    expect(within(allocated).getByText('10.42.0.0/24')).toBeInTheDocument();
    expect(within(allocated).getByText('alice-laptop')).toBeInTheDocument();
    expect(within(allocated).getByText('Allocated')).toBeInTheDocument();

    const free = within(table).getByRole('row', { name: /^10\.42\.0\.3\b/ });
    expect(within(free).getByText('Free')).toBeInTheDocument();
    expect(within(free).getByText('—')).toBeInTheDocument();

    const v6 = within(table).getByRole('row', { name: /^fd00:42::2\b/ });
    expect(within(v6).getByText('IPv6')).toBeInTheDocument();
    expect(within(v6).getByText('fd00:42::/64')).toBeInTheDocument();
  });

  it('searches by network, IP or peer', async () => {
    const asked = serveAllocations([
      { ...corp, ip: '10.42.0.2', peer_name: 'alice-laptop', allocated: true },
      { ...corp, ip: '10.42.0.3', peer_name: 'bob-laptop', allocated: true },
    ]);
    const { user } = renderApp('/ipam');
    await screen.findByText('alice-laptop');

    await user.type(screen.getByLabelText('Search'), 'bob');

    await waitFor(() => expect(screen.queryByText('alice-laptop')).not.toBeInTheDocument());
    expect(screen.getByText('bob-laptop')).toBeInTheDocument();
    expect(asked.at(-1)?.get('filter')).toBe('bob');
    expect(asked.at(-1)?.get('page')).toBe('1');

    await user.type(screen.getByLabelText('Search'), 'zzz');
    expect(await screen.findByText('No address matches “bobzzz”')).toBeInTheDocument();
  });

  it('pages through the addresses', async () => {
    const asked = serveAllocations(
      Array.from({ length: 60 }, (_, i) => ({ ...corp, ip: `10.42.0.${i + 1}`, allocated: i % 2 === 0 })),
    );
    const { user } = renderApp('/ipam');

    expect(await screen.findByText('10.42.0.50')).toBeInTheDocument();
    expect(screen.queryByText('10.42.0.51')).not.toBeInTheDocument();
    const pagination = screen.getByRole('navigation', { name: 'Pagination' });
    expect(pagination).toHaveTextContent('1–50 of 60');

    await user.click(within(pagination).getByRole('button', { name: 'Next' }));

    expect(await screen.findByText('10.42.0.51')).toBeInTheDocument();
    expect(screen.queryByText('10.42.0.50')).not.toBeInTheDocument();
    expect(asked.at(-1)?.get('page')).toBe('2');
    expect(asked.at(-1)?.get('page_size')).toBe('50');
  });

  it('says when there is no address yet', async () => {
    renderApp('/ipam');
    expect(await screen.findByText('No address yet')).toBeInTheDocument();
  });

  it('shows why the addresses could not be loaded', async () => {
    server.use(http.get('*/api/v1/ipam', () => HttpResponse.json({ error: 'IPAM is unavailable' }, { status: 500 })));
    renderApp('/ipam');
    expect(await screen.findByRole('alert')).toHaveTextContent('IPAM is unavailable');
  });
});
