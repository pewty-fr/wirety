import { screen, waitFor, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { server } from '../test/setup';
import { db, requestsTo, seedNetwork, seedPeer } from '../test/fakeApi';
import { findDialog, peerSessions as sessions } from '../test/helpers';
import { renderApp } from '../test/renderApp';

const DISMISSED_KEY = 'wirety.captivePortalAlert.dismissed';
const BOOKMARK_TIP_KEY = 'wirety.captivePortalAlert.bookmarkTipDismissed';

/** corp, with a jump peer and two devices of the signed-in user. */
function seedCorp() {
  const net = seedNetwork({ name: 'corp' });
  seedPeer(net.id, { name: 'jump-paris', is_jump: true, address: '10.42.0.1' });
  const laptop = seedPeer(net.id, { name: 'alice-laptop', owner_id: db.me.id });
  const phone = seedPeer(net.id, { name: 'alice-phone', owner_id: db.me.id });
  return { net, laptop, phone };
}

/** Renders the app with the alert allowed to show (renderApp dismisses it). */
function renderWithAlert() {
  const result = renderApp('/groups');
  sessionStorage.removeItem(DISMISSED_KEY);
  return result;
}

describe('Captive-portal alert', () => {
  it('asks to sign in the devices in use, through the network’s portal', async () => {
    const { laptop, phone } = seedCorp();
    sessions({ [laptop.id]: { has_active_agent: true }, [phone.id]: { has_active_agent: true } });
    const { user } = renderWithAlert();

    const dialog = await findDialog('Network sign-in required');
    expect(within(dialog).getByText('2 of your devices need to authenticate before they can access the network.')).toBeInTheDocument();
    const corp = within(dialog).getByRole('listitem', { name: 'corp' });
    expect(within(corp).getByText('alice-laptop, alice-phone')).toBeInTheDocument();
    const signIn = within(corp).getByRole('link', { name: 'Sign in' });
    expect(signIn).toHaveAttribute('href', 'http://10.42.0.1/');
    expect(signIn).toHaveAttribute('target', '_blank');
    expect(signIn).toHaveAttribute('rel', expect.stringContaining('noopener'));
    expect(within(dialog).queryByRole('button', { name: 'Reset auth' })).not.toBeInTheDocument();

    await user.click(within(dialog).getByRole('button', { name: 'Dismiss' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(sessionStorage.getItem(DISMISSED_KEY)).toBe('1');
  });

  it('stays dismissed when the page is reloaded', async () => {
    const { laptop } = seedCorp();
    sessions({ [laptop.id]: { has_active_agent: true } });
    renderApp('/groups'); // dismissed earlier in this browser session

    // The devices are loaded (the menu counts the one waiting)…
    expect(await screen.findByRole('link', { name: 'My devices, 1 device needs your attention' })).toBeInTheDocument();
    // …and loading them is not a new problem to prompt about.
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(sessionStorage.getItem(DISMISSED_KEY)).toBe('1');
  });

  it('does not offer the portal to a quarantined device', async () => {
    const { laptop } = seedCorp();
    sessions({ [laptop.id]: { has_active_agent: true, captive_portal_state: 'quarantined' } });
    renderWithAlert();

    const dialog = await findDialog('Network sign-in required');
    expect(within(dialog).getByText('One of your devices needs to authenticate before it can access the network.')).toBeInTheDocument();
    expect(within(dialog).getByText('Quarantined — contact an administrator')).toBeInTheDocument();
    expect(within(dialog).queryByRole('link', { name: 'Sign in' })).not.toBeInTheDocument();
  });

  it('resets the sign-in of every device of the network after one confirmation', async () => {
    const { net, laptop, phone } = seedCorp();
    sessions({ [laptop.id]: { captive_portal_state: 'pending_auth' }, [phone.id]: { captive_portal_state: 'pending_auth' } });
    const { user } = renderWithAlert();

    const dialog = await findDialog('Suspicious activity detected');
    expect(within(dialog).getByText('Token issued from an unknown source')).toBeInTheDocument();
    expect(within(dialog).queryByRole('link', { name: 'Sign in' })).not.toBeInTheDocument();

    // Cancel sends nothing.
    await user.click(within(dialog).getByRole('button', { name: 'Reset auth' }));
    let confirm = await findDialog('Reset sign-in on “corp”?');
    expect(within(confirm).getByText('“alice-laptop”, “alice-phone”')).toBeInTheDocument();
    await user.click(within(confirm).getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Reset sign-in on “corp”?' })).not.toBeInTheDocument());
    expect(requestsTo('POST', /\/revoke-auth$/)).toHaveLength(0);
    expect(screen.getByRole('dialog', { name: 'Suspicious activity detected' })).toBeInTheDocument();

    await user.click(within(dialog).getByRole('button', { name: 'Reset auth' }));
    confirm = await findDialog('Reset sign-in on “corp”?');
    await user.click(within(confirm).getByRole('button', { name: 'Reset auth' }));

    expect(await screen.findByText('Sign-in reset on 2 devices of “corp”')).toBeInTheDocument();
    expect(requestsTo('POST', /\/revoke-auth$/).map((r) => r.path)).toEqual([
      `/networks/${net.id}/peers/${laptop.id}/revoke-auth`,
      `/networks/${net.id}/peers/${phone.id}/revoke-auth`,
    ]);
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(sessionStorage.getItem(DISMISSED_KEY)).toBe('1');
  });

  it('stops at the first failure and says which device was not reset', async () => {
    const { laptop, phone } = seedCorp();
    sessions({ [laptop.id]: { captive_portal_state: 'pending_auth' }, [phone.id]: { captive_portal_state: 'pending_auth' } });
    server.use(
      http.post(`*/api/v1/networks/:id/peers/${phone.id}/revoke-auth`, () =>
        HttpResponse.json({ error: 'database unavailable' }, { status: 500 }),
      ),
    );
    const { user } = renderWithAlert();

    const dialog = await findDialog('Suspicious activity detected');
    await user.click(within(dialog).getByRole('button', { name: 'Reset auth' }));
    const confirm = await findDialog('Reset sign-in on “corp”?');
    await user.click(within(confirm).getByRole('button', { name: 'Reset auth' }));

    expect(await within(dialog).findByRole('alert')).toHaveTextContent(
      '1 of 2 devices reset; could not reset “alice-phone”: database unavailable',
    );
    expect(requestsTo('POST', /\/revoke-auth$/)).toHaveLength(1);
    expect(screen.getByRole('dialog', { name: 'Suspicious activity detected' })).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Reset auth' })).toBeEnabled();
    expect(sessionStorage.getItem(DISMISSED_KEY)).toBeNull();
  });

  it('hides the bookmark tip for good', async () => {
    const { laptop } = seedCorp();
    sessions({ [laptop.id]: { has_active_agent: true } });
    const { user } = renderWithAlert();

    const dialog = await findDialog('Network sign-in required');
    expect(within(dialog).getByText('Bookmark your devices')).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Don’t show this tip again' }));

    expect(within(dialog).queryByText('Bookmark your devices')).not.toBeInTheDocument();
    expect(localStorage.getItem(BOOKMARK_TIP_KEY)).toBe('1');
  });
});
