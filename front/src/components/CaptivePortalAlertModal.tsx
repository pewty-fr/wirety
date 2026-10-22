import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { ArrowPathIcon, ArrowTopRightOnSquareIcon, BookmarkIcon, XMarkIcon } from '@heroicons/react/20/solid';
import { ExclamationTriangleIcon, ShieldExclamationIcon } from '@heroicons/react/24/outline';
import { useCaptivePortalCheck } from '../hooks/useCaptivePortalCheck';
import type { CaptivePortalNetworkInfo } from '../hooks/useCaptivePortalCheck';
import { useAttention } from '../hooks/useAttention';
import { useNetworks, usePeers } from '../hooks/useQueries';
import api from '../api/client';
import {
  Alert,
  Badge,
  Button,
  IconButton,
  Modal,
  ModalBody,
  ModalFooter,
  apiErrorMessage,
  buttonClass,
  cx,
  useConfirm,
  useToast,
} from './ui';

// sessionStorage key for "user dismissed the modal in this browser session".
// We don't use localStorage — the dismissal should reset on a new session so a
// user who reconnects to WG and reopens the dashboard tomorrow sees the prompt
// again.
const SESSION_DISMISS_KEY = 'wirety.captivePortalAlert.dismissed';

// localStorage key for the bookmark tip.  This is a one-time onboarding hint
// the user can dismiss permanently.
const BOOKMARK_TIP_KEY = 'wirety.captivePortalAlert.bookmarkTipDismissed';

const kbdClass = 'rounded border border-line-strong bg-surface px-1 py-px font-mono text-[11px] text-ink';

const plural = (n: number, one: string, many: string) => (n === 1 ? one : many);

/**
 * CaptivePortalAlertModal pops up when the current user has WireGuard peers
 * that need to authenticate to a captive portal — OR when there's a security
 * signal (suspicious sign-in attempts on a peer the user isn't currently using).
 *
 * The detection is server-side-state-based (see useCaptivePortalCheck) — we
 * cannot actively probe HTTP URLs from an HTTPS dashboard due to the browser's
 * mixed-content block, so we use the captive_portal_state field that the
 * backend already computes for every peer.
 *
 * Two visually-distinct branches inside the same modal:
 *
 *   1. Normal sign-in needed: peer is active but not authenticated.  CTA opens
 *      http://<jump_peer_wg_ip>/ in a new tab — that hits the agent's port-80
 *      captive portal which redirects through the bouncer to the OIDC sign-in
 *      page (with phishing-defense state cookie).
 *
 *   2. SUSPICIOUS ACTIVITY: peer is in pending_auth but the user has no recent
 *      session on it.  Someone else generated a captive portal token in this
 *      peer's name — most likely a stolen WG config in active use.  CTA is
 *      "Reset auth" which clears the whitelist + pending tokens + quarantine
 *      counter, kicking the impostor out.  We DO NOT show a Sign-in CTA here,
 *      because clicking it would just generate a fresh token but wouldn't
 *      address the underlying problem.
 */
export default function CaptivePortalAlertModal() {
  const networks = useCaptivePortalCheck();
  const confirm = useConfirm();
  const toast = useToast();
  const queryClient = useQueryClient();
  const [dismissed, setDismissed] = useState<boolean>(() => {
    return typeof window !== 'undefined' && window.sessionStorage.getItem(SESSION_DISMISS_KEY) === '1';
  });
  const [bookmarkTipDismissed, setBookmarkTipDismissed] = useState<boolean>(() => {
    return typeof window !== 'undefined' && window.localStorage.getItem(BOOKMARK_TIP_KEY) === '1';
  });
  // The network whose devices are being reset, and the last reset failure.
  const [resettingNetworkId, setResettingNetworkId] = useState<string | null>(null);
  const [resetError, setResetError] = useState<string | null>(null);

  // Reset session-dismissal whenever the user transitions from "everything
  // authenticated" to "something needs auth" — a fresh problem deserves a
  // fresh prompt even within the same session.
  const hasAlerts = networks.length > 0;
  const anySuspicious = networks.some(n => n.hasSuspiciousActivity);

  // Draw the user's attention when a device needs sign-in: a one-shot beep and,
  // while the dashboard tab is in the background, a flashing tab title. This
  // fires whenever there are alerts, independent of whether the modal itself is
  // dismissed — a backgrounded operator should still notice.
  useAttention(hasAlerts, anySuspicious ? '🔴 Suspicious activity' : '🔴 Sign-in required');

  // Null until the devices are loaded: the first load is not a transition, or
  // a dismissal would not survive reloading the page.
  const networksLoaded = useNetworks().isSuccess;
  const peersLoaded = usePeers(1, 1000, 10000).isSuccess;
  const loaded = networksLoaded && peersLoaded;
  const [prevHadAlerts, setPrevHadAlerts] = useState<boolean | null>(null);
  useEffect(() => {
    if (!loaded) return;
    if (hasAlerts && prevHadAlerts === false) {
      window.sessionStorage.removeItem(SESSION_DISMISS_KEY);
      setDismissed(false);
    }
    setPrevHadAlerts(hasAlerts);
  }, [loaded, hasAlerts, prevHadAlerts]);

  const dismissForSession = () => {
    window.sessionStorage.setItem(SESSION_DISMISS_KEY, '1');
    setDismissed(true);
    setResetError(null);
  };

  const dismissBookmarkTip = () => {
    window.localStorage.setItem(BOOKMARK_TIP_KEY, '1');
    setBookmarkTipDismissed(true);
  };

  const isOpen = hasAlerts && !dismissed;

  // For suspicious activity, reset every affected peer in the network (not
  // just the suspicious ones — the whole network's auth state should be wiped
  // to fully evict). One confirmation for the action, then the peers one by
  // one rather than in parallel, stopping at the first failure so a partial
  // failure leaves clear state.
  const handleResetAuth = async (n: CaptivePortalNetworkInfo) => {
    const peers = n.affectedPeers;
    const ok = await confirm({
      title: `Reset sign-in on “${n.network.name}”?`,
      message: (
        <div className="space-y-2">
          <p>
            {plural(peers.length, 'This device', `These ${peers.length} devices`)} will be removed from the captive-portal
            whitelist, outstanding sign-in tokens cancelled and any quarantine lifted:{' '}
            <span className="font-medium text-ink">{peers.map(p => `“${p.name}”`).join(', ')}</span>.
          </p>
          <p>
            Anyone using a copy of {plural(peers.length, 'its', 'their')} WireGuard config loses network access
            immediately. You will sign in again the next time you use {plural(peers.length, 'the device', 'them')}.
          </p>
        </div>
      ),
      confirmLabel: 'Reset auth',
      destructive: true,
    });
    if (!ok) return;

    setResetError(null);
    setResettingNetworkId(n.network.id);
    let done = 0;
    try {
      for (const p of peers) {
        await api.revokePeerAuthentication(n.network.id, p.id);
        done++;
      }
    } catch (err) {
      const failed = peers[done];
      setResetError(
        `${done > 0 ? `${done} of ${peers.length} devices reset; ` : ''}could not reset “${failed.name}”: ${apiErrorMessage(err)}`,
      );
      return;
    } finally {
      setResettingNetworkId(null);
      void queryClient.invalidateQueries({ queryKey: ['peers'] });
    }
    toast.success(
      peers.length === 1 ? `Sign-in reset on “${peers[0].name}”` : `Sign-in reset on ${peers.length} devices of “${n.network.name}”`,
    );
    // Closure: dismiss the modal — the affected peers' state will refresh on
    // the next data poll.  Leaving the modal open with stale data is confusing.
    dismissForSession();
  };

  const affectedCount = networks.reduce((sum, n) => sum + n.affectedPeers.length, 0);

  return (
    <Modal
      isOpen={isOpen}
      onClose={dismissForSession}
      title={anySuspicious ? 'Suspicious activity detected' : 'Network sign-in required'}
      size="md"
    >
      <ModalBody>
        <div className="flex items-start gap-3">
          <div
            className={cx(
              'flex size-9 shrink-0 items-center justify-center rounded-full',
              anySuspicious ? 'bg-red-50 dark:bg-red-500/10' : 'bg-amber-50 dark:bg-amber-500/10',
            )}
          >
            {anySuspicious ? (
              <ShieldExclamationIcon className="size-5 text-red-600 dark:text-red-400" aria-hidden="true" />
            ) : (
              <ExclamationTriangleIcon className="size-5 text-amber-600 dark:text-amber-400" aria-hidden="true" />
            )}
          </div>
          <p className="text-sm text-ink-secondary">
            {anySuspicious
              ? 'One of your devices is requesting network authentication, but you don’t appear to be using it right now. ' +
                'This could mean someone else is trying to use your WireGuard config. Review and act below.'
              : affectedCount === 1
                ? 'One of your devices needs to authenticate before it can access the network.'
                : `${affectedCount} of your devices need to authenticate before they can access the network.`}
          </p>
        </div>

        {resetError && <Alert tone="danger" title="Reset failed">{resetError}</Alert>}

        {/* Per-network breakdown */}
        <ul className="divide-y divide-line overflow-hidden rounded-control border border-line">
          {networks.map(n => (
            <li
              key={n.network.id}
              aria-label={n.network.name}
              className={cx(
                'flex items-center justify-between gap-3 px-4 py-3',
                n.hasSuspiciousActivity && 'bg-red-50/60 dark:bg-red-500/5',
              )}
            >
              <div className="min-w-0 space-y-1">
                <p className="truncate text-sm font-medium text-ink">{n.network.name}</p>
                <p className="text-xs text-ink-muted">{n.affectedPeers.map(p => p.name).join(', ')}</p>
                {(n.hasSuspiciousActivity || n.hasPending || n.hasQuarantined) && (
                  <div className="flex flex-wrap gap-1.5">
                    {n.hasSuspiciousActivity && (
                      <Badge tone="danger" dot>
                        Token issued from an unknown source
                      </Badge>
                    )}
                    {!n.hasSuspiciousActivity && n.hasPending && (
                      <Badge tone="info" dot>
                        Sign-in in progress
                      </Badge>
                    )}
                    {n.hasQuarantined && (
                      <Badge tone="danger" dot>
                        Quarantined — contact an administrator
                      </Badge>
                    )}
                  </div>
                )}
              </div>

              {/* Action — depends on whether this is suspicious or normal sign-in */}
              {n.hasSuspiciousActivity ? (
                <Button
                  variant="danger"
                  size="sm"
                  icon={<ArrowPathIcon className="size-4" />}
                  loading={resettingNetworkId === n.network.id}
                  disabled={resettingNetworkId !== null}
                  onClick={() => void handleResetAuth(n)}
                >
                  {resettingNetworkId === n.network.id ? 'Resetting…' : 'Reset auth'}
                </Button>
              ) : n.portalURL && !n.hasQuarantined ? (
                <a href={n.portalURL} target="_blank" rel="noreferrer noopener" className={buttonClass({ size: 'sm' })}>
                  <ArrowTopRightOnSquareIcon className="size-4" aria-hidden="true" />
                  Sign in
                </a>
              ) : (
                <span className="shrink-0 text-xs text-ink-muted">{n.hasQuarantined ? 'Blocked' : 'No portal URL'}</span>
              )}
            </li>
          ))}
        </ul>

        {/* Explainer for the chosen branch */}
        <p className="text-xs leading-relaxed text-ink-muted">
          {anySuspicious ? (
            <>
              <span className="font-medium text-ink-secondary">What happens when you reset:</span> the device will be
              removed from the captive-portal whitelist, any outstanding sign-in tokens will be cancelled, and the strike
              counter cleared. Anyone currently using a copy of this device’s config will lose network access
              immediately. You’ll need to sign in again the next time you actually use the device.
            </>
          ) : (
            <>
              <span className="font-medium text-ink-secondary">How this works:</span> <em>Sign in</em> opens the
              captive portal in a new tab. The new tab must reach the network through your active WireGuard tunnel, so
              make sure WireGuard is connected on the device you’re authenticating. You’ll be redirected to your
              organization’s sign-in page automatically.
            </>
          )}
        </p>

        {/* Bookmark tip — onboarding hint, dismissible permanently */}
        {!bookmarkTipDismissed && (
          <div className="flex items-start gap-2.5 rounded-control border border-line bg-surface-muted px-3 py-2.5">
            <BookmarkIcon className="mt-0.5 size-4 shrink-0 text-primary-600 dark:text-primary-400" aria-hidden="true" />
            <div className="min-w-0 flex-1 text-sm">
              <p className="font-medium text-ink">Bookmark your devices</p>
              <p className="mt-0.5 text-xs text-ink-muted">
                Add <code className="font-mono text-ink-secondary">{typeof window !== 'undefined' ? `${window.location.origin}/devices` : '/devices'}</code>{' '}
                to your bookmarks (<kbd className={kbdClass}>⌘D</kbd> / <kbd className={kbdClass}>Ctrl+D</kbd>) and open it
                whenever you reconnect WireGuard: you can sign your devices in from there, even once this window is
                closed, and see if anything looks suspicious.
              </p>
            </div>
            <IconButton label="Don’t show this tip again" onClick={dismissBookmarkTip} className="-mt-1 -mr-1.5">
              <XMarkIcon className="size-4" />
            </IconButton>
          </div>
        )}
      </ModalBody>
      <ModalFooter>
        <Link to="/devices" onClick={dismissForSession} className={buttonClass({ variant: 'ghost' })}>
          My devices
        </Link>
        <Button variant="secondary" onClick={dismissForSession}>
          Dismiss
        </Button>
      </ModalFooter>
    </Modal>
  );
}
