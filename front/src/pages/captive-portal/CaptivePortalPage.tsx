import { useEffect, useState } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { useAuth } from '../../contexts/AuthContext';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import {
  faWifi,
  faShieldAlt,
  faCheckCircle,
  faExclamationTriangle,
  faTriangleExclamation,
} from '@fortawesome/free-solid-svg-icons';

interface TokenPreview {
  peer_id: string;
  peer_name: string;
  peer_wg_ip: string;
  network_id: string;
  network_name: string;
  peer_endpoint: string;
  endpoint_ip: string;
}

/**
 * safeRedirect validates the post-auth `redirect` query param before it is used
 * as a navigation target or an <a href>.
 *
 * The destination is whatever internal VPN resource the user originally tried to
 * reach (an arbitrary private host/IP), so we cannot restrict it to a known host
 * allowlist. We MUST, however, reject any non-http(s) scheme: a `javascript:` or
 * `data:` value here would execute script in the user's browser (XSS) when fed
 * to window.location.replace() or rendered as a link. Relative values are
 * resolved against our own origin and are always safe.
 *
 * Returns the validated absolute URL, or null if the param is absent/unsafe.
 * (Residual: a post-auth redirect to an arbitrary http(s) host is still
 * possible — proportionate, since the user has already completed SSO and the
 * explicit "this is me" confirmation by this point.)
 */
function safeRedirect(raw: string | null): string | null {
  if (!raw) return null;
  let parsed: URL;
  try {
    parsed = new URL(raw, window.location.origin);
  } catch {
    return null;
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return null;
  return parsed.href;
}

/**
 * CaptivePortalPage — sign-in landing target for the captive-portal redirect.
 *
 * Phishing-defense flow (see server migration 026 + service.go):
 *
 *   1. Agent's HTTP server intercepts the unauthenticated peer's request
 *      and 302s to {server}/api/v1/captive-portal/start?token=…
 *   2. /start sets the cp_state cookie (browser-binding) and 302s to
 *      /captive-portal?token=…
 *   3. THIS PAGE loads.  We DO NOT auto-authenticate.  Instead we fetch
 *      /api/v1/captive-portal/preview to get peer + endpoint details and
 *      show them to the user, who must explicitly confirm "yes, this is me"
 *      before the auth POST runs.
 *   4. On confirm, /api/v1/captive-portal/authenticate is called.  The
 *      browser automatically attaches the cp_state cookie; the server
 *      verifies it matches the consume_state on the token row.  Mismatch
 *      → reject (the phisher's victim doesn't carry the cookie set by
 *      step 2 in their browser).
 */
export default function CaptivePortalPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { login, logout, user, isAuthenticated } = useAuth();

  // 'initial' = first load before we've fetched the preview
  // 'preview' = preview loaded, awaiting user click
  // 'authenticating' = auth POST in flight
  // 'success' / 'error' = terminal states
  const [status, setStatus] = useState<
    'initial' | 'preview' | 'authenticating' | 'success' | 'error'
  >('initial');
  const [errorMessage, setErrorMessage] = useState('');
  const [preview, setPreview] = useState<TokenPreview | null>(null);

  const captiveToken = searchParams.get('token');
  // Validate the redirect target — reject javascript:/data: and other dangerous
  // schemes before it is ever used as a navigation target or link href.
  const redirectUrl = safeRedirect(searchParams.get('redirect'));

  // Fetch the preview as soon as we have a session.  This validates the token,
  // checks ownership, and gives us the data to show the user — but does NOT
  // consume the token (that happens only when the user clicks Continue).
  useEffect(() => {
    if (!isAuthenticated || !user || !captiveToken) return;
    if (preview || status !== 'initial') return;

    void (async () => {
      try {
        const response = await fetch(
          `/api/v1/captive-portal/preview?token=${encodeURIComponent(captiveToken)}`,
          { credentials: 'include' }
        );
        if (!response.ok) {
          const error = await response.json().catch(() => ({}));
          throw new Error(error?.error || `preview failed (${response.status})`);
        }
        const data = (await response.json()) as TokenPreview;
        setPreview(data);
        setStatus('preview');
      } catch (err) {
        setStatus('error');
        setErrorMessage(err instanceof Error ? err.message : 'Failed to load token details');
      }
    })();
  }, [isAuthenticated, user, captiveToken, preview, status]);

  const handleConfirm = async () => {
    if (!captiveToken) return;
    setStatus('authenticating');
    try {
      const response = await fetch('/api/v1/captive-portal/authenticate', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ captive_token: captiveToken }),
      });
      if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        throw new Error(error?.error || 'Authentication failed');
      }
      setStatus('success');
      // DNS TTL for intercepted internal domains is 1 s, so by the time this
      // timer fires the browser will re-resolve to the real service IP.
      setTimeout(() => {
        if (redirectUrl) {
          window.location.replace(redirectUrl);
        }
      }, 1500);
    } catch (err) {
      console.error('Captive portal authentication error:', err);
      setStatus('error');
      setErrorMessage(err instanceof Error ? err.message : 'Authentication failed');
    }
  };

  const handleLogin = () => {
    sessionStorage.setItem('captive_portal_return', window.location.href);
    login();
  };

  if (!captiveToken) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-primary-500 to-accent-blue dark:from-dark dark:to-primary-700 flex items-center justify-center px-4">
        <div className="max-w-md w-full bg-white dark:bg-gray-800 rounded-lg shadow-2xl p-8">
          <div className="text-center">
            <FontAwesomeIcon icon={faExclamationTriangle} className="text-6xl text-yellow-500 mb-4" />
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-4">Invalid Request</h1>
            <p className="text-gray-600 dark:text-gray-400">
              Missing authentication token. Please try accessing the network again.
            </p>
          </div>
        </div>
      </div>
    );
  }

  if (status === 'success') {
    return (
      <div className="min-h-screen bg-gradient-to-br from-green-500 to-green-600 flex items-center justify-center px-4">
        <div className="max-w-md w-full bg-white dark:bg-gray-800 rounded-lg shadow-2xl p-8">
          <div className="text-center">
            <FontAwesomeIcon icon={faCheckCircle} className="text-6xl text-green-500 mb-4" />
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-4">Access Granted!</h1>
            <p className="text-gray-600 dark:text-gray-400 mb-4">
              You now have access to this network.
            </p>
            {redirectUrl ? (
              <>
                <p className="text-sm text-gray-500 dark:text-gray-500 mb-3">
                  Redirecting you to your destination…
                </p>
                <a
                  href={redirectUrl}
                  className="text-sm text-green-600 dark:text-green-400 underline"
                >
                  Click here if not redirected automatically.
                </a>
              </>
            ) : (
              <p className="text-sm text-gray-500 dark:text-gray-500">
                You can now close this tab and access the network.
              </p>
            )}
          </div>
        </div>
      </div>
    );
  }

  if (status === 'error') {
    // Classify the error so we can render an appropriately-loud warning when
    // the failure is a phishing-defense trip (browser binding mismatch).  Other
    // failures (expired token, ownership mismatch, network errors) get a
    // softer presentation.
    //
    // Strings here are matched against what the server returns from
    // service.AuthenticateCaptivePortal — keep in sync if you change those.
    const isPhishingSignal =
      errorMessage.toLowerCase().includes('session mismatch') ||
      errorMessage.toLowerCase().includes('not bound to a browser session');
    const isOwnershipError = errorMessage.includes('belongs to another user');

    return (
      <div className="min-h-screen bg-gradient-to-br from-red-500 to-red-600 flex items-center justify-center px-4 py-8">
        <div className="max-w-md w-full bg-white dark:bg-gray-800 rounded-lg shadow-2xl p-8">
          <div className="text-center mb-5">
            <FontAwesomeIcon icon={faExclamationTriangle} className="text-6xl text-red-500 mb-4" />
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-2">
              {isPhishingSignal ? 'Sign-in blocked for safety' : 'Authentication Failed'}
            </h1>
          </div>

          {isPhishingSignal ? (
            // Phishing-defense trip.  Make the security implication obvious
            // and show the same endpoint details the user saw on the preview
            // screen, so they can verify whether the request came from their
            // own connection or someone else's.
            <>
              <div className="bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-900 rounded-lg p-4 mb-5">
                <p className="text-sm text-red-800 dark:text-red-200 font-medium mb-2">
                  This sign-in link was opened in a different browser than the one that requested it.
                </p>
                <p className="text-xs text-red-700 dark:text-red-300 leading-relaxed">
                  This is the response we give when a captive-portal link is shared, forwarded, or
                  pasted into another browser — usually one of two situations:
                </p>
                <ul className="text-xs text-red-700 dark:text-red-300 list-disc list-inside mt-2 space-y-1">
                  <li>You legitimately copy-pasted the URL into another browser. In that case, just
                    re-open the captive portal from the device that needs network access.</li>
                  <li>Someone else is using your WireGuard config and tried to phish you into
                    completing their sign-in. <strong>Verify the public IP below</strong>, and if it
                    isn't yours, do <strong>not</strong> sign in here — instead go to the dashboard
                    and click <em>Reset Auth</em> on the device.</li>
                </ul>
              </div>

              {preview ? (
                <dl className="bg-gray-50 dark:bg-gray-900 rounded-lg p-5 mb-5 text-sm space-y-3">
                  <div className="flex justify-between gap-4">
                    <dt className="text-gray-500 dark:text-gray-400">Device</dt>
                    <dd className="text-gray-900 dark:text-white font-medium text-right">
                      {preview.peer_name}
                    </dd>
                  </div>
                  <div className="flex justify-between gap-4">
                    <dt className="text-gray-500 dark:text-gray-400">Network</dt>
                    <dd className="text-gray-900 dark:text-white font-medium text-right">
                      {preview.network_name}
                    </dd>
                  </div>
                  <div className="flex justify-between gap-4">
                    <dt className="text-gray-500 dark:text-gray-400">VPN address</dt>
                    <dd className="text-gray-900 dark:text-white font-mono text-right">
                      {preview.peer_wg_ip}
                    </dd>
                  </div>
                  <div className="flex justify-between gap-4 pt-3 border-t border-gray-200 dark:border-gray-700">
                    <dt className="text-gray-500 dark:text-gray-400">Public IP that requested sign-in</dt>
                    <dd className="text-red-700 dark:text-red-400 font-mono font-semibold text-right break-all">
                      {preview.endpoint_ip || '(unknown)'}
                    </dd>
                  </div>
                </dl>
              ) : (
                <p className="text-xs text-gray-500 dark:text-gray-400 italic mb-5">
                  Token preview unavailable — could not display the requesting endpoint IP.
                </p>
              )}

              <p className="text-xs text-gray-500 dark:text-gray-400 mb-5 text-center">
                Raw error: <code className="font-mono">{errorMessage}</code>
              </p>

              <button
                onClick={() => navigate('/dashboard')}
                className="btn-brand w-full"
              >
                Go to Dashboard
              </button>
            </>
          ) : (
            // Generic auth failure (expired token, ownership mismatch, etc.).
            <div className="text-center">
              <p className="text-gray-600 dark:text-gray-400 mb-4">{errorMessage}</p>
              {isOwnershipError ? (
                <button
                  onClick={() => { logout(); window.location.reload(); }}
                  className="btn-brand"
                >
                  Sign in with a different account
                </button>
              ) : (
                <button
                  onClick={() => navigate('/dashboard')}
                  className="btn-brand"
                >
                  Go to Dashboard
                </button>
              )}
            </div>
          )}
        </div>
      </div>
    );
  }

  if (status === 'authenticating') {
    return (
      <div className="min-h-screen bg-gradient-to-br from-primary-500 to-accent-blue dark:from-dark dark:to-primary-700 flex items-center justify-center px-4">
        <div className="max-w-md w-full bg-white dark:bg-gray-800 rounded-lg shadow-2xl p-8">
          <div className="text-center">
            <div className="animate-spin rounded-full h-16 w-16 border-b-2 border-primary-600 mx-auto mb-4" />
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-4">Authenticating…</h1>
            <p className="text-gray-600 dark:text-gray-400">Please wait while we verify your access.</p>
          </div>
        </div>
      </div>
    );
  }

  // Initial state — before sign-in OR while preview is loading.
  return (
    <div className="min-h-screen bg-gradient-to-br from-primary-500 to-accent-blue dark:from-dark dark:to-primary-700 flex items-center justify-center px-4">
      <div className="max-w-md w-full bg-white dark:bg-gray-800 rounded-lg shadow-2xl p-8">
        <div className="text-center mb-6">
          <div className="inline-flex items-center justify-center w-20 h-20 bg-gradient-to-br from-primary-500 to-accent-blue rounded-full mb-4 shadow-lg">
            <FontAwesomeIcon icon={faWifi} className="text-4xl text-white" />
          </div>
          <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-2">Network Access Required</h1>
          <p className="text-gray-600 dark:text-gray-400">
            Authenticate to access the internet through this network
          </p>
        </div>

        {!isAuthenticated ? (
          <>
            <div className="bg-gray-50 dark:bg-gray-900 rounded-lg p-6 mb-6">
              <div className="flex items-start gap-3 mb-4">
                <FontAwesomeIcon icon={faShieldAlt} className="text-primary-600 dark:text-primary-400 mt-1" />
                <div>
                  <h3 className="font-semibold text-gray-900 dark:text-white mb-1">Secure Authentication</h3>
                  <p className="text-sm text-gray-600 dark:text-gray-400">
                    This network requires authentication through your organization's identity provider.
                  </p>
                </div>
              </div>
              <div className="flex items-start gap-3">
                <FontAwesomeIcon icon={faWifi} className="text-primary-600 dark:text-primary-400 mt-1" />
                <div>
                  <h3 className="font-semibold text-gray-900 dark:text-white mb-1">Internet Access</h3>
                  <p className="text-sm text-gray-600 dark:text-gray-400">
                    Once authenticated, you'll have full internet access through the jump server.
                  </p>
                </div>
              </div>
            </div>
            <button
              onClick={handleLogin}
              className="w-full btn-brand font-medium py-3 px-4 rounded-lg transition-all"
            >
              Sign In to Continue
            </button>
          </>
        ) : status === 'initial' || !preview ? (
          <div className="text-center py-4">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600 mx-auto mb-4" />
            <p className="text-gray-600 dark:text-gray-400">Verifying your sign-in request…</p>
          </div>
        ) : (
          // Preview + Continue.  This is the user-visible half of phishing
          // defense: the user sees the device name and the public IP that
          // requested the sign-in, and must explicitly confirm "yes, this
          // is me" before the token is consumed.
          <>
            <div className="bg-amber-50 dark:bg-amber-950/40 border border-amber-200 dark:border-amber-900 rounded-lg p-4 mb-5">
              <div className="flex items-start gap-3">
                <FontAwesomeIcon icon={faTriangleExclamation} className="text-amber-600 dark:text-amber-400 mt-0.5 flex-shrink-0" />
                <div className="text-sm text-gray-800 dark:text-gray-200">
                  <p className="font-semibold mb-1">Verify before granting access</p>
                  <p className="text-xs">
                    Confirm the device and public IP below. If you don't recognise either — for example, the IP isn't your current
                    connection — <strong>do not continue</strong>. Your VPN config may be in use by someone else.
                  </p>
                </div>
              </div>
            </div>

            <dl className="bg-gray-50 dark:bg-gray-900 rounded-lg p-5 mb-6 text-sm space-y-3">
              <div className="flex justify-between gap-4">
                <dt className="text-gray-500 dark:text-gray-400">Device</dt>
                <dd className="text-gray-900 dark:text-white font-medium text-right">
                  {preview.peer_name}
                </dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-gray-500 dark:text-gray-400">Network</dt>
                <dd className="text-gray-900 dark:text-white font-medium text-right">
                  {preview.network_name}
                </dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-gray-500 dark:text-gray-400">VPN address</dt>
                <dd className="text-gray-900 dark:text-white font-mono text-right">
                  {preview.peer_wg_ip}
                </dd>
              </div>
              <div className="flex justify-between gap-4 pt-3 border-t border-gray-200 dark:border-gray-700">
                <dt className="text-gray-500 dark:text-gray-400">Public IP</dt>
                <dd className="text-amber-700 dark:text-amber-400 font-mono font-semibold text-right break-all">
                  {preview.endpoint_ip || '(unknown)'}
                </dd>
              </div>
            </dl>

            <div className="flex gap-3">
              <button
                onClick={() => navigate('/dashboard')}
                className="flex-1 px-4 py-3 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 rounded-lg transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleConfirm}
                className="flex-1 btn-brand font-medium py-3 px-4 rounded-lg transition-all"
              >
                Continue, this is me
              </button>
            </div>
          </>
        )}

        <p className="text-xs text-center text-gray-500 dark:text-gray-500 mt-6">
          Powered by Wirety
        </p>
      </div>
    </div>
  );
}
