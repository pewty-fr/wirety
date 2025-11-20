export interface Network {
  id: string;
  name: string;
  cidr: string;
  domain: string;
  created_at: string;
  updated_at: string;
  peer_count?: number;
}

export interface Peer {
  id: string;
  name: string;
  public_key: string;
  address: string;
  endpoint: string;
  listen_port?: number;
  additional_allowed_ips?: string[];
  is_jump: boolean;
  jump_nat_interface?: string;
  is_isolated: boolean;
  full_encapsulation: boolean;
  use_agent: boolean;
  created_at: string;
  updated_at: string;
  network_id?: string;
  network_name?: string;
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

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  page_size: number;
}
