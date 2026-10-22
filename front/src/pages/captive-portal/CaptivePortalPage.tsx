import { useEffect, useState } from 'react';
import type { ComponentType, ReactNode, SVGProps } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { ArrowRightEndOnRectangleIcon, ClockIcon, LockClosedIcon, WifiIcon } from '@heroicons/react/20/solid';
import { CheckCircleIcon, LinkSlashIcon, ShieldExclamationIcon, XCircleIcon } from '@heroicons/react/24/outline';
import { useAuth } from '../../contexts/AuthContext';
import type { CaptivePortalEvent } from '../../types';
import { describeCaptivePortalEvent } from '../../utils/captivePortalEvents';
import { Alert, Button, Card, Spinner, cx } from '../../components/ui';
import AuthShell, { StatusIcon } from '../auth/AuthShell';
import type { StatusTone } from '../auth/AuthShell';

interface TokenPreview {
  peer_id: string;
  peer_name: string;
  peer_wg_ip: string;
  network_id: string;
  network_name: string;
  peer_endpoint: string;
  endpoint_ip: string;
  // Why the device's previous captive-portal access ended.
  last_sign_out?: CaptivePortalEvent;
}

/** Tells the user why they have to sign in again. */
function PreviousSignOut({ event }: { event?: CaptivePortalEvent }) {
  if (!event) return null;
  const { label, explanation } = describeCaptivePortalEvent(event);
  return (
    <div className="flex gap-2.5 rounded-control border border-line bg-surface-muted px-3 py-2.5 text-sm">
      <ClockIcon className="mt-0.5 size-4 shrink-0 text-ink-subtle" aria-hidden="true" />
      <div className="min-w-0 space-y-0.5">
        <p className="font-medium text-ink">Why sign in again?</p>
        <p className="text-ink-secondary">
          {label} on {new Date(event.created_at).toLocaleString()}. {explanation}
          {event.detail && <> ({event.detail})</>}
        </p>
      </div>
    </div>
  );
}

/**
 * The sign-in request the user is about to approve: which device, and from
 * which public IP. The public IP is the phishing tell — it must stand out.
 */
function RequestDetails({ preview, ipLabel, tone }: { preview: TokenPreview; ipLabel: string; tone: 'warning' | 'danger' }) {
  return (
    <dl aria-label="Sign-in request" className="overflow-hidden rounded-control border border-line text-sm">
      <div className="flex items-baseline justify-between gap-4 px-3.5 py-2.5">
        <dt className="text-ink-muted">Device</dt>
        <dd className="text-right text-[15px] font-semibold break-all text-ink">{preview.peer_name}</dd>
      </div>
      <div className="flex items-baseline justify-between gap-4 border-t border-line px-3.5 py-2.5">
        <dt className="text-ink-muted">Network</dt>
        <dd className="text-right font-medium break-all text-ink">{preview.network_name}</dd>
      </div>
      <div className="flex items-baseline justify-between gap-4 border-t border-line px-3.5 py-2.5">
        <dt className="text-ink-muted">VPN address</dt>
        <dd className="text-right font-mono text-[13px] text-ink">{preview.peer_wg_ip}</dd>
      </div>
      <div
        className={cx(
          'flex items-baseline justify-between gap-4 border-t px-3.5 py-3',
          tone === 'danger'
            ? 'border-red-200 bg-red-50 dark:border-red-500/20 dark:bg-red-500/10'
            : 'border-amber-200 bg-amber-50 dark:border-amber-500/20 dark:bg-amber-500/10',
        )}
      >
        <dt className={tone === 'danger' ? 'text-red-800 dark:text-red-200' : 'text-amber-900 dark:text-amber-200'}>
          {ipLabel}
        </dt>
        <dd
          className={cx(
            'text-right font-mono text-base font-semibold break-all',
            tone === 'danger' ? 'text-red-700 dark:text-red-300' : 'text-amber-800 dark:text-amber-300',
          )}
        >
          {preview.endpoint_ip || '(unknown)'}
        </dd>
      </div>
    </dl>
  );
}

/** A status screen: icon, title, message, then any content. */
function StatusCard({
  icon,
  tone,
  title,
  message,
  children,
}: {
  icon: ComponentType<SVGProps<SVGSVGElement>>;
  tone: StatusTone;
  title: string;
  message?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <Card className="space-y-5 p-6 sm:p-8">
      <div className="space-y-3 text-center">
        <StatusIcon icon={icon} tone={tone} />
        <h1 className="text-lg font-semibold tracking-tight text-ink">{title}</h1>
        {message && <div className="text-sm text-ink-muted">{message}</div>}
      </div>
      {children}
    </Card>
  );
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
  const { login, logout, user, isAuthenticated, isLoading } = useAuth();

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

  const goToDashboard = (
    <Button variant="secondary" className="w-full" onClick={() => navigate('/dashboard')}>
      Go to dashboard
    </Button>
  );

  if (!captiveToken) {
    return (
      <AuthShell footer="Powered by Wirety">
        <StatusCard
          icon={LinkSlashIcon}
          tone="warning"
          title="Invalid sign-in link"
          message="Missing authentication token. Please try accessing the network again."
        />
      </AuthShell>
    );
  }

  if (status === 'success') {
    return (
      <AuthShell footer="Powered by Wirety">
        <StatusCard
          icon={CheckCircleIcon}
          tone="success"
          title="Access granted"
          message="You now have access to this network."
        >
          {redirectUrl ? (
            <div role="status" className="space-y-1.5 text-center text-sm">
              <p className="flex items-center justify-center gap-2 text-ink-secondary">
                <Spinner className="text-ink-subtle" />
                Redirecting you to your destination…
              </p>
              <a
                href={redirectUrl}
                className="font-medium text-primary-600 underline-offset-2 hover:underline dark:text-primary-400"
              >
                Click here if not redirected automatically.
              </a>
            </div>
          ) : (
            <p className="text-center text-sm text-ink-secondary">You can now close this tab and access the network.</p>
          )}
        </StatusCard>
      </AuthShell>
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

    if (isPhishingSignal) {
      // Phishing-defense trip.  Make the security implication obvious and show
      // the same endpoint details the user saw on the preview screen, so they
      // can verify whether the request came from their own connection or
      // someone else's.
      return (
        <AuthShell wide footer="Powered by Wirety">
          <StatusCard icon={ShieldExclamationIcon} tone="danger" title="Sign-in blocked for safety">
            <Alert tone="danger" title="This sign-in link was opened in a different browser than the one that requested it.">
              <p>
                This is the response we give when a captive-portal link is shared, forwarded, or pasted into another
                browser — usually one of two situations:
              </p>
              <ul className="mt-1.5 list-disc space-y-1 pl-4">
                <li>
                  You legitimately copy-pasted the URL into another browser. In that case, just re-open the captive
                  portal from the device that needs network access.
                </li>
                <li>
                  Someone else is using your WireGuard config and tried to phish you into completing their sign-in.{' '}
                  <strong>Verify the public IP below</strong>, and if it isn’t yours, do <strong>not</strong> sign in
                  here — instead go to the dashboard and click <em>Reset auth</em> on the device.
                </li>
              </ul>
            </Alert>

            {preview ? (
              <>
                <RequestDetails preview={preview} ipLabel="Public IP that requested sign-in" tone="danger" />
                <PreviousSignOut event={preview.last_sign_out} />
              </>
            ) : (
              <p className="text-sm text-ink-muted italic">
                Token preview unavailable — could not display the requesting endpoint IP.
              </p>
            )}

            <p className="text-center text-xs break-words text-ink-muted">
              Raw error: <code className="font-mono">{errorMessage}</code>
            </p>

            {goToDashboard}
          </StatusCard>
        </AuthShell>
      );
    }

    // Generic auth failure (expired token, ownership mismatch, etc.).
    return (
      <AuthShell footer="Powered by Wirety">
        <StatusCard icon={XCircleIcon} tone="danger" title="Authentication failed">
          <Alert tone="danger" className="break-words">
            {errorMessage}
          </Alert>
          {isOwnershipError ? (
            <Button
              className="w-full"
              onClick={() => {
                logout();
                window.location.reload();
              }}
            >
              Sign in with a different account
            </Button>
          ) : (
            goToDashboard
          )}
        </StatusCard>
      </AuthShell>
    );
  }

  // Not signed in yet: the identity provider comes first.
  if (!isAuthenticated && !isLoading) {
    return (
      <AuthShell footer="Powered by Wirety">
        <Card className="space-y-6 p-6 sm:p-8">
          <div className="space-y-1">
            <h1 className="text-lg font-semibold tracking-tight text-ink">Network access required</h1>
            <p className="text-sm text-ink-muted">Sign in to access the network through this VPN.</p>
          </div>
          <ul className="space-y-4">
            <li className="flex gap-3">
              <LockClosedIcon className="mt-0.5 size-4 shrink-0 text-primary-600 dark:text-primary-400" aria-hidden="true" />
              <div className="text-sm">
                <p className="font-medium text-ink">Secure authentication</p>
                <p className="text-ink-muted">This network requires authentication through your organization’s identity provider.</p>
              </div>
            </li>
            <li className="flex gap-3">
              <WifiIcon className="mt-0.5 size-4 shrink-0 text-primary-600 dark:text-primary-400" aria-hidden="true" />
              <div className="text-sm">
                <p className="font-medium text-ink">Internet access</p>
                <p className="text-ink-muted">Once authenticated, you’ll have full internet access through the jump server.</p>
              </div>
            </li>
          </ul>
          <Button className="w-full" icon={<ArrowRightEndOnRectangleIcon className="size-4" />} onClick={handleLogin}>
            Sign in to continue
          </Button>
        </Card>
      </AuthShell>
    );
  }

  // Signed in (or checking the session), the preview is loading.
  if (!preview) {
    return (
      <AuthShell footer="Powered by Wirety">
        <div role="status" className="flex flex-col items-center gap-3 py-6 text-center">
          <Spinner className="size-6 text-primary-500" />
          <p className="text-sm font-medium text-ink">Verifying your sign-in request…</p>
        </div>
      </AuthShell>
    );
  }

  // Preview + Continue.  This is the user-visible half of phishing defense:
  // the user sees the device name and the public IP that requested the
  // sign-in, and must explicitly confirm "yes, this is me" before the token
  // is consumed.
  const authenticating = status === 'authenticating';
  return (
    <AuthShell wide footer="Powered by Wirety">
      <Card>
        <div className="space-y-5 p-6 sm:p-8">
          <div className="space-y-1">
            <h1 className="text-lg font-semibold tracking-tight text-ink">Is this you?</h1>
            <p className="text-sm text-ink-muted">
              A device asked to join the network with your account. Check that it is yours before granting it access.
            </p>
          </div>

          <Alert tone="warning" title="Verify before granting access">
            Confirm the device and public IP below. If you don’t recognise either — for example, the IP isn’t your
            current connection — <strong>do not continue</strong>. Your VPN config may be in use by someone else.
          </Alert>

          <RequestDetails preview={preview} ipLabel="Public IP" tone="warning" />

          <PreviousSignOut event={preview.last_sign_out} />
        </div>

        <div className="flex flex-col-reverse gap-2 rounded-b-panel border-t border-line bg-surface-muted/60 px-6 py-4 sm:flex-row sm:justify-end sm:px-8">
          <Button variant="secondary" onClick={() => navigate('/dashboard')} disabled={authenticating}>
            Cancel
          </Button>
          <Button onClick={handleConfirm} loading={authenticating}>
            Continue, this is me
          </Button>
        </div>
      </Card>
    </AuthShell>
  );
}
