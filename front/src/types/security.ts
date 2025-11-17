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
