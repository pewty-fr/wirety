import type { QueryClient } from '@tanstack/react-query';
import type { BadgeTone, ComboboxOption } from '../../components/ui';
import type { Peer, User } from '../../types';

export interface PeerStatus {
  label: string;
  tone: BadgeTone;
  /** A sign-in is in progress. */
  pending?: boolean;
}

/** A WireGuard handshake younger than this means the tunnel is up (WireGuard's own ~180 s). */
const CONNECTED_WINDOW_MS = 3 * 60 * 1000;

/**
 * The overall state of a peer, most important first:
 * quarantined > sign-in pending > offline > connected (jump peers, or
 * authenticated peers) > tunnel up but not authenticated (the jump peer's
 * firewall still drops its traffic, so it is not shown as connected).
 */
export function peerStatus(peer: Peer, now: number): PeerStatus {
  const session = peer.session_status;
  const portal = session?.captive_portal_state;
  const lastSeen = session?.current_session?.last_seen;
  const online = Boolean(session?.has_active_agent) || (lastSeen ? now - new Date(lastSeen).getTime() <= CONNECTED_WINDOW_MS : false);

  if (portal === 'quarantined') return { label: 'Quarantined', tone: 'danger' };
  if (portal === 'pending_auth') return { label: 'Pending sign-in', tone: 'warning', pending: true };
  if (!online) return { label: peer.use_agent ? 'Agent offline' : 'Offline', tone: 'neutral' };
  if (peer.is_jump || portal === 'authenticated') return { label: 'Connected', tone: 'success' };
  return { label: 'Not authenticated', tone: 'warning' };
}

/** Whether the peer's agent is connected, disconnected, or not used. */
export function agentStatus(peer: Peer): PeerStatus {
  if (!peer.use_agent) return { label: 'No agent', tone: 'neutral' };
  return peer.session_status?.has_active_agent
    ? { label: 'Agent connected', tone: 'success' }
    : { label: 'Agent disconnected', tone: 'danger' };
}

/** The captive-portal authentication of a (non-jump) peer. */
export function portalStatus(peer: Peer): PeerStatus {
  switch (peer.session_status?.captive_portal_state) {
    case 'authenticated':
      return { label: 'Authenticated', tone: 'success' };
    case 'pending_auth':
      return { label: 'Pending sign-in', tone: 'info', pending: true };
    case 'quarantined':
      return { label: 'Quarantined', tone: 'danger' };
    default:
      return { label: 'Not authenticated', tone: 'neutral' };
  }
}

/** Users as owner options: name, and e-mail as description. */
export function userOptions(users: User[]): ComboboxOption[] {
  return users.map((u) => ({ value: u.id, label: u.name || u.email, description: u.email }));
}

/** Refreshes every view of peers after a change. */
export function refreshPeers(queryClient: QueryClient) {
  for (const key of ['peers', 'networkPeers', 'peer', 'peerSession', 'peerReachability', 'acls']) {
    void queryClient.invalidateQueries({ queryKey: [key] });
  }
}
