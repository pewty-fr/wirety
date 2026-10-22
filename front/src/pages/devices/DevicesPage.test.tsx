import { screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { db, requestsTo, seedNetwork, seedPeer, seedUser } from '../../test/fakeApi';
import { findDialog, minutesAgo, peerEvents, peerSessions } from '../../test/helpers';
import { renderApp } from '../../test/renderApp';

/** A user (not an administrator) with a laptop on dev and prod, and a phone on prod. */
function seedUserDevices() {
  db.me = { ...db.me, id: 'alice', name: 'Alice', role: 'user' };
  const bob = seedUser({ email: 'bob@example.com', role: 'user' });
  const dev = seedNetwork({ name: 'dev', cidr: '10.10.0.0/24' });
  const prod = seedNetwork({ name: 'prod', cidr: '10.20.0.0/24', cidr_v6: 'fd20::/64' });
  seedPeer(dev.id, { name: 'jump-dev', is_jump: true, address: '10.10.0.1' });
  seedPeer(prod.id, { name: 'jump-prod', is_jump: true, address: '10.20.0.1' });
  const devLaptop = seedPeer(dev.id, { name: 'alice-laptop', owner_id: 'alice', address: '10.10.0.2' });
  const prodLaptop = seedPeer(prod.id, { name: 'alice-laptop', owner_id: 'alice', address: '10.20.0.2' });
  const phone = seedPeer(prod.id, { name: 'alice-phone', owner_id: 'alice', address: '10.20.0.3', use_agent: true });
  seedPeer(prod.id, { name: 'bob-laptop', owner_id: bob.id, address: '10.20.0.4' });
  return { dev, prod, devLaptop, prodLaptop, phone };
}

const section = (name: string) => screen.findByRole('region', { name });

describe('My devices', () => {
  it('lists the user’s own devices by network, with the sign-in each network needs', async () => {
    const { devLaptop, prodLaptop, phone } = seedUserDevices();
    peerSessions({
      // The tunnel to prod is up, not signed in yet; dev is idle but signed in.
      [prodLaptop.id]: { current_session: { last_seen: minutesAgo(1), hostname: 'alice-mbp' } as never },
      [devLaptop.id]: { captive_portal_state: 'authenticated' },
      [phone.id]: { has_active_agent: true, captive_portal_state: 'authenticated' },
    });
    renderApp('/devices');

    const prod = await section('prod');
    const dev = await section('dev');
    expect(within(prod).getByText('10.20.0.0/24 · fd20::/64')).toBeInTheDocument();

    const laptop = await within(prod).findByRole('listitem', { name: 'alice-laptop' });
    expect(within(laptop).getByText('Not authenticated')).toBeInTheDocument();
    expect(within(laptop).getByText('Sign-in required')).toBeInTheDocument();
    expect(within(laptop).getByText(/alice-mbp/)).toBeInTheDocument();
    expect(within(within(prod).getByRole('listitem', { name: 'alice-phone' })).getByText('Connected')).toBeInTheDocument();
    // Other people's devices and jump peers are not the user's devices.
    expect(within(prod).queryByRole('listitem', { name: 'bob-laptop' })).not.toBeInTheDocument();
    expect(within(prod).queryByRole('listitem', { name: 'jump-prod' })).not.toBeInTheDocument();

    // Each network signs in through its own jump peer, over the tunnel.
    const signInProd = within(prod).getByRole('link', { name: 'Sign in' });
    expect(signInProd).toHaveAttribute('href', 'http://10.20.0.1/');
    expect(signInProd).toHaveAttribute('target', '_blank');
    expect(signInProd).toHaveAttribute('rel', expect.stringContaining('noopener'));
    expect(within(prod).getByText(/It only answers through the “prod” WireGuard tunnel/)).toBeInTheDocument();
    expect(within(dev).getByRole('link', { name: 'Sign in' })).toHaveAttribute('href', 'http://10.10.0.1/');
    expect(within(within(dev).getByRole('listitem', { name: 'alice-laptop' })).queryByText('Sign-in required')).not.toBeInTheDocument();
  });

  it('tells why a device was signed out, and since when one is signed in', async () => {
    const { devLaptop, prodLaptop } = seedUserDevices();
    peerSessions({ [devLaptop.id]: { captive_portal_state: 'authenticated' } });
    peerEvents({
      [prodLaptop.id]: [
        { event: 'tunnel_inactive', peer_ip: '10.20.0.2', created_at: minutesAgo(180) },
        { event: 'authenticated', peer_ip: '10.20.0.2', created_at: minutesAgo(600) },
      ],
      [devLaptop.id]: [{ event: 'authenticated', peer_ip: '10.10.0.2', created_at: minutesAgo(30) }],
    });
    renderApp('/devices');

    const prodLaptop_ = await within(await section('prod')).findByRole('listitem', { name: 'alice-laptop' });
    expect(await within(prodLaptop_).findByText('Tunnel disconnected')).toBeInTheDocument();
    expect(within(prodLaptop_).getByText(/3h ago — The VPN tunnel was inactive for too long/)).toBeInTheDocument();
    const devLaptop_ = within(await section('dev')).getByRole('listitem', { name: 'alice-laptop' });
    expect(await within(devLaptop_).findByText('Signed in 30m ago')).toBeInTheDocument();
  });

  it('resets the sign-in of a device someone else is signing in as', async () => {
    const { prod, prodLaptop } = seedUserDevices();
    peerSessions({ [prodLaptop.id]: { captive_portal_state: 'pending_auth' } });
    const { user } = renderApp('/devices');

    const laptop = await within(await section('prod')).findByRole('listitem', { name: 'alice-laptop' });
    expect(await within(laptop).findByText('Sign-in requested from an unknown source')).toBeInTheDocument();

    // Cancel sends nothing.
    await user.click(within(laptop).getByRole('button', { name: 'Reset sign-in' }));
    let confirm = await findDialog('Reset the sign-in of “alice-laptop”?');
    await user.click(within(confirm).getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(requestsTo('POST', /\/revoke-auth$/)).toHaveLength(0);

    await user.click(within(laptop).getByRole('button', { name: 'Reset sign-in' }));
    confirm = await findDialog('Reset the sign-in of “alice-laptop”?');
    await user.click(within(confirm).getByRole('button', { name: 'Reset sign-in' }));

    expect(await screen.findByText('Sign-in of “alice-laptop” reset')).toBeInTheDocument();
    expect(requestsTo('POST', /\/revoke-auth$/).map((r) => r.path)).toEqual([`/networks/${prod.id}/peers/${prodLaptop.id}/revoke-auth`]);
  });

  it('says a network without a jump peer has no portal', async () => {
    db.me = { ...db.me, id: 'alice', role: 'user' };
    const lab = seedNetwork({ name: 'lab' });
    seedPeer(lab.id, { name: 'alice-laptop', owner_id: 'alice' });
    renderApp('/devices');

    const labSection = await section('lab');
    expect(within(labSection).queryByRole('link', { name: 'Sign in' })).not.toBeInTheDocument();
    expect(within(labSection).getByText('This network has no jump peer yet, so no captive portal to sign in to.')).toBeInTheDocument();
  });

  it('opens a device’s details', async () => {
    const { prodLaptop } = seedUserDevices();
    const { user } = renderApp('/devices');

    const laptop = await within(await section('prod')).findByRole('listitem', { name: 'alice-laptop' });
    await user.click(within(laptop).getByRole('button', { name: 'Details' }));

    const dialog = await findDialog('alice-laptop');
    expect(within(dialog).getByText(prodLaptop.address)).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Download .conf' })).toBeInTheDocument();
  });

  it('starts empty, with a way to add a device', async () => {
    seedNetwork({ name: 'corp' });
    const { user } = renderApp('/devices');

    expect(await screen.findByText('No device yet')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'New device' }));
    expect(await findDialog('New peer')).toBeInTheDocument();
  });

  it('is in the menu, with the number of devices waiting for their owner', async () => {
    const { prodLaptop } = seedUserDevices();
    peerSessions({ [prodLaptop.id]: { has_active_agent: true } });
    const { user } = renderApp('/dashboard');

    const link = await screen.findByRole('link', { name: /^My devices/ });
    await waitFor(() => expect(link).toHaveAccessibleName('My devices, 1 device needs your attention'));
    await user.click(link);
    expect(await screen.findByRole('heading', { name: 'My devices' })).toBeInTheDocument();
  });

  it('is offered by the sign-in alert, which it then replaces', async () => {
    const { prodLaptop } = seedUserDevices();
    peerSessions({ [prodLaptop.id]: { has_active_agent: true } });
    const { user } = renderApp('/dashboard', { captivePortalAlert: true });

    const alert = await findDialog('Network sign-in required');
    await user.click(within(alert).getByRole('link', { name: 'My devices' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(await screen.findByRole('heading', { name: 'My devices' })).toBeInTheDocument();
    expect(sessionStorage.getItem('wirety.captivePortalAlert.dismissed')).toBe('1');
  });
});
