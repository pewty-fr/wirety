import type { Peer } from '../types';

/** A device seen within this delay is in use: its owner may be trying to sign in. */
const RECENTLY_ACTIVE_MS = 5 * 60 * 1000;

/** The device's agent is connected, or its tunnel was seen within 5 minutes. */
export function isDeviceActive(peer: Peer, now: number): boolean {
  const lastSeen = peer.session_status?.current_session?.last_seen;
  const lastSeenAge = lastSeen ? now - new Date(lastSeen).getTime() : Infinity;
  return Boolean(peer.session_status?.has_active_agent) || lastSeenAge <= RECENTLY_ACTIVE_MS;
}

/**
 * A sign-in token was requested for the device while its owner is not using
 * it: most likely someone else uses a copy of its configuration.
 */
export function isSuspicious(peer: Peer, now: number): boolean {
  return !peer.is_jump && peer.session_status?.captive_portal_state === 'pending_auth' && !isDeviceActive(peer, now);
}

/**
 * The device needs its owner's attention: in use but not signed in, or
 * suspicious. A device not in use and never signed in does not.
 */
export function needsAttention(peer: Peer, now: number): boolean {
  if (peer.is_jump) return false;
  const state = peer.session_status?.captive_portal_state;
  if (state === 'authenticated') return false;
  return isDeviceActive(peer, now) || state === 'pending_auth';
}

/**
 * The captive portal of a network: its jump peer's WireGuard address, over
 * plain HTTP (the portal has no HTTPS listener — it would have to intercept
 * the device's TLS). It only answers through the network's tunnel. Opening it
 * from an HTTPS page in a new tab is allowed (mixed-content blocking only
 * applies to resources loaded in the page).
 */
export function captivePortalURL(jump: Peer | undefined): string | null {
  const v4 = jump?.address?.replace(/\/.*$/, '');
  if (v4) return `http://${v4}/`;
  const v6 = jump?.address_v6?.replace(/\/.*$/, '');
  return v6 ? `http://[${v6}]/` : null;
}
