import { screen, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { server } from '../../test/setup';
import { db, seedNetwork, seedPeer, seedUser } from '../../test/fakeApi';
import { renderApp } from '../../test/renderApp';

/** The text of a label and its value (a stat, a count). */
const entry = (label: string) => screen.getByText(label).parentElement;
/** A stat card of the page (the sidebar has links named alike). */
const stat = (name: RegExp) => within(screen.getByRole('main')).getByRole('link', { name });

describe('Dashboard', () => {
  it('sums up networks, peers, address use and users', async () => {
    const corp = seedNetwork({ name: 'corp', cidr: '10.42.0.0/24' });
    const lab = seedNetwork({ name: 'lab', cidr: '10.43.0.0/28', cidr_v6: 'fd00:43::/64' });
    seedPeer(corp.id, { name: 'jump-paris', is_jump: true, use_agent: true });
    seedPeer(corp.id, { name: 'alice-laptop', use_agent: true });
    seedPeer(corp.id, { name: 'bob-laptop' });
    seedPeer(lab.id, { name: 'ci-runner' });
    seedUser({ email: 'bob@example.com', role: 'user' });
    seedUser({ email: 'carol@example.com', role: 'administrator' });
    renderApp('/dashboard');

    expect(await screen.findByRole('heading', { level: 1, name: 'Dashboard' })).toBeInTheDocument();
    expect(await within(screen.getByRole('main')).findByRole('link', { name: /^Networks/ })).toHaveTextContent('Networks2');
    expect(stat(/^Peers/)).toHaveTextContent('Peers4');
    // 4 peers in 254 + 14 IPv4 addresses.
    expect(stat(/^IP utilization/)).toHaveTextContent('1%4 of 268 IPv4 addresses used');
    expect(stat(/^IP utilization/)).toHaveAttribute('href', '/ipam');

    expect(await screen.findByText('4 peers')).toBeInTheDocument();
    expect(entry('Jump servers')).toHaveTextContent('1');
    expect(entry('Regular')).toHaveTextContent('3');
    expect(entry('Agent-managed')).toHaveTextContent('2');
    expect(entry('Static config')).toHaveTextContent('2');
    expect(stat(/^Peers/)).toHaveTextContent('1 jump server · 2 with the agent');

    expect(await screen.findByText('Total users')).toBeInTheDocument();
    expect(entry('Total users')).toHaveTextContent('3');
    expect(entry('Administrators')).toHaveTextContent('2');
    expect(entry('Regular users')).toHaveTextContent('1');
    expect(screen.getByRole('link', { name: 'View all users' })).toHaveAttribute('href', '/users');
  });

  it('lists the five most recent networks, which lead to the networks page', async () => {
    for (let day = 1; day <= 6; day++) {
      seedNetwork({ name: `net-${day}`, cidr: `10.${day}.0.0/24`, created_at: `2026-10-0${day}T10:00:00Z` });
    }
    const { user } = renderApp('/dashboard');

    const recent = await screen.findByRole('list', { name: 'Recent networks' });
    const links = within(recent).getAllByRole('link');
    expect(links.map((l) => l.textContent)).toEqual([
      'net-610.6.0.0/240 peers',
      'net-510.5.0.0/240 peers',
      'net-410.4.0.0/240 peers',
      'net-310.3.0.0/240 peers',
      'net-210.2.0.0/240 peers',
    ]);
    links.forEach((l) => expect(l).toHaveAttribute('href', '/networks'));

    await user.click(links[0]);

    expect(await screen.findByRole('heading', { level: 1, name: 'Networks' })).toBeInTheDocument();
  });

  it('starts empty', async () => {
    renderApp('/dashboard');

    expect(await screen.findByText('No network yet.')).toBeInTheDocument();
    expect(stat(/^Networks/)).toHaveTextContent('Networks0');
    expect(stat(/^IP utilization/)).toHaveTextContent('0%');
    expect(screen.getByRole('link', { name: 'View all networks' })).toHaveAttribute('href', '/networks');
  });

  it('shows no user summary to users', async () => {
    db.me = { ...db.me, role: 'user' };
    seedNetwork({ name: 'corp' });
    renderApp('/dashboard');

    expect(await screen.findByRole('list', { name: 'Recent networks' })).toBeInTheDocument();
    expect(screen.queryByText('Total users')).not.toBeInTheDocument();
  });

  it('shows why the dashboard could not be loaded', async () => {
    server.use(http.get('*/api/v1/networks', () => HttpResponse.json({ error: 'database is down' }, { status: 500 })));
    renderApp('/dashboard');

    expect(await screen.findByRole('alert')).toHaveTextContent('database is down');
  });
});
