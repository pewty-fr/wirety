import axios, { AxiosInstance } from 'axios';
import {
  Network,
  NetworkCreateRequest,
  NetworkUpdateRequest,
  Peer,
  PeerCreateRequest,
  PeerUpdateRequest,
  IPAMAllocation,
  PaginatedResponse,
  TokenResponse,
  ConfigResponse,
  AvailableCIDRsResponse,
} from '../types/api';
import { SecurityIncident, PeerSessionStatus } from '../types/security';
import { User, AuthConfig, UserUpdateRequest, DefaultNetworkPermissions } from '../types/auth';

class ApiClient {
  private client: AxiosInstance;
  private accessToken: string | null = null;

  constructor(baseURL: string = 'http://localhost:8080/api/v1') {
    this.client = axios.create({
      baseURL,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Add request interceptor to include auth token
    this.client.interceptors.request.use((config) => {
      if (this.accessToken) {
        config.headers.Authorization = `Bearer ${this.accessToken}`;
      }
      return config;
    });
  }

  setBaseURL(url: string) {
    this.client.defaults.baseURL = url;
  }

  setAccessToken(token: string | null) {
    this.accessToken = token;
  }

  getAccessToken(): string | null {
    return this.accessToken;
  }

  // Network endpoints
  async getNetworks(
    page: number = 1,
    pageSize: number = 20,
    filter?: string
  ): Promise<PaginatedResponse<Network>> {
    const params: any = { page, page_size: pageSize };
    if (filter) params.filter = filter;
    const { data } = await this.client.get('/networks', { params });
    return data;
  }

  async getNetwork(id: string): Promise<Network> {
    const { data } = await this.client.get(`/networks/${id}`);
    return data;
  }

  async createNetwork(network: NetworkCreateRequest): Promise<Network> {
    const { data } = await this.client.post('/networks', network);
    return data;
  }

  async updateNetwork(id: string, network: NetworkUpdateRequest): Promise<Network> {
    const { data } = await this.client.put(`/networks/${id}`, network);
    return data;
  }

  async deleteNetwork(id: string): Promise<void> {
    await this.client.delete(`/networks/${id}`);
  }

  // Peer endpoints
  async getPeers(
    networkId: string,
    page: number = 1,
    pageSize: number = 20,
    filter?: string
  ): Promise<PaginatedResponse<Peer>> {
    const params: any = { page, page_size: pageSize };
    if (filter) params.filter = filter;
    const { data } = await this.client.get(`/networks/${networkId}/peers`, { params });
    return data;
  }

  async getPeer(networkId: string, peerId: string): Promise<Peer> {
    const { data } = await this.client.get(`/networks/${networkId}/peers/${peerId}`);
    return data;
  }

  async createPeer(networkId: string, peer: PeerCreateRequest): Promise<Peer> {
    const { data } = await this.client.post(`/networks/${networkId}/peers`, peer);
    return data;
  }

  async updatePeer(networkId: string, peerId: string, peer: PeerUpdateRequest): Promise<Peer> {
    const { data } = await this.client.put(`/networks/${networkId}/peers/${peerId}`, peer);
    return data;
  }

  async deletePeer(networkId: string, peerId: string): Promise<void> {
    await this.client.delete(`/networks/${networkId}/peers/${peerId}`);
  }

  async getPeerToken(networkId: string, peerId: string): Promise<TokenResponse> {
    const { data } = await this.client.get(`/networks/${networkId}/peers/${peerId}`);
    return data;
  }

  async getPeerConfig(networkId: string, peerId: string): Promise<ConfigResponse> {
    const { data } = await this.client.get(`/networks/${networkId}/peers/${peerId}/config`);
    return data;
  }

  // IPAM endpoints
  async getIPAM(
    page: number = 1,
    pageSize: number = 20,
    filter?: string
  ): Promise<PaginatedResponse<IPAMAllocation>> {
    const params: any = { page, page_size: pageSize };
    if (filter) params.filter = filter;
    const { data } = await this.client.get('/ipam', { params });
    return data;
  }

  async getNetworkIPAM(networkId: string): Promise<IPAMAllocation[]> {
    const { data } = await this.client.get(`/networks/${networkId}/ipam`);
    return data;
  }

  // CIDR suggestion endpoint
  async getAvailableCIDRs(maxPeers: number, count: number = 1, baseCIDR?: string): Promise<AvailableCIDRsResponse> {
    const params: any = { max_peers: maxPeers, count };
    if (baseCIDR) params.base_cidr = baseCIDR;
    const { data } = await this.client.get('/ipam/available-cidrs', { params });
    return data;
  }

  // Security endpoints
  async getPeerSessionStatus(networkId: string, peerId: string): Promise<PeerSessionStatus> {
    const { data } = await this.client.get(`/networks/${networkId}/peers/${peerId}/session`);
    return data;
  }

  async getNetworkSessions(networkId: string): Promise<PeerSessionStatus[]> {
    const { data } = await this.client.get(`/networks/${networkId}/sessions`);
    return data;
  }

  // Security incidents (mock for now - will be implemented in backend)
  async getSecurityIncidents(resolved?: boolean): Promise<SecurityIncident[]> {
    // This will need backend implementation to track blocked peers
    const params: any = {};
    if (resolved !== undefined) params.resolved = resolved;
    const { data } = await this.client.get('/security/incidents', { params });
    return data;
  }

  async resolveSecurityIncident(incidentId: string): Promise<void> {
    await this.client.post(`/security/incidents/${incidentId}/resolve`);
  }

  async reconnectPeer(networkId: string, peerId: string): Promise<void> {
    // Re-create connections to all jump servers for this peer
    await this.client.post(`/networks/${networkId}/peers/${peerId}/reconnect`);
  }

  // Authentication endpoints
  async getAuthConfig(): Promise<AuthConfig> {
    const { data } = await this.client.get('/auth/config');
    return data;
  }

  async exchangeToken(code: string, redirectUri: string): Promise<{ access_token: string; token_type: string; expires_in: number }> {
    const { data } = await this.client.post('/auth/token', {
      code,
      redirect_uri: redirectUri,
    });
    return data;
  }

  async getCurrentUser(): Promise<User> {
    const { data } = await this.client.get('/users/me');
    return data;
  }

  // User management endpoints
  async listUsers(): Promise<User[]> {
    const { data } = await this.client.get('/users');
    return data;
  }

  async getUser(userId: string): Promise<User> {
    const { data } = await this.client.get(`/users/${userId}`);
    return data;
  }

  async updateUser(userId: string, updates: UserUpdateRequest): Promise<User> {
    const { data } = await this.client.put(`/users/${userId}`, updates);
    return data;
  }

  async deleteUser(userId: string): Promise<void> {
    await this.client.delete(`/users/${userId}`);
  }

  async getDefaultPermissions(): Promise<DefaultNetworkPermissions> {
    const { data } = await this.client.get('/users/defaults');
    return data;
  }

  async updateDefaultPermissions(permissions: DefaultNetworkPermissions): Promise<DefaultNetworkPermissions> {
    const { data } = await this.client.put('/users/defaults', permissions);
    return data;
  }
}

export default new ApiClient();
