import axios from 'axios';
import api from '../../api/client';
import type { Network, Peer, PolicyRule } from '../../types';

/** The name of the group, policy (and the route's network) set up with a network's first jump peer. */
export const DEFAULT_GROUP = 'default';

/** Groups need the server's database: without it their endpoints answer 503. */
export function groupsUnavailable(err: unknown): boolean {
  return axios.isAxiosError(err) && err.response?.status === 503;
}

/**
 * The rules of a network's default policy: its peers reach each other and the
 * internet. One input/output pair (plus internet egress) per address family
 * the network has, so dual-stack peers get IPv6 reachability too. Without the
 * internet rule the chain ends with DROP for anything outside the VPN CIDR;
 * admins can remove it later for a fully restrictive setup.
 */
export function defaultPolicyRules(net: Network): PolicyRule[] {
  const rules: PolicyRule[] = [];
  const rule = (direction: PolicyRule['direction'], target: string, description: string): PolicyRule => ({
    id: crypto.randomUUID(),
    direction,
    action: 'allow',
    target,
    target_type: 'cidr',
    description,
  });
  if (net.cidr) {
    rules.push(
      rule('input', net.cidr, 'allow network IPv4 CIDR resources to access a peer'),
      rule('output', net.cidr, 'allow a peer to access network IPv4 CIDR resources'),
      rule('output', '0.0.0.0/0', 'allow a peer to access the IPv4 internet'),
    );
  }
  if (net.cidr_v6) {
    rules.push(
      rule('input', net.cidr_v6, 'allow network IPv6 CIDR resources to access a peer'),
      rule('output', net.cidr_v6, 'allow a peer to access network IPv6 CIDR resources'),
      rule('output', '::/0', 'allow a peer to access the IPv6 internet'),
    );
  }
  return rules;
}

/**
 * Gives a network its default access when its first jump peer is created: a
 * "default" group holding every regular peer, with a "default" policy (see
 * defaultPolicyRules) and a single dual-stack route to the network's CIDRs
 * through the jump peer. Each step waits for the previous one; the first
 * failure is thrown.
 *
 * Returns false when the network already had a jump peer (nothing to do).
 */
export async function setUpDefaultAccess(net: Network, jumpPeer: Peer): Promise<boolean> {
  const peers = await api.getAllNetworkPeers(net.id);
  const jumpPeers = peers.filter((p) => p.is_jump);
  if (jumpPeers.length !== 1) return false;

  const group = await api.createGroup(net.id, {
    name: DEFAULT_GROUP,
    description: 'default group generated at network creation',
    priority: 100,
  });
  const policy = await api.createPolicy(net.id, {
    name: DEFAULT_GROUP,
    description: 'default policy for network access',
    rules: defaultPolicyRules(net),
  });
  await api.attachPolicyToGroup(net.id, group.id, policy.id);

  if (net.cidr || net.cidr_v6) {
    const route = await api.createRoute(net.id, {
      name: net.name,
      description: 'default route for network',
      destination_cidr: net.cidr || undefined,
      destination_cidr_v6: net.cidr_v6 || undefined,
      jump_peer_id: jumpPeer.id,
    });
    await api.attachRouteToGroup(net.id, group.id, route.id);
  }

  for (const peer of peers.filter((p) => !p.is_jump)) {
    await api.addPeerToGroup(net.id, group.id, peer.id);
  }
  return true;
}

/**
 * Adds a new peer to its network's "default" group, if there is one.
 * Returns the group's name when the peer was added.
 */
export async function joinDefaultGroup(networkId: string, peer: Peer): Promise<string | null> {
  let groups;
  try {
    groups = await api.getGroups(networkId);
  } catch (err) {
    if (groupsUnavailable(err)) return null;
    throw err;
  }
  const group = groups.find((g) => g.name === DEFAULT_GROUP);
  if (!group || peer.group_ids?.includes(group.id)) return null;
  await api.addPeerToGroup(networkId, group.id, peer.id);
  return group.name;
}
