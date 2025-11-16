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

  setBaseURL(url: string) {
    this.client.defaults.baseURL = url;
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
}

export default new ApiClient();
