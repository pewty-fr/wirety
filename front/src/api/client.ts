import axios from 'axios';
import type { AxiosInstance } from 'axios';
import type { Network, Peer, IPAMAllocation, SecurityIncident, User, PaginatedResponse, PeerSessionStatus, ACL } from '../types';

class ApiClient {
  private client: AxiosInstance;

  constructor(baseURL?: string) {
    const apiBaseUrl = '/api/v1';
    this.client = axios.create({
      baseURL: apiBaseUrl,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Add request interceptor to include auth token
    this.client.interceptors.request.use((config) => {
      const token = localStorage.getItem('access_token');
      if (token) {
        config.headers.Authorization = `Bearer ${token}`;
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

  async createNetwork(data: { name: string; cidr: string; dns?: string[] }): Promise<Network> {
    const response = await this.client.post('/networks', data);
    return response.data;
  }

  async updateNetwork(id: string, data: { name?: string; cidr?: string }): Promise<Network> {
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
          if (peer.use_agent) {
            try {
              peer.session_status = await this.getPeerSessionStatus(network.id, peer.id);
            } catch (error) {
              console.warn(`Failed to fetch session status for peer ${peer.id}:`, error);
            }
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
    if (peer.use_agent) {
      try {
        peer.session_status = await this.getPeerSessionStatus(networkId, peerId);
      } catch (error) {
        console.warn(`Failed to fetch session status for peer ${peerId}:`, error);
      }
    }
    return peer;
  }

  async createPeer(networkId: string, data: {
    name: string;
    endpoint?: string;
    listen_port?: number;
    is_jump: boolean;
    is_isolated?: boolean;
    full_encapsulation?: boolean;
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
    is_isolated?: boolean;
    full_encapsulation?: boolean;
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

  // ACL
  async getACL(networkId: string): Promise<ACL> {
    const response = await this.client.get(`/networks/${networkId}/acl`);
    return response.data;
  }
}

export const api = new ApiClient();
export default api;
