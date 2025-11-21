import { useQuery } from '@tanstack/react-query';
import api from '../api/client';
import type { Network, ACL, SecurityIncident } from '../types';

// Query Keys
export const queryKeys = {
  networks: ['networks'] as const,
  network: (networkId: string) => ['network', networkId] as const,
  peers: (page: number, pageSize: number) => ['peers', page, pageSize] as const,
  peer: (networkId: string, peerId: string) => ['peer', networkId, peerId] as const,
  peerSession: (networkId: string, peerId: string) => ['peerSession', networkId, peerId] as const,
  networkPeers: (networkId: string) => ['networkPeers', networkId] as const,
  acl: (networkId: string) => ['acl', networkId] as const,
  acls: (networkIds: string[]) => ['acls', networkIds] as const,
  incidents: (resolved?: boolean) => ['incidents', resolved] as const,
};

// Networks Query
export function useNetworks() {
  return useQuery({
    queryKey: queryKeys.networks,
    queryFn: async () => {
      const response = await api.getNetworks(1, 100);
      return response.data || [];
    },
  });
}

// Single Network Query
export function useNetwork(networkId: string, enabled: boolean = true) {
  return useQuery({
    queryKey: queryKeys.network(networkId),
    queryFn: () => api.getNetwork(networkId),
    enabled: enabled && !!networkId,
    staleTime: 30000, // 30 seconds
  });
}

// Peers Query
export function usePeers(page: number, pageSize: number) {
  return useQuery({
    queryKey: queryKeys.peers(page, pageSize),
    queryFn: async () => {
      const response = await api.getAllPeers(page, pageSize);
      return {
        peers: response.peers || [],
        total: response.total || 0,
      };
    },
    refetchInterval: 20000, // auto refresh every 20s for list view statuses
    refetchIntervalInBackground: true,
  });
}

// Single Peer Query
export function usePeer(networkId: string, peerId: string, poll: boolean = true) {
  return useQuery({
    queryKey: queryKeys.peer(networkId, peerId),
    queryFn: () => api.getPeer(networkId, peerId),
    enabled: !!networkId && !!peerId,
    refetchInterval: poll ? 20000 : false,
    refetchIntervalInBackground: true,
  });
}

// Peer Session Query
export function usePeerSession(networkId: string, peerId: string, enabled: boolean = true) {
  return useQuery({
    queryKey: queryKeys.peerSession(networkId, peerId),
    queryFn: () => api.getPeerSessionStatus(networkId, peerId),
    enabled: enabled && !!networkId && !!peerId,
    staleTime: 5000, // 5 seconds for session data
  });
}

// Network Peers Query (for topology)
export function useNetworkPeers(networkId: string, enabled: boolean = true, poll: boolean = true) {
  return useQuery({
    queryKey: queryKeys.networkPeers(networkId),
    queryFn: () => api.getAllNetworkPeers(networkId),
    enabled: enabled && !!networkId,
    refetchInterval: poll ? 20000 : false,
    refetchIntervalInBackground: true,
  });
}

// ACL Query
export function useACL(networkId: string, enabled: boolean = true) {
  return useQuery({
    queryKey: queryKeys.acl(networkId),
    queryFn: () => api.getACL(networkId),
    enabled: enabled && !!networkId,
  });
}

// Multiple ACLs Query
export function useACLs(networks: Network[]) {
  const networkIds = networks.map(n => n.id);
  
  return useQuery({
    queryKey: queryKeys.acls(networkIds),
    queryFn: async () => {
      const acls: Record<string, ACL> = {};
      await Promise.all(
        networks.map(async (network) => {
          try {
            const acl = await api.getACL(network.id);
            acls[network.id] = acl;
          } catch (error) {
            console.warn(`Failed to load ACL for network ${network.id}:`, error);
          }
        })
      );
      return acls;
    },
    enabled: networks.length > 0,
  });
}

// Security Incidents Query
export function useSecurityIncidents(resolved?: boolean, pageSize: number = 200) {
  return useQuery({
    queryKey: queryKeys.incidents(resolved),
    queryFn: async () => {
      const response = await api.getSecurityIncidents(1, pageSize, resolved);
      const incidentPeers = new Set<string>();
      response.data.forEach((inc: SecurityIncident) => {
        if (!inc.resolved && inc.peer_id) {
          incidentPeers.add(inc.peer_id);
        }
      });
      return {
        incidents: response.data,
        incidentPeerIds: incidentPeers,
      };
    },
  });
}
