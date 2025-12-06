import axios from 'axios';
import type { AxiosInstance } from 'axios';
import type { Network, Peer, IPAMAllocation, SecurityIncident, SecurityConfig, SecurityConfigUpdateRequest, User, PaginatedResponse, PeerSessionStatus, ACL, Group, Policy, PolicyRule, Route, DNSMapping } from '../types';

class ApiClient {
  private client: AxiosInstance;

  constructor() {
    const apiBaseUrl = '/api/v1';
    this.client = axios.create({
      baseURL: apiBaseUrl,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Add request interceptor to include session hash
    this.client.interceptors.request.use((config) => {
      const sessionHash = localStorage.getItem('session_hash');
      if (sessionHash) {
        config.headers.Authorization = `Session ${sessionHash}`;
      }
      return config;
    });
  }

  // Networks
  async getNetworks(page: number = 1, pageSize: number = 20, filter?: string): Promise<PaginatedResponse<Network>> {
    const params = new URLSearchParams({ page: page.toString(), page_size: pageSize.toString() });
    if (filter) params.append('filter', filter);
    const response = await this.client.get(`/networks?${params}`);
    return response.data;
  }

  async getNetwork(id: string): Promise<Network> {
    const response = await this.client.get(`/networks/${id}`);
    return response.data;
  }

  async createNetwork(data: { name: string; cidr: string; dns?: string[]; domain_suffix?: string; default_group_ids?: string[] }): Promise<Network> {
    const response = await this.client.post('/networks', data);
    return response.data;
  }

  async updateNetwork(id: string, data: { name?: string; cidr?: string; dns?: string[]; domain_suffix?: string; default_group_ids?: string[] }): Promise<Network> {
    const response = await this.client.put(`/networks/${id}`, data);
    return response.data;
  }

  async deleteNetwork(id: string): Promise<void> {
    await this.client.delete(`/networks/${id}`);
  }

  // Peers - Get all peers from all networks
  async getAllPeers(page: number = 1, pageSize: number = 20): Promise<{ peers: Peer[], total: number }> {
    // Since there's no global /peers endpoint, we need to fetch all networks first
    // then get peers for each network
    const networks = await this.getNetworks(1, 100); // Get up to 100 networks
    const allPeers: Peer[] = [];
    
    for (const network of networks.data) {
      try {
        const params = new URLSearchParams({ page: '1', page_size: '100' });
        const response = await this.client.get(`/networks/${network.id}/peers?${params}`);
        const peers = response.data.data || [];
        // Add network info to each peer and fetch session status for agents
        for (const peer of peers) {
          peer.network_id = network.id;
          peer.network_name = network.name;
          // Fetch session status for peers using agents
          try {
            peer.session_status = await this.getPeerSessionStatus(network.id, peer.id);
          } catch (error) {
            console.warn(`Failed to fetch session status for peer ${peer.id}:`, error);
          }
        }
        allPeers.push(...peers);
      } catch (error) {
        console.warn(`Failed to fetch peers for network ${network.id}:`, error);
      }
    }
    
    // Simple client-side pagination
    const start = (page - 1) * pageSize;
    const end = start + pageSize;
    const paginatedPeers = allPeers.slice(start, end);
    
    return {
      peers: paginatedPeers,
      total: allPeers.length
    };
  }

  async getPeers(networkId: string, page: number = 1, pageSize: number = 20): Promise<PaginatedResponse<Peer>> {
    const params = new URLSearchParams({ page: page.toString(), page_size: pageSize.toString() });
    const response = await this.client.get(`/networks/${networkId}/peers?${params}`);
    return response.data;
  }

  async getAllNetworkPeers(networkId: string): Promise<Peer[]> {
    // Get all peers for a network (no pagination)
    const params = new URLSearchParams({ page: '1', page_size: '1000' });
    const response = await this.client.get(`/networks/${networkId}/peers?${params}`);
    return response.data.data || [];
  }

  async getPeerSessionStatus(networkId: string, peerId: string): Promise<PeerSessionStatus> {
    const response = await this.client.get(`/networks/${networkId}/peers/${peerId}/session`);
    return response.data;
  }

  async getPeer(networkId: string, peerId: string): Promise<Peer> {
    const response = await this.client.get(`/networks/${networkId}/peers/${peerId}`);
    const peer = response.data;
    // Fetch session status for peers using agents
    try {
      peer.session_status = await this.getPeerSessionStatus(networkId, peerId);
    } catch (error) {
      console.warn(`Failed to fetch session status for peer ${peerId}:`, error);
    }
    return peer;
  }

  async createPeer(networkId: string, data: {
    name: string;
    endpoint?: string;
    listen_port?: number;
    is_jump: boolean;
    use_agent: boolean;
    additional_allowed_ips?: string[];
  }): Promise<Peer> {
    const response = await this.client.post(`/networks/${networkId}/peers`, data);
    return response.data;
  }

  async updatePeer(networkId: string, peerId: string, data: {
    name?: string;
    endpoint?: string;
    listen_port?: number;
    additional_allowed_ips?: string[];
  }): Promise<Peer> {
    const response = await this.client.put(`/networks/${networkId}/peers/${peerId}`, data);
    return response.data;
  }

  async deletePeer(networkId: string, peerId: string): Promise<void> {
    await this.client.delete(`/networks/${networkId}/peers/${peerId}`);
  }

  async getPeerConfig(networkId: string, peerId: string): Promise<string> {
    const response = await this.client.get(`/networks/${networkId}/peers/${peerId}/config`);
    // API returns { config: string }
    return response.data.config;
  }

  // IPAM
  async getIPAMAllocations(page: number = 1, pageSize: number = 20, filter?: string): Promise<{ data: IPAMAllocation[], total: number, page: number, page_size: number }> {
    const params = new URLSearchParams({ page: page.toString(), page_size: pageSize.toString() });
    if (filter) params.append('filter', filter);
    const response = await this.client.get(`/ipam?${params}`);
    return response.data;
  }

  async getNetworkIPAM(networkId: string): Promise<IPAMAllocation[]> {
    const response = await this.client.get(`/ipam/networks/${networkId}`);
    return response.data;
  }

  async getSuggestedCIDRs(maxPeers: number, count: number = 10, baseCIDR: string = '10.0.0.0/8'): Promise<{
    base_cidr: string;
    requested_max_peers: number;
    suggested_prefix: number;
    usable_hosts: number;
    cidrs: string[];
  }> {
    const params = new URLSearchParams({
      max_peers: maxPeers.toString(),
      count: count.toString(),
      base_cidr: baseCIDR,
    });
    const response = await this.client.get(`/ipam/available-cidrs?${params}`);
    return response.data;
  }

  // Security
  async getSecurityIncidents(page: number = 1, pageSize: number = 20, resolved?: boolean): Promise<{ data: SecurityIncident[], total: number, page: number, page_size: number }> {
    const params = new URLSearchParams({ page: page.toString(), page_size: pageSize.toString() });
    if (resolved !== undefined) {
      params.append('resolved', resolved.toString());
    }
    const response = await this.client.get(`/security/incidents?${params}`);
    return response.data;
  }

  async resolveIncident(incidentId: string): Promise<void> {
    await this.client.post(`/security/incidents/${incidentId}/resolve`);
  }

  async getSecurityConfig(networkId: string): Promise<SecurityConfig> {
    const response = await this.client.get(`/networks/${networkId}/security/config`);
    return response.data;
  }

  async updateSecurityConfig(networkId: string, data: SecurityConfigUpdateRequest): Promise<SecurityConfig> {
    const response = await this.client.put(`/networks/${networkId}/security/config`, data);
    return response.data;
  }

  // Users
  async getCurrentUser(): Promise<User> {
    const response = await this.client.get('/users/me');
    return response.data;
  }

  async getUsers(page: number = 1, pageSize: number = 20): Promise<User[]> {
    const params = new URLSearchParams({ page: page.toString(), page_size: pageSize.toString() });
    const response = await this.client.get(`/users?${params}`);
    // Backend returns array directly, not paginated response
    return response.data;
  }

  async getUser(id: string): Promise<User> {
    const response = await this.client.get(`/users/${id}`);
    return response.data;
  }

  async updateUser(id: string, data: {
    name?: string;
    role?: string;
    authorized_networks?: string[];
  }): Promise<User> {
    const response = await this.client.put(`/users/${id}`, data);
    return response.data;
  }

  async getDefaultPermissions(): Promise<{
    default_role: 'administrator' | 'user';
    default_authorized_networks: string[];
  }> {
    const response = await this.client.get('/users/defaults');
    return response.data;
  }

  async updateDefaultPermissions(data: {
    default_role: 'administrator' | 'user';
    default_authorized_networks: string[];
  }): Promise<{
    default_role: 'administrator' | 'user';
    default_authorized_networks: string[];
  }> {
    const response = await this.client.put('/users/defaults', data);
    return response.data;
  }

  // ACL
  async getACL(networkId: string): Promise<ACL> {
    const response = await this.client.get(`/networks/${networkId}/acl`);
    return response.data;
  }

  // Groups
  async getGroups(networkId: string): Promise<Group[]> {
    const response = await this.client.get(`/networks/${networkId}/groups`);
    return response.data;
  }

  async getGroup(networkId: string, groupId: string): Promise<Group> {
    const response = await this.client.get(`/networks/${networkId}/groups/${groupId}`);
    return response.data;
  }

  async createGroup(networkId: string, data: { name: string; description?: string; priority?: number }): Promise<Group> {
    const response = await this.client.post(`/networks/${networkId}/groups`, data);
    return response.data;
  }

  async updateGroup(networkId: string, groupId: string, data: { name?: string; description?: string; priority?: number }): Promise<Group> {
    const response = await this.client.put(`/networks/${networkId}/groups/${groupId}`, data);
    return response.data;
  }

  async deleteGroup(networkId: string, groupId: string): Promise<void> {
    await this.client.delete(`/networks/${networkId}/groups/${groupId}`);
  }

  async addPeerToGroup(networkId: string, groupId: string, peerId: string): Promise<void> {
    await this.client.post(`/networks/${networkId}/groups/${groupId}/peers/${peerId}`);
  }

  async removePeerFromGroup(networkId: string, groupId: string, peerId: string): Promise<void> {
    await this.client.delete(`/networks/${networkId}/groups/${groupId}/peers/${peerId}`);
  }

  async getGroupPolicies(networkId: string, groupId: string): Promise<Policy[]> {
    const response = await this.client.get(`/networks/${networkId}/groups/${groupId}/policies`);
    return response.data;
  }

  async attachPolicyToGroup(networkId: string, groupId: string, policyId: string): Promise<void> {
    await this.client.post(`/networks/${networkId}/groups/${groupId}/policies/${policyId}`);
  }

  async detachPolicyFromGroup(networkId: string, groupId: string, policyId: string): Promise<void> {
    await this.client.delete(`/networks/${networkId}/groups/${groupId}/policies/${policyId}`);
  }

  async reorderGroupPolicies(networkId: string, groupId: string, policyIds: string[]): Promise<void> {
    await this.client.put(`/networks/${networkId}/groups/${groupId}/policies/order`, { policy_ids: policyIds });
  }

  async getGroupRoutes(networkId: string, groupId: string): Promise<Route[]> {
    const response = await this.client.get(`/networks/${networkId}/groups/${groupId}/routes`);
    return response.data;
  }

  async attachRouteToGroup(networkId: string, groupId: string, routeId: string): Promise<void> {
    await this.client.post(`/networks/${networkId}/groups/${groupId}/routes/${routeId}`);
  }

  async detachRouteFromGroup(networkId: string, groupId: string, routeId: string): Promise<void> {
    await this.client.delete(`/networks/${networkId}/groups/${groupId}/routes/${routeId}`);
  }

  // Policies
  async getPolicies(networkId: string): Promise<Policy[]> {
    const response = await this.client.get(`/networks/${networkId}/policies`);
    return response.data;
  }

  async getPolicy(networkId: string, policyId: string): Promise<Policy> {
    const response = await this.client.get(`/networks/${networkId}/policies/${policyId}`);
    return response.data;
  }

  async createPolicy(networkId: string, data: { name: string; description?: string; rules?: PolicyRule[] }): Promise<Policy> {
    const response = await this.client.post(`/networks/${networkId}/policies`, data);
    return response.data;
  }

  async updatePolicy(networkId: string, policyId: string, data: { name?: string; description?: string }): Promise<Policy> {
    const response = await this.client.put(`/networks/${networkId}/policies/${policyId}`, data);
    return response.data;
  }

  async deletePolicy(networkId: string, policyId: string): Promise<void> {
    await this.client.delete(`/networks/${networkId}/policies/${policyId}`);
  }

  async addRuleToPolicy(networkId: string, policyId: string, rule: Omit<PolicyRule, 'id'>): Promise<PolicyRule> {
    const response = await this.client.post(`/networks/${networkId}/policies/${policyId}/rules`, rule);
    return response.data;
  }

  async removeRuleFromPolicy(networkId: string, policyId: string, ruleId: string): Promise<void> {
    await this.client.delete(`/networks/${networkId}/policies/${policyId}/rules/${ruleId}`);
  }

  // Routes
  async getRoutes(networkId: string): Promise<Route[]> {
    const response = await this.client.get(`/networks/${networkId}/routes`);
    return response.data;
  }

  async getRoute(networkId: string, routeId: string): Promise<Route> {
    const response = await this.client.get(`/networks/${networkId}/routes/${routeId}`);
    return response.data;
  }

  async createRoute(networkId: string, data: {
    name: string;
    description?: string;
    destination_cidr: string;
    jump_peer_id: string;
    domain_suffix?: string;
  }): Promise<Route> {
    const response = await this.client.post(`/networks/${networkId}/routes`, data);
    return response.data;
  }

  async updateRoute(networkId: string, routeId: string, data: {
    name?: string;
    description?: string;
    destination_cidr?: string;
    jump_peer_id?: string;
    domain_suffix?: string;
  }): Promise<Route> {
    const response = await this.client.put(`/networks/${networkId}/routes/${routeId}`, data);
    return response.data;
  }

  async deleteRoute(networkId: string, routeId: string): Promise<void> {
    await this.client.delete(`/networks/${networkId}/routes/${routeId}`);
  }

  // DNS Mappings
  async getDNSMappings(networkId: string, routeId: string): Promise<DNSMapping[]> {
    const response = await this.client.get(`/networks/${networkId}/routes/${routeId}/dns`);
    return response.data;
  }

  async createDNSMapping(networkId: string, routeId: string, data: {
    name: string;
    ip_address: string;
  }): Promise<DNSMapping> {
    const response = await this.client.post(`/networks/${networkId}/routes/${routeId}/dns`, data);
    return response.data;
  }

  async updateDNSMapping(networkId: string, routeId: string, dnsId: string, data: {
    name?: string;
    ip_address?: string;
  }): Promise<DNSMapping> {
    const response = await this.client.put(`/networks/${networkId}/routes/${routeId}/dns/${dnsId}`, data);
    return response.data;
  }

  async deleteDNSMapping(networkId: string, routeId: string, dnsId: string): Promise<void> {
    await this.client.delete(`/networks/${networkId}/routes/${routeId}/dns/${dnsId}`);
  }

  async getNetworkDNSRecords(networkId: string): Promise<DNSMapping[]> {
    const response = await this.client.get(`/networks/${networkId}/dns`);
    return response.data;
  }
}

export const api = new ApiClient();
export default api;
