export interface Network {
  id: string;
  name: string;
  cidr: string;
  created_at: string;
  updated_at: string;
  peer_count?: number;
}

// Helper function to compute domain for a network
export const getNetworkDomain = (network: Network): string => {
  return `${network.name}.local`;
};

export interface Peer {
  id: string;
  name: string;
  public_key: string;
  address: string;
  endpoint: string;
  listen_port?: number;
  additional_allowed_ips?: string[];
  token?: string;
  is_jump: boolean;
  is_isolated: boolean;
  full_encapsulation: boolean;
  use_agent: boolean;
  owner_id?: string;
  created_at: string;
  updated_at: string;
  network_id?: string;
  network_name?: string;
  session?: PeerSession;
  session_status?: PeerSessionStatus;
}

export interface PeerSession {
  peer_id: string;
  reported_endpoint?: string;
  last_seen?: string;
  connected: boolean;
}

export interface PeerSessionStatus {
  peer_id: string;
  has_active_agent: boolean;
  current_session?: AgentSession;
  conflicting_sessions?: AgentSession[];
  recent_endpoint_changes?: EndpointChange[];
  suspicious_activity: boolean;
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

export interface EndpointChange {
  peer_id: string;
  old_endpoint: string;
  new_endpoint: string;
  changed_at: string;
  source: string;
}

export interface IPAMAllocation {
  network_id: string;
  network_name: string;
  network_cidr: string;
  ip: string;
  peer_id?: string;
  peer_name?: string;
  allocated: boolean;
}

export interface SecurityIncident {
  id: string;
  peer_id: string;
  peer_name: string;
  network_id: string;
  network_name: string;
  incident_type: 'shared_config' | 'session_conflict' | 'suspicious_activity';
  detected_at: string;
  public_key: string;
  endpoints: string[];
  details: string;
  resolved: boolean;
  resolved_at?: string;
  resolved_by?: string;
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
