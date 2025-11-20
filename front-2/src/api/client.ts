import axios from 'axios';
import type { AxiosInstance } from 'axios';
import type { Network, Peer, IPAMAllocation, SecurityIncident, User, PaginatedResponse } from '../types';

class ApiClient {
  private client: AxiosInstance;

  constructor(baseURL: string = 'http://localhost:8080/api/v1') {
    this.client = axios.create({
      baseURL,
      headers: {
        'Content-Type': 'application/json',
      },
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

  async createNetwork(data: { name: string; cidr: string; domain: string; dns?: string[] }): Promise<Network> {
    const response = await this.client.post('/networks', data);
    return response.data;
  }

  async updateNetwork(id: string, data: { name?: string; cidr?: string; domain?: string }): Promise<Network> {
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
        // Add network info to each peer
        peers.forEach((peer: Peer) => {
          peer.network_id = network.id;
          peer.network_name = network.name;
        });
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

  async createPeer(networkId: string, data: {
    name: string;
    endpoint?: string;
    listen_port?: number;
    is_jump: boolean;
    jump_nat_interface?: string;
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

  // Security
  async getSecurityIncidents(page: number = 1, pageSize: number = 20): Promise<{ data: SecurityIncident[], total: number, page: number, page_size: number }> {
    const params = new URLSearchParams({ page: page.toString(), page_size: pageSize.toString() });
    const response = await this.client.get(`/security/incidents?${params}`);
    return response.data;
  }

  async resolveIncident(incidentId: string): Promise<void> {
    await this.client.post(`/security/incidents/${incidentId}/resolve`);
  }

  // Users
  async getUsers(page: number = 1, pageSize: number = 20): Promise<PaginatedResponse<User>> {
    const params = new URLSearchParams({ page: page.toString(), page_size: pageSize.toString() });
    const response = await this.client.get(`/users?${params}`);
    return response.data;
  }

  async getUser(id: string): Promise<User> {
    const response = await this.client.get(`/users/${id}`);
    return response.data;
  }
}

export const api = new ApiClient();
export default api;
