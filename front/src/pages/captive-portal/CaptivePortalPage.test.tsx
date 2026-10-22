import { screen, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { server } from '../../test/setup';
import { renderApp } from '../../test/renderApp';
import type { CaptivePortalEvent } from '../../types';

const TOKEN = 'cpt_abc+/=';
const PAGE = `/captive-portal?token=${encodeURIComponent(TOKEN)}&redirect=${encodeURIComponent('http://wiki.corp.internal/start')}`;

const lastSignOut: CaptivePortalEvent = {
  event: 'endpoint_changed',
  detail: 'from 198.51.100.4',
  peer_ip: '10.42.0.2',
  created_at: '2026-10-01T09:00:00Z',
};

const preview = {
  peer_id: 'peer-1',
  peer_name: 'alice-laptop',
  peer_wg_ip: '10.42.0.2',
  network_id: 'net-1',
  network_name: 'corp',
  peer_endpoint: '203.0.113.7:51820',
  endpoint_ip: '203.0.113.7',
  last_sign_out: lastSignOut,
};

interface Call {
  method: string;
  url: URL;
  body?: unknown;
}

/**
 * The captive-portal endpoints (missing from the fake API). Each one answers
 * with its error when given, and records what it received.
 */
function captivePortalApi({ previewError, authError }: { previewError?: string; authError?: string } = {}) {
  const calls: Call[] = [];
  server.use(
    http.get('*/api/v1/captive-portal/preview', ({ request }) => {
      calls.push({ method: 'GET', url: new URL(request.url) });
      return previewError ? HttpResponse.json({ error: previewError }, { status: 401 }) : HttpResponse.json(preview);
    }),
    http.post('*/api/v1/captive-portal/authenticate', async ({ request }) => {
      calls.push({ method: 'POST', url: new URL(request.url), body: await request.json() });
      return authError
        ? HttpResponse.json({ error: authError }, { status: 401 })
        : HttpResponse.json({ peer_ip: '10.42.0.2', network_id: 'net-1', whitelisted: true });
    }),
  );
  return calls;
}

/** What the page passed to fetch() for the captive-portal endpoints. */
function captivePortalFetches() {
  return vi
    .mocked(globalThis.fetch)
    .mock.calls.filter(([input]) => String(input).includes('/captive-portal/'));
}

/** The sign-in request shown to the user, as { term: value }. */
function requestDetails() {
  const dl = screen.getByLabelText('Sign-in request');
  const terms = within(dl).getAllByRole('term').map((t) => t.textContent);
  const values = within(dl).getAllByRole('definition').map((d) => d.textContent);
  return Object.fromEntries(terms.map((t, i) => [t, values[i]]));
}

describe('Captive portal', () => {
  beforeEach(() => {
    // The page logs failed authentications.
    vi.spyOn(console, 'error').mockImplementation(() => {});
    vi.spyOn(globalThis, 'fetch');
  });
  afterEach(() => {
    window.history.replaceState({}, '', '/');
    vi.restoreAllMocks();
  });

  it('shows the device and public IP, and grants access only once the user confirms', async () => {
    const calls = captivePortalApi();
    const { user } = renderApp(PAGE);

    expect(await screen.findByRole('heading', { name: 'Is this you?' })).toBeInTheDocument();
    expect(requestDetails()).toEqual({
      Device: 'alice-laptop',
      Network: 'corp',
      'VPN address': '10.42.0.2',
      'Public IP': '203.0.113.7',
    });
    expect(screen.getByText('Why sign in again?')).toBeInTheDocument();
    expect(screen.getByText(/^Network changed on .*\(from 198\.51\.100\.4\)$/)).toBeInTheDocument();
    // Previewing does not consume the token.
    expect(calls).toHaveLength(1);
    expect(calls[0].method).toBe('GET');
    expect(calls[0].url.searchParams.get('token')).toBe(TOKEN);
    expect(captivePortalFetches()).toEqual([
      [`/api/v1/captive-portal/preview?token=${encodeURIComponent(TOKEN)}`, { credentials: 'include' }],
    ]);

    await user.click(screen.getByRole('button', { name: 'Continue, this is me' }));

    expect(await screen.findByRole('heading', { name: 'Access granted' })).toBeInTheDocument();
    expect(calls[1]).toMatchObject({ method: 'POST', body: { captive_token: TOKEN } });
    expect(captivePortalFetches()[1]).toEqual([
      '/api/v1/captive-portal/authenticate',
      {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ captive_token: TOKEN }),
      },
    ]);
    expect(screen.getByRole('link', { name: 'Click here if not redirected automatically.' })).toHaveAttribute(
      'href',
      'http://wiki.corp.internal/start',
    );
  });

  it('never follows a script redirect', async () => {
    captivePortalApi();
    const { user } = renderApp(`/captive-portal?token=${TOKEN}&redirect=${encodeURIComponent('javascript:alert(1)')}`);

    await user.click(await screen.findByRole('button', { name: 'Continue, this is me' }));

    expect(await screen.findByRole('heading', { name: 'Access granted' })).toBeInTheDocument();
    expect(screen.getByText('You can now close this tab and access the network.')).toBeInTheDocument();
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });

  it('cancels without consuming the token', async () => {
    const calls = captivePortalApi();
    const { user } = renderApp(PAGE);

    await user.click(await screen.findByRole('button', { name: 'Cancel' }));

    expect(await screen.findByRole('navigation', { name: 'Main' })).toBeInTheDocument();
    expect(calls.filter((c) => c.method === 'POST')).toHaveLength(0);
  });

  it('shows why the token cannot be used', async () => {
    const calls = captivePortalApi({ previewError: 'token expired or already used' });
    const { user } = renderApp(PAGE);

    expect(await screen.findByRole('heading', { name: 'Authentication failed' })).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('token expired or already used');
    expect(screen.queryByRole('button', { name: 'Continue, this is me' })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Go to dashboard' }));
    expect(await screen.findByRole('navigation', { name: 'Main' })).toBeInTheDocument();
    expect(calls.filter((c) => c.method === 'POST')).toHaveLength(0);
  });

  it('blocks a link opened in another browser and shows the public IP that requested it', async () => {
    captivePortalApi({ authError: 'session mismatch: please reopen the captive portal link in the browser that received it' });
    const { user } = renderApp(PAGE);

    await user.click(await screen.findByRole('button', { name: 'Continue, this is me' }));

    expect(await screen.findByRole('heading', { name: 'Sign-in blocked for safety' })).toBeInTheDocument();
    expect(
      screen.getByText('This sign-in link was opened in a different browser than the one that requested it.'),
    ).toBeInTheDocument();
    expect(requestDetails()).toMatchObject({ Device: 'alice-laptop', 'Public IP that requested sign-in': '203.0.113.7' });
    expect(screen.getByText('session mismatch: please reopen the captive portal link in the browser that received it')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Go to dashboard' })).toBeInTheDocument();
  });

  it('offers another account when the device belongs to someone else', async () => {
    captivePortalApi({ previewError: 'access denied: this peer belongs to another user' });
    let loggedOut = false;
    server.use(
      http.post('*/api/v1/auth/logout', () => {
        loggedOut = true;
        return HttpResponse.json({});
      }),
    );
    const { user } = renderApp(PAGE);

    expect(await screen.findByRole('alert')).toHaveTextContent('access denied: this peer belongs to another user');
    await user.click(screen.getByRole('button', { name: 'Sign in with a different account' }));

    await vi.waitFor(() => expect(loggedOut).toBe(true));
  });

  it('sends a signed-out user to the identity provider, then back to this page', async () => {
    const calls = captivePortalApi();
    server.use(
      http.get('*/api/v1/auth/config', () =>
        HttpResponse.json({
          enabled: true,
          issuer_url: 'https://idp.example.com',
          client_id: 'wirety',
          simple_auth: false,
          authorization_endpoint: 'https://idp.example.com/authorize',
          end_session_endpoint: '',
          scope: 'openid',
          authorization_extra_params: '',
        }),
      ),
      http.get('*/api/v1/users/me', () => HttpResponse.json({ error: 'unauthorized' }, { status: 401 })),
    );
    window.history.replaceState({}, '', PAGE);
    const { user } = renderApp(PAGE);

    expect(await screen.findByRole('heading', { name: 'Network access required' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Sign in to continue' }));

    expect(sessionStorage.getItem('captive_portal_return')).toBe(`http://localhost:5173${PAGE}`);
    expect(sessionStorage.getItem('oauth_state')).toMatch(/^[0-9a-f]{64}$/);
    expect(calls).toHaveLength(0);
  });

  it('explains a link without token', async () => {
    renderApp('/captive-portal');

    expect(await screen.findByRole('heading', { name: 'Invalid sign-in link' })).toBeInTheDocument();
    expect(screen.getByText('Missing authentication token. Please try accessing the network again.')).toBeInTheDocument();
  });
});
