import { screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { server } from '../../test/setup';
import { db, requests, requestsTo } from '../../test/fakeApi';
import { renderApp } from '../../test/renderApp';
import type { AuthConfig } from '../../contexts/AuthContext';

const simpleAuth: AuthConfig = {
  enabled: false,
  issuer_url: '',
  client_id: '',
  simple_auth: true,
  authorization_endpoint: '',
  end_session_endpoint: '',
  scope: '',
  authorization_extra_params: '',
};

const sso: AuthConfig = {
  enabled: true,
  issuer_url: 'https://idp.example.com',
  client_id: 'wirety',
  simple_auth: false,
  authorization_endpoint: 'https://idp.example.com/authorize',
  end_session_endpoint: 'https://idp.example.com/logout',
  scope: 'openid profile email',
  authorization_extra_params: '',
};

/**
 * Signed out: /users/me answers 401 until /auth/login accepts the password
 * `secret` (or `signIn()` is called).
 */
function signedOut(config: AuthConfig = simpleAuth) {
  let signedIn = false;
  server.use(
    http.get('*/api/v1/auth/config', () => HttpResponse.json(config)),
    http.get('*/api/v1/users/me', () =>
      signedIn ? HttpResponse.json(db.me) : HttpResponse.json({ error: 'unauthorized' }, { status: 401 }),
    ),
    http.post('*/api/v1/auth/login', async ({ request }) => {
      const body = (await request.json()) as { username: string; password: string };
      requests.push({ method: 'POST', path: '/auth/login', body });
      if (body.password !== 'secret') return HttpResponse.json({ error: 'invalid credentials' }, { status: 401 });
      signedIn = true;
      return HttpResponse.json({ session_hash: 'session', expires_in: 3600 });
    }),
  );
  return { signIn: () => (signedIn = true) };
}

/** Puts the browser on `url` (the login page reads window.location directly). */
function visit(url: string) {
  window.history.replaceState({}, '', url);
}

describe('Login', () => {
  beforeEach(() => {
    // jsdom cannot navigate to the identity provider: it reports it on the console.
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });
  afterEach(() => {
    visit('/');
    vi.restoreAllMocks();
  });

  describe('with a password', () => {
    it('signs in with the username and password', async () => {
      signedOut();
      const { user } = renderApp('/dashboard');

      expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeInTheDocument();
      expect(screen.getByLabelText(/^Username or email/)).toHaveValue('admin');
      await user.clear(screen.getByLabelText(/^Username or email/));
      await user.type(screen.getByLabelText(/^Username or email/), 'alice@example.com');
      await user.type(screen.getByLabelText(/^Password/), 'secret');
      await user.click(screen.getByRole('button', { name: 'Sign in' }));

      expect(await screen.findByRole('navigation', { name: 'Main' })).toBeInTheDocument();
      expect(requestsTo('POST', /^\/auth\/login$/).map((r) => r.body)).toEqual([
        { username: 'alice@example.com', password: 'secret' },
      ]);
    });

    it('shows wrong credentials in the form and clears the password', async () => {
      signedOut();
      const { user } = renderApp('/dashboard');
      await user.type(await screen.findByLabelText(/^Password/), 'wrong');

      await user.click(screen.getByRole('button', { name: 'Sign in' }));

      expect(await screen.findByRole('alert')).toHaveTextContent('Invalid credentials. Please try again.');
      expect(screen.getByLabelText(/^Password/)).toHaveValue('');
      expect(screen.getByLabelText(/^Password/)).toHaveFocus();
      expect(screen.getByRole('heading', { name: 'Sign in' })).toBeInTheDocument();
    });

    it('checks the form before sending anything', async () => {
      signedOut();
      const { user } = renderApp('/dashboard');
      await user.clear(await screen.findByLabelText(/^Username or email/));

      await user.click(screen.getByRole('button', { name: 'Sign in' }));

      expect(screen.getByText('Enter your username or email.')).toBeInTheDocument();
      expect(screen.getByText('Enter your password.')).toBeInTheDocument();
      expect(screen.getByLabelText(/^Password/)).toHaveAttribute('aria-invalid', 'true');
      expect(requestsTo('POST', /^\/auth\/login$/)).toHaveLength(0);
    });

    it('shows and hides the password with a keyboard-reachable button', async () => {
      signedOut();
      const { user } = renderApp('/dashboard');
      const password = await screen.findByLabelText(/^Password/);
      await user.type(password, 'secret');
      expect(password).toHaveAttribute('type', 'password');

      await user.tab();
      const toggle = screen.getByRole('button', { name: 'Show password' });
      expect(toggle).toHaveFocus();
      await user.keyboard('{Enter}');

      expect(password).toHaveAttribute('type', 'text');
      expect(screen.getByRole('button', { name: 'Hide password' })).toHaveAttribute('aria-controls', password.id);
      await user.click(screen.getByRole('button', { name: 'Hide password' }));
      expect(password).toHaveAttribute('type', 'password');
    });

    it('says when the session has expired', async () => {
      signedOut();
      visit('/?session_expired=1');
      renderApp('/dashboard');

      expect(await screen.findByText('Your session has expired. Please sign in again.')).toBeInTheDocument();
      expect(screen.getByLabelText(/^Password/)).toBeInTheDocument();
    });
  });

  describe('with SSO', () => {
    it('sends the browser to the identity provider', async () => {
      signedOut(sso);
      const { user } = renderApp('/dashboard');

      await user.click(await screen.findByRole('button', { name: 'Sign in with SSO' }));

      // login() stored the anti-CSRF state it sends to the identity provider.
      expect(sessionStorage.getItem('oauth_state')).toMatch(/^[0-9a-f]{64}$/);
      expect(screen.queryByLabelText(/^Password/)).not.toBeInTheDocument();
    });

    it('signs the user back in automatically when the session expired', async () => {
      signedOut(sso);
      visit('/?session_expired=1');
      renderApp('/dashboard');

      expect(await screen.findByText('Session expired — signing you back in…')).toBeInTheDocument();
      expect(sessionStorage.getItem('oauth_state')).toMatch(/^[0-9a-f]{64}$/);
      expect(sessionStorage.getItem('auth_auto_reauth_at')).not.toBeNull();
    });

    it('shows the sign-in page instead of looping when an automatic attempt just ran', async () => {
      signedOut(sso);
      sessionStorage.setItem('auth_auto_reauth_at', String(Date.now()));
      visit('/?session_expired=1');
      renderApp('/dashboard');

      expect(await screen.findByRole('button', { name: 'Sign in with SSO' })).toBeInTheDocument();
      expect(screen.getByText('Your session has expired. Please sign in again.')).toBeInTheDocument();
      expect(sessionStorage.getItem('oauth_state')).toBeNull();
    });

    it('completes the callback and opens the dashboard', async () => {
      const { signIn } = signedOut(sso);
      const exchanges: unknown[] = [];
      server.use(
        http.post('*/api/v1/auth/token', async ({ request }) => {
          exchanges.push(await request.json());
          signIn();
          return HttpResponse.json({});
        }),
      );
      sessionStorage.setItem('oauth_state', 'state-1');
      visit('/?code=code-1&state=state-1');
      renderApp('/dashboard');

      expect(await screen.findByRole('navigation', { name: 'Main' })).toBeInTheDocument();
      expect(exchanges).toEqual([{ code: 'code-1', redirect_uri: 'http://localhost:5173/' }]);
      expect(window.location.search).toBe('');
    });

    it('shows why the callback failed, then goes back to the sign-in page', async () => {
      signedOut(sso);
      server.use(
        http.post('*/api/v1/auth/token', () => HttpResponse.json({ error: 'invalid_grant: code expired' }, { status: 400 })),
      );
      sessionStorage.setItem('oauth_state', 'state-1');
      visit('/?code=code-1&state=state-1');
      const { user } = renderApp('/dashboard');

      expect(await screen.findByRole('heading', { name: 'Sign-in failed' })).toBeInTheDocument();
      expect(screen.getByRole('alert')).toHaveTextContent('invalid_grant: code expired');

      await user.click(screen.getByRole('button', { name: 'Back to sign-in' }));

      expect(await screen.findByRole('button', { name: 'Sign in with SSO' })).toBeInTheDocument();
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      expect(window.location.search).toBe('');
    });

    it('rejects a callback it did not request, and says so', async () => {
      signedOut(sso);
      const exchanges: unknown[] = [];
      server.use(
        http.post('*/api/v1/auth/token', () => {
          exchanges.push('called');
          return HttpResponse.json({});
        }),
      );
      visit('/?code=code-1&state=forged');
      renderApp('/dashboard');

      expect(await screen.findByRole('alert')).toHaveTextContent(
        'Sign-in could not be verified (state mismatch). Please try logging in again.',
      );
      expect(screen.getByRole('button', { name: 'Sign in with SSO' })).toBeInTheDocument();
      await waitFor(() => expect(window.location.search).toBe(''));
      expect(exchanges).toHaveLength(0);
    });
  });
});
