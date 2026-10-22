import { useEffect, useRef, useState } from 'react';
import type { FormEvent } from 'react';
import { ArrowRightEndOnRectangleIcon } from '@heroicons/react/20/solid';
import { XCircleIcon } from '@heroicons/react/24/outline';
import { useAuth } from '../../contexts/AuthContext';
import { claimAutoReauth } from '../../auth/reauth';
import { Alert, Button, Card, PasswordInput, Spinner, TextInput } from '../../components/ui';
import AuthShell, { StatusIcon } from './AuthShell';

export default function LoginPage() {
  const { login, simpleLogin, authConfig, isLoading, oauthError, clearOauthError } = useAuth();
  // Username defaults to "admin" — the bootstrap account configured via AUTH_PASSWORD.
  // In simple-auth mode admins can also create regular users with email + password.
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('');
  const [fieldErrors, setFieldErrors] = useState<{ username?: string; password?: string }>({});
  const [loginError, setLoginError] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const passwordRef = useRef<HTMLInputElement>(null);

  const sessionExpired = new URLSearchParams(window.location.search).get('session_expired') === '1';

  // Check if we're in the middle of OAuth callback
  const hasAuthCode = new URLSearchParams(window.location.search).has('code');

  // When the session expired and SSO is the only way in, sign the user back in
  // automatically: with the identity provider's session still alive the round
  // trip needs no interaction, so the dashboard keeps running (and keeps
  // showing captive-portal alerts) instead of stopping here.
  const ssoOnly = !!authConfig?.enabled && !authConfig.simple_auth;
  const [autoReauth, setAutoReauth] = useState(false);
  useEffect(() => {
    if (sessionExpired && ssoOnly && !isLoading && !hasAuthCode && !oauthError && claimAutoReauth()) {
      setAutoReauth(true);
      login();
    }
  }, [sessionExpired, ssoOnly, isLoading, hasAuthCode, oauthError, login]);

  const handleSimpleLogin = async (e: FormEvent) => {
    e.preventDefault();
    if (isSubmitting) return;
    const errors = {
      username: username.trim() ? undefined : 'Enter your username or email.',
      password: password ? undefined : 'Enter your password.',
    };
    setFieldErrors(errors);
    if (errors.username || errors.password) return;
    setLoginError('');
    setIsSubmitting(true);
    const ok = await simpleLogin(username, password);
    if (!ok) {
      setLoginError('Invalid credentials. Please try again.');
      setPassword('');
      passwordRef.current?.focus();
    }
    setIsSubmitting(false);
  };

  // The OAuth callback failed: say why, then back to the sign-in screen.
  if (hasAuthCode && oauthError) {
    return (
      <AuthShell>
        <Card className="space-y-5 p-6 text-center sm:p-8">
          <StatusIcon icon={XCircleIcon} tone="danger" />
          <div className="space-y-1">
            <h1 className="text-lg font-semibold text-ink">Sign-in failed</h1>
            <p className="text-sm text-ink-muted">Your identity provider signed you in, but Wirety could not open a session.</p>
          </div>
          <Alert tone="danger" className="text-left break-words">
            {oauthError}
          </Alert>
          <Button
            className="w-full"
            onClick={() => {
              window.history.replaceState({}, document.title, window.location.pathname);
              clearOauthError();
            }}
          >
            Back to sign-in
          </Button>
        </Card>
      </AuthShell>
    );
  }

  // OAuth callback in progress, session check, or automatic re-authentication.
  if (isLoading || hasAuthCode || autoReauth) {
    return (
      <AuthShell>
        <div role="status" className="flex flex-col items-center gap-3 py-6 text-center">
          <Spinner className="size-6 text-primary-500" />
          <p className="text-sm font-medium text-ink">
            {hasAuthCode ? 'Completing sign-in…' : autoReauth ? 'Session expired — signing you back in…' : 'Loading…'}
          </p>
          <p className="text-xs text-ink-muted">Please wait</p>
        </div>
      </AuthShell>
    );
  }

  return (
    <AuthShell footer="WireGuard network management">
      <Card className="p-6 sm:p-8">
        <div className="mb-6 space-y-1">
          <h1 className="text-lg font-semibold tracking-tight text-ink">Sign in</h1>
          <p className="text-sm text-ink-muted">
            {authConfig?.simple_auth ? 'Use your Wirety account.' : 'Use your organization’s account.'}
          </p>
        </div>

        <div className="space-y-4">
          {sessionExpired && (
            <Alert tone="warning">Your session has expired. Please sign in again.</Alert>
          )}
          {oauthError && (
            <Alert tone="danger" title="Sign-in failed" className="break-words">
              {oauthError}
            </Alert>
          )}

          {authConfig?.simple_auth ? (
            <form onSubmit={handleSimpleLogin} noValidate className="space-y-4">
              {loginError && <Alert tone="danger">{loginError}</Alert>}
              <TextInput
                label="Username or email"
                required
                autoComplete="username"
                autoCapitalize="none"
                spellCheck={false}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="admin or your email"
                error={fieldErrors.username}
              />
              <PasswordInput
                ref={passwordRef}
                label="Password"
                required
                autoFocus
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Enter your password"
                error={fieldErrors.password}
              />
              <Button type="submit" className="w-full" loading={isSubmitting}>
                {isSubmitting ? 'Signing in…' : 'Sign in'}
              </Button>
            </form>
          ) : (
            <div className="space-y-3">
              <Button className="w-full" icon={<ArrowRightEndOnRectangleIcon className="size-4" />} onClick={login}>
                Sign in with SSO
              </Button>
              <p className="text-center text-xs text-ink-muted">You will be redirected to your organization’s login page.</p>
            </div>
          )}
        </div>
      </Card>
    </AuthShell>
  );
}
