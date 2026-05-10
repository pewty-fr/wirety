import { useMemo } from 'react';
import { useNetworks, usePeers } from './useQueries';
import { useAuth } from '../contexts/AuthContext';
import type { Peer, Network } from '../types';

/**
 * One entry per network the current user owns peers in that need attention.
 * Aggregating by network avoids showing N pop-ups when N devices on the same
 * network all need to sign in — the user only needs to authenticate once per
 * device, but the captive-portal URL is per-jump-peer.
 */
export interface CaptivePortalNetworkInfo {
  network: Network;
  /** WireGuard IP of the jump peer for this network (without prefix), or null if the network has no jump peer in our peer list. */
  jumpPeerWgIP: string | null;
  /** The current user's peers on this network that are not currently authenticated. */
  affectedPeers: Peer[];
  /** True if at least one affected peer is in pending_auth (token outstanding) — usually means a sign-in flow was started but not completed. */
  hasPending: boolean;
  /** True if at least one affected peer is quarantined — the captive portal won't help, admin reset is required. */
  hasQuarantined: boolean;
  /**
   * SECURITY SIGNAL.  True if at least one affected peer has captive-portal
   * state of "pending_auth" while the user is NOT currently active on that
   * peer (no recent agent heartbeat, no recent jump-peer-reported session).
   *
   * This means: somebody hit the captive portal HTTP server pretending to be
   * this peer — but we have no evidence that the legitimate owner is using
   * the device right now.  The most plausible explanation is that someone
   * else has a copy of the WireGuard config and is currently using it.
   *
   * The dashboard surfaces this as a different message ("Suspicious activity")
   * with a "Reset Auth" CTA instead of the normal Sign-in CTA, so the owner
   * can kick the impostor without inadvertently authenticating them.
   */
  hasSuspiciousActivity: boolean;
  /** Subset of affectedPeers where hasSuspiciousActivity-like criteria are true (pending_auth + not active). */
  suspiciousPeers: Peer[];
}

/**
 * Detects when the current user has WireGuard peers that need to authenticate
 * to a captive portal.
 *
 * The signal we use is purely server-side state we already have:
 *   peer.session_status.captive_portal_state ∈ {authenticated, pending_auth, quarantined, ""}
 *
 * We INTENTIONALLY do not try to do an active HTTP probe (e.g. fetch
 * http://example.com from the dashboard JS) — modern browsers block all mixed
 * content from HTTPS pages, so the probe would fail uniformly regardless of
 * whether a captive portal is actually intercepting it.  The state-based
 * approach is more accurate anyway: the backend already knows authoritatively
 * whether each peer is in the whitelist or not.
 *
 * Filtering rules:
 *   - Only owned peers (this user's own devices, not other people's).
 *   - Skip jump peers — they don't authenticate to themselves.
 *   - Skip authenticated peers (nothing to do).
 *   - Skip dormant peers — if a peer hasn't had recent connectivity, the user
 *     probably isn't actively trying to use it right now and a sign-in pop-up
 *     would be noise.  We treat as "active" any peer with has_active_agent OR
 *     a session last_seen within the last 5 minutes.
 */
export function useCaptivePortalCheck(): CaptivePortalNetworkInfo[] {
  const { user } = useAuth();
  const { data: networks = [] } = useNetworks();
  // Pull all peers — page size 1000 is fine for any realistic deployment and
  // saves us from having to paginate inside the hook.
  const { data: peersData } = usePeers(1, 1000);

  const peers = peersData?.peers || [];

  return useMemo(() => {
    if (!user) return [];

    const recentlyActiveThresholdMs = 5 * 60 * 1000;
    const now = Date.now();

    const myPeers = peers.filter(p => p.owner_id === user.id);

    const isActive = (p: Peer): boolean => {
      const lastSeen = p.session_status?.current_session?.last_seen;
      const lastSeenAge = lastSeen ? now - new Date(lastSeen).getTime() : Infinity;
      return !!p.session_status?.has_active_agent || lastSeenAge <= recentlyActiveThresholdMs;
    };

    // Two distinct categories deserve a pop-up:
    //   1. Active + needs auth      — the normal "please sign in" case
    //   2. Inactive + pending_auth  — security signal (token issued, but the
    //                                  legitimate owner isn't currently on
    //                                  the device → someone else is)
    // Active + no state (= never authed, never used the device)         → no popup
    // Inactive + no state                                                → no popup
    const peersNeedingAttention = myPeers.filter(p => {
      if (p.is_jump) return false;
      const state = p.session_status?.captive_portal_state;
      if (state === 'authenticated') return false;
      if (isActive(p)) return true;
      // Not active — only surface if there's an outstanding pending_auth token.
      // That token was generated by SOMEONE accessing the captive portal as
      // this peer, and it wasn't us (we're not active).  Suspicious.
      return state === 'pending_auth';
    });

    // Group affected peers by network so the pop-up renders one row per
    // network even if multiple devices on that network need sign-in.
    const byNetwork = new Map<string, Peer[]>();
    for (const p of peersNeedingAttention) {
      if (!p.network_id) continue;
      const list = byNetwork.get(p.network_id) ?? [];
      list.push(p);
      byNetwork.set(p.network_id, list);
    }

    const result: CaptivePortalNetworkInfo[] = [];
    for (const [networkId, affectedPeers] of byNetwork.entries()) {
      const network = networks.find(n => n.id === networkId);
      if (!network) continue;

      const jumpPeer = peers.find(p => p.network_id === networkId && p.is_jump);
      const jumpPeerWgIP = jumpPeer ? jumpPeer.address.replace(/\/.*$/, '') : null;

      const states = affectedPeers.map(p => p.session_status?.captive_portal_state);
      const suspiciousPeers = affectedPeers.filter(
        p => p.session_status?.captive_portal_state === 'pending_auth' && !isActive(p)
      );

      result.push({
        network,
        jumpPeerWgIP,
        affectedPeers,
        hasPending: states.includes('pending_auth'),
        hasQuarantined: states.includes('quarantined'),
        hasSuspiciousActivity: suspiciousPeers.length > 0,
        suspiciousPeers,
      });
    }

    return result;
  }, [user, peers, networks]);
}
