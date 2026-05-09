export interface Network {
  id: string;
  name: string;
  cidr: string;
  cidr_v6?: string;      // IPv6 CIDR (optional, dual-stack)
  dns: string[];
  domain_suffix?: string;
  default_group_ids?: string[];
  created_at: string;
  updated_at: string;
  peer_count?: number;
}

// Helper function to compute domain for a network
export const getNetworkDomain = (network: Network): string => {
  return `${network.name}.${network.domain_suffix}`;
};

export interface Peer {
  id: string;
  name: string;
  public_key: string;
  address: string;
  address_v6?: string;   // IPv6 WireGuard address (optional, dual-stack)
  endpoint: string;
  listen_port?: number;
  token?: string;
  is_jump: boolean;
  use_agent: boolean;
  owner_id?: string;
  group_ids?: string[];
  created_at: string;
  updated_at: string;
  network_id?: string;
  network_name?: string;
  session_status?: PeerConnectivityStatus;
}

export interface PeerConnectivityStatus {
  peer_id: string;
  has_active_agent: boolean;
  current_session?: AgentSession;
  last_checked: string;
}

export interface AgentSession {
  peer_id: string;
  hostname: string;
  system_uptime: number;
  wireguard_uptime: number;
  reported_endpoint: string;
  peer_endpoints?: Record<string, string>;
  last_seen: string;
  first_seen: string;
  session_id: string;
}

export interface IPAMAllocation {
  network_id: string;
  network_name: string;
  network_cidr: string;
  family?: 'ipv4' | 'ipv6';   // present on dual-stack-aware servers; "ipv4" implied if absent
  ip: string;
  peer_id?: string;
  peer_name?: string;
  allocated: boolean;
}


export interface User {
  id: string;
  email: string;
  name: string;
  role: 'administrator' | 'user';
  authorized_networks: string[];
  created_at: string;
  updated_at: string;
  last_login_at: string;
}

export interface ACLRule {
  id: string;
  source_peer: string;
  target_peer: string;
  action: string;
  description: string;
}

export interface ACL {
  id: string;
  name: string;
  enabled: boolean;
  blocked_peers: Record<string, boolean>;
  rules: ACLRule[];
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  page_size: number;
}

// Groups, Policies, Routes, DNS types
export interface Group {
  id: string;
  network_id: string;
  network_name?: string; // Added for cross-network display
  name: string;
  description: string;
  priority: number; // 0-999, lower = higher priority (0 for quarantine, 100 default)
  peer_ids: string[];
  policy_ids: string[];
  route_ids: string[];
  created_at: string;
  updated_at: string;
}

export interface Policy {
  id: string;
  network_id: string;
  network_name?: string; // Added for cross-network display
  name: string;
  description: string;
  rules: PolicyRule[];
  created_at: string;
  updated_at: string;
}

export interface PolicyRule {
  id: string;
  direction: 'input' | 'output';
  action: 'allow' | 'deny';
  target: string;
  target_type: 'cidr' | 'peer' | 'group' | 'route';
  description: string;
}

export interface Route {
  id: string;
  network_id: string;
  network_name?: string; // Added for cross-network display
  name: string;
  description: string;
  destination_cidr: string;
  jump_peer_id: string;
  domain_suffix: string;
  created_at: string;
  updated_at: string;
}

// Peer reachability — computed server-side from ACL + policies + routes
export interface PeerAccess {
  peer_id: string;
  peer_name: string;
  address: string;
  is_jump: boolean;
  allowed: boolean;
  /** "acl_disabled" | "blocked" | "deny_rule" | "allow_rule" | "default_allow" */
  reason: string;
}

export interface RuleAccess {
  direction: 'input' | 'output';
  action: 'allow' | 'deny';
  target_type: 'cidr' | 'peer' | 'group';
  target: string;
  addresses: string[];
  policy_name: string;
  group_name: string;
  description?: string;
}

export interface RouteAccess {
  route_id: string;
  route_name: string;
  destination_cidr: string;
  jump_peer_id: string;
  jump_peer_name: string;
  group_name: string;
}

export interface PeerReachability {
  peer_id: string;
  peer_name: string;
  peer_address: string;
  is_jump: boolean;
  peer_access: PeerAccess[] | null;
  rules: RuleAccess[] | null;
  routes: RouteAccess[] | null;
}

export interface DNSMapping {
  id: string;
  route_id: string;
  name: string;
  ip_address: string;
  created_at: string;
  updated_at: string;
}

export interface APIToken {
  id: string;
  name: string;
  token?: string; // only present on creation
  created_at: string;
  expires_at?: string;
  last_used_at?: string;
}
