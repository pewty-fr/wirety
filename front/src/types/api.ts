export interface Network {
  id: string;
  name: string;
  cidr: string;
  domain: string;
  gateway_peer_id?: string;
  created_at: string;
  updated_at: string;
}

export interface NetworkCreateRequest {
  name: string;
  cidr: string;
  domain: string;
}

export interface NetworkUpdateRequest {
  name?: string;
  cidr?: string;
  domain?: string;
}

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
  jump_nat_interface?: string;
  is_isolated: boolean;
  full_encapsulation: boolean;
  created_at: string;
  updated_at: string;
  network_id?: string; // Added for when loading all peers across networks
  network_name?: string; // Added to display network name on peer cards
}

export interface PeerCreateRequest {
  name: string;
  endpoint?: string;
  listen_port?: number;
  is_jump: boolean;
  jump_nat_interface?: string;
  is_isolated: boolean;
  full_encapsulation: boolean;
  additional_allowed_ips?: string[];
}

export interface PeerUpdateRequest {
  name?: string;
  endpoint?: string;
  listen_port?: number;
  is_isolated?: boolean;
  full_encapsulation?: boolean;
  additional_allowed_ips?: string[];
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

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  page_size: number;
}

export interface TokenResponse {
  token: string;
}

export interface ConfigResponse {
  config: string;
}

export interface AvailableCIDRsResponse {
  base_cidr: string;
  requested_max_peers: number;
  suggested_prefix: number;
  usable_hosts: number;
  cidrs: string[];
}
