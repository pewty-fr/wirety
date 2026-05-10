import { useEffect, useMemo, useState } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import {
  faTriangleExclamation,
  faArrowUpRightFromSquare,
  faStar,
  faXmark,
  faShieldAlt,
  faShieldHalved,
  faRotateLeft,
} from '@fortawesome/free-solid-svg-icons';
import Modal from './Modal';
import { useCaptivePortalCheck } from '../hooks/useCaptivePortalCheck';
import api from '../api/client';

// sessionStorage key for "user dismissed the modal in this browser session".
// We don't use localStorage — the dismissal should reset on a new session so a
// user who reconnects to WG and reopens the dashboard tomorrow sees the prompt
// again.
const SESSION_DISMISS_KEY = 'wirety.captivePortalAlert.dismissed';

// localStorage key for the bookmark tip.  This is a one-time onboarding hint
// the user can dismiss permanently.
const BOOKMARK_TIP_KEY = 'wirety.captivePortalAlert.bookmarkTipDismissed';

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
  const [dismissed, setDismissed] = useState<boolean>(() => {
    return typeof window !== 'undefined' && window.sessionStorage.getItem(SESSION_DISMISS_KEY) === '1';
  });
  const [bookmarkTipDismissed, setBookmarkTipDismissed] = useState<boolean>(() => {
    return typeof window !== 'undefined' && window.localStorage.getItem(BOOKMARK_TIP_KEY) === '1';
  });
  const [resettingPeerId, setResettingPeerId] = useState<string | null>(null);

  // Reset session-dismissal whenever the user transitions from "everything
  // authenticated" to "something needs auth" — a fresh problem deserves a
  // fresh prompt even within the same session.
  const hasAlerts = networks.length > 0;
  const [prevHadAlerts, setPrevHadAlerts] = useState(hasAlerts);
  useEffect(() => {
    if (hasAlerts && !prevHadAlerts) {
      window.sessionStorage.removeItem(SESSION_DISMISS_KEY);
      setDismissed(false);
    }
    setPrevHadAlerts(hasAlerts);
  }, [hasAlerts, prevHadAlerts]);

  const dismissForSession = () => {
    window.sessionStorage.setItem(SESSION_DISMISS_KEY, '1');
    setDismissed(true);
  };

  const dismissBookmarkTip = () => {
    window.localStorage.setItem(BOOKMARK_TIP_KEY, '1');
    setBookmarkTipDismissed(true);
  };

  const isOpen = hasAlerts && !dismissed;
  const anySuspicious = networks.some(n => n.hasSuspiciousActivity);

  // Build the captive-portal URL for each network.  HTTP not HTTPS because
  // the captive portal HTTPS listener uses a self-signed cert (browser would
  // reject); HTTP works fine since window.open() to HTTP from an HTTPS page
  // is allowed (mixed-content blocking only applies to in-page resource loads).
  const networkLinks = useMemo(
    () =>
      networks.map(n => ({
        ...n,
        portalURL: n.jumpPeerWgIP ? `http://${n.jumpPeerWgIP}/` : null,
      })),
    [networks]
  );

  const handleResetAuth = async (networkId: string, peerId: string) => {
    if (
      !confirm(
        'Reset captive-portal authentication for this device?\n\n' +
          'This will clear the whitelist, cancel any pending sign-in tokens, ' +
          'and lift any quarantine. If someone else is using a copy of this ' +
          "device's WireGuard config, they'll be kicked off the network."
      )
    ) {
      return;
    }
    setResettingPeerId(peerId);
    try {
      await api.revokePeerAuthentication(networkId, peerId);
      // Closure: dismiss the modal — the affected peer's state will refresh
      // on the next data poll.  Leaving the modal open with stale data is
      // confusing.
      dismissForSession();
    } catch (err) {
      const e = err as { response?: { data?: { error?: string } }; message?: string };
      alert(e?.response?.data?.error || e?.message || 'Failed to reset auth state');
    } finally {
      setResettingPeerId(null);
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={dismissForSession}
      title={anySuspicious ? 'Suspicious activity detected' : 'Network sign-in required'}
      size="md"
    >
      <div className="space-y-5">
        <div className="flex items-start gap-3">
          <div className="flex-shrink-0 mt-0.5">
            <div
              className={`w-10 h-10 rounded-full flex items-center justify-center ${
                anySuspicious
                  ? 'bg-red-100 dark:bg-red-900/40'
                  : 'bg-amber-100 dark:bg-amber-900/40'
              }`}
            >
              <FontAwesomeIcon
                icon={anySuspicious ? faShieldHalved : faTriangleExclamation}
                className={`text-lg ${
                  anySuspicious
                    ? 'text-red-600 dark:text-red-400'
                    : 'text-amber-600 dark:text-amber-400'
                }`}
              />
            </div>
          </div>
          <div className="flex-1">
            {anySuspicious ? (
              <p className="text-sm text-gray-700 dark:text-gray-200">
                One of your devices is requesting network authentication, but you don't
                appear to be using it right now. This could mean someone else is
                trying to use your WireGuard config. Review and act below.
              </p>
            ) : (
              <p className="text-sm text-gray-700 dark:text-gray-200">
                {networks.length === 1
                  ? 'One of your devices needs to authenticate before it can access the network.'
                  : `${networks.flatMap(n => n.affectedPeers).length} of your devices need to authenticate before they can access the network.`}
              </p>
            )}
          </div>
        </div>

        {/* Per-network breakdown */}
        <div className="space-y-3">
          {networkLinks.map(n => (
            <div
              key={n.network.id}
              className={`border rounded-lg p-4 ${
                n.hasSuspiciousActivity
                  ? 'border-red-200 dark:border-red-900 bg-red-50 dark:bg-red-950/30'
                  : 'border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50'
              }`}
            >
              <div className="flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <FontAwesomeIcon
                      icon={n.hasSuspiciousActivity ? faShieldHalved : faShieldAlt}
                      className={`text-sm ${
                        n.hasSuspiciousActivity
                          ? 'text-red-600 dark:text-red-400'
                          : 'text-primary-600 dark:text-primary-400'
                      }`}
                    />
                    <span className="font-medium text-gray-900 dark:text-white truncate">
                      {n.network.name}
                    </span>
                  </div>
                  <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                    {n.affectedPeers.map(p => p.name).join(', ')}
                    {n.hasSuspiciousActivity && (
                      <span className="ml-2 text-red-600 dark:text-red-400 font-medium">
                        • token issued from an unknown source
                      </span>
                    )}
                    {!n.hasSuspiciousActivity && n.hasPending && (
                      <span className="ml-2 text-blue-600 dark:text-blue-400">• sign-in in progress</span>
                    )}
                    {n.hasQuarantined && (
                      <span className="ml-2 text-red-600 dark:text-red-400">• quarantined — contact admin</span>
                    )}
                  </div>
                </div>

                {/* Action — depends on whether this is suspicious or normal sign-in */}
                {n.hasSuspiciousActivity ? (
                  <button
                    onClick={() => {
                      // For suspicious activity, reset every affected peer in
                      // the network (not just the suspicious ones — the whole
                      // network's auth state should be wiped to fully evict).
                      // We sequence through them rather than parallelizing so
                      // a partial failure leaves clear state.
                      void (async () => {
                        for (const p of n.affectedPeers) {
                          if (resettingPeerId) break;
                          await handleResetAuth(n.network.id, p.id);
                        }
                      })();
                    }}
                    disabled={!!resettingPeerId}
                    className="flex-shrink-0 inline-flex items-center gap-2 px-3 py-2 bg-red-600 hover:bg-red-700 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors"
                  >
                    <FontAwesomeIcon icon={faRotateLeft} className="text-xs" />
                    {resettingPeerId ? 'Resetting…' : 'Reset auth'}
                  </button>
                ) : n.portalURL && !n.hasQuarantined ? (
                  <a
                    href={n.portalURL}
                    target="_blank"
                    rel="noreferrer noopener"
                    className="flex-shrink-0 inline-flex items-center gap-2 px-3 py-2 bg-primary-600 hover:bg-primary-700 text-white text-sm font-medium rounded-lg transition-colors"
                  >
                    <FontAwesomeIcon icon={faArrowUpRightFromSquare} className="text-xs" />
                    Sign in
                  </a>
                ) : (
                  <span className="flex-shrink-0 text-xs text-gray-500 dark:text-gray-400 italic">
                    {n.hasQuarantined ? 'blocked' : 'no portal URL'}
                  </span>
                )}
              </div>
            </div>
          ))}
        </div>

        {/* Explainer for the chosen branch */}
        {anySuspicious ? (
          <div className="text-xs text-gray-600 dark:text-gray-400 leading-relaxed border-t border-gray-200 dark:border-gray-700 pt-4">
            <strong className="text-gray-800 dark:text-gray-200">What happens when you reset:</strong>{' '}
            The device will be removed from the captive-portal whitelist, any
            outstanding sign-in tokens will be cancelled, and the strike counter
            cleared. Anyone currently using a copy of this device's config will
            lose network access immediately. You'll need to sign in again the
            next time you actually use the device.
          </div>
        ) : (
          <div className="text-xs text-gray-500 dark:text-gray-400 leading-relaxed border-t border-gray-200 dark:border-gray-700 pt-4">
            <strong className="text-gray-700 dark:text-gray-300">How this works:</strong>{' '}
            Clicking <em>Sign in</em> opens the captive portal in a new tab. The
            new tab must reach the network through your active WireGuard tunnel,
            so make sure WireGuard is connected on the device you're authenticating.
            You'll be redirected to your organization's sign-in page automatically.
          </div>
        )}

        {/* Bookmark tip — onboarding hint, dismissible permanently */}
        {!bookmarkTipDismissed && (
          <div className="bg-blue-50 dark:bg-blue-950/40 border border-blue-200 dark:border-blue-900 rounded-lg p-4">
            <div className="flex items-start gap-3">
              <FontAwesomeIcon icon={faStar} className="text-blue-600 dark:text-blue-400 mt-0.5" />
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-gray-900 dark:text-white">Bookmark this page</p>
                <p className="text-xs text-gray-600 dark:text-gray-300 mt-1">
                  Add{' '}
                  <code className="px-1 py-0.5 bg-white dark:bg-gray-800 rounded text-blue-700 dark:text-blue-300 font-mono">
                    {typeof window !== 'undefined' ? window.location.origin : ''}
                  </code>{' '}
                  to your bookmarks (<kbd className="px-1.5 py-0.5 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded text-xs font-mono">⌘D</kbd>{' '}
                  /{' '}
                  <kbd className="px-1.5 py-0.5 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded text-xs font-mono">Ctrl+D</kbd>) and come back here whenever you reconnect WireGuard — we'll let you know if any device needs to sign in or if anything looks suspicious.
                </p>
              </div>
              <button
                onClick={dismissBookmarkTip}
                className="flex-shrink-0 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 transition-colors"
                title="Don't show this tip again"
              >
                <FontAwesomeIcon icon={faXmark} />
              </button>
            </div>
          </div>
        )}

        <div className="flex justify-end gap-3 pt-2 border-t border-gray-200 dark:border-gray-700">
          <button
            onClick={dismissForSession}
            className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
          >
            Dismiss
          </button>
        </div>
      </div>
    </Modal>
  );
}
