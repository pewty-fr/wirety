import React, { useState, useEffect } from 'react';
import { View, ScrollView, StyleSheet } from 'react-native';
import { Title, Card, Text, ActivityIndicator, Chip } from 'react-native-paper';
import { useNavigation, useRoute } from '@react-navigation/native';
import api from '../../services/api';
import { Peer, Network } from '../../types/api';
import { NetworkGraphComponent } from '../../components/NetworkGraphComponent';

interface GraphNode {
  id: string;
  name: string;
  type: 'current' | 'jump' | 'regular' | 'internet' | 'network';
  address?: string;
  endpoint?: string;
  isolated?: boolean;
  fullEncapsulation?: boolean;
  is_jump?: boolean;
  jump_nat_interface?: string;
  is_isolated?: boolean;
  full_encapsulation?: boolean;
}

interface GraphEdge {
  from: string;
  to: string;
  type: 'direct' | 'tunnel' | 'blocked' | 'internet';
  label?: string;
}

export const PeerNetworkGraphScreen = () => {
  const navigation = useNavigation();
  const route = useRoute();
  const { networkId, peerId } = route.params as { networkId: string; peerId: string };
  const [currentPeer, setCurrentPeer] = useState<Peer | null>(null);
  const [network, setNetwork] = useState<Network | null>(null);
  const [allPeers, setAllPeers] = useState<Peer[]>([]);
  const [loading, setLoading] = useState(true);
  const [nodes, setNodes] = useState<GraphNode[]>([]);
  const [edges, setEdges] = useState<GraphEdge[]>([]);

  useEffect(() => {
    loadNetworkData();
  }, [networkId, peerId]);

  const loadNetworkData = async () => {
    setLoading(true);
    try {
      const [peerData, networkData, peersData] = await Promise.all([
        api.getPeer(networkId, peerId),
        api.getNetwork(networkId),
        api.getPeers(networkId, 1, 1000, '').then(response => response.data)
      ]);
      
      setCurrentPeer(peerData);
      setNetwork(networkData);
      setAllPeers(peersData);
      generateGraph(peerData, networkData, peersData);
    } catch (error) {
      console.error('Failed to load network data:', error);
    } finally {
      setLoading(false);
    }
  };

  const generateGraph = (currentPeer: Peer, network: Network, allPeers: Peer[]) => {
    const graphNodes: GraphNode[] = [];
    const graphEdges: GraphEdge[] = [];

    // Add current peer node
    graphNodes.push({
      id: currentPeer.id,
      name: currentPeer.name,
      type: 'current',
      address: currentPeer.address,
      endpoint: currentPeer.endpoint,
      isolated: currentPeer.is_isolated,
      fullEncapsulation: currentPeer.full_encapsulation,
      is_jump: currentPeer.is_jump,
      jump_nat_interface: currentPeer.jump_nat_interface,
      is_isolated: currentPeer.is_isolated,
      full_encapsulation: currentPeer.full_encapsulation
    });

    // Get jump servers
    const jumpServers = allPeers.filter(p => p.is_jump);
    
    // Get other regular peers (excluding current peer)
    const otherPeers = allPeers.filter(p => !p.is_jump && p.id !== currentPeer.id);

    if (currentPeer.is_jump) {
      // JUMP SERVER PERSPECTIVE
      // Jump server can see all other peers
      otherPeers.forEach(peer => {
        graphNodes.push({
          id: peer.id,
          name: peer.name,
          type: 'regular',
          address: peer.address,
          isolated: peer.is_isolated,
          fullEncapsulation: peer.full_encapsulation,
          is_jump: peer.is_jump,
          jump_nat_interface: peer.jump_nat_interface,
          is_isolated: peer.is_isolated,
          full_encapsulation: peer.full_encapsulation
        });

        // Direct connection to all peers
        graphEdges.push({
          from: currentPeer.id,
          to: peer.id,
          type: peer.is_isolated ? 'blocked' : 'direct',
          label: peer.is_isolated ? 'Isolated' : 'Direct'
        });
      });

      // Other jump servers
      jumpServers.filter(js => js.id !== currentPeer.id).forEach(jumpServer => {
        graphNodes.push({
          id: jumpServer.id,
          name: jumpServer.name,
          type: 'jump',
          address: jumpServer.address,
          endpoint: jumpServer.endpoint,
          isolated: jumpServer.is_isolated,
          fullEncapsulation: jumpServer.full_encapsulation,
          is_jump: jumpServer.is_jump,
          jump_nat_interface: jumpServer.jump_nat_interface,
          is_isolated: jumpServer.is_isolated,
          full_encapsulation: jumpServer.full_encapsulation
        });

        graphEdges.push({
          from: currentPeer.id,
          to: jumpServer.id,
          type: 'direct',
          label: 'Jump-to-Jump'
        });
      });

      // Internet access via NAT interface
      if (currentPeer.jump_nat_interface) {
        graphNodes.push({
          id: 'internet',
          name: 'Internet',
          type: 'internet'
        });

        graphEdges.push({
          from: currentPeer.id,
          to: 'internet',
          type: 'internet',
          label: `via ${currentPeer.jump_nat_interface}`
        });
      }

    } else {
      // REGULAR PEER PERSPECTIVE
      
      // Add jump servers (regular peers connect through jumps)
      jumpServers.forEach(jumpServer => {
        graphNodes.push({
          id: jumpServer.id,
          name: jumpServer.name,
          type: 'jump',
          address: jumpServer.address,
          endpoint: jumpServer.endpoint,
          isolated: jumpServer.is_isolated,
          fullEncapsulation: jumpServer.full_encapsulation,
          is_jump: jumpServer.is_jump,
          jump_nat_interface: jumpServer.jump_nat_interface,
          is_isolated: jumpServer.is_isolated,
          full_encapsulation: jumpServer.full_encapsulation
        });

        graphEdges.push({
          from: currentPeer.id,
          to: jumpServer.id,
          type: 'tunnel',
          label: 'Tunnel'
        });

        // If jump has NAT interface and current peer has full encapsulation, show internet access
        if (jumpServer.jump_nat_interface && currentPeer.full_encapsulation) {
          if (!graphNodes.find(n => n.id === 'internet')) {
            graphNodes.push({
              id: 'internet',
              name: 'Internet',
              type: 'internet'
            });
          }

          // Full encapsulation: all traffic goes through jump
          graphEdges.push({
            from: jumpServer.id,
            to: 'internet',
            type: 'internet',
            label: 'Full Traffic'
          });
        }
      });

      // Add ALL other peers to show complete network topology
      otherPeers.forEach(peer => {
        graphNodes.push({
          id: peer.id,
          name: peer.name,
          type: 'regular',
          address: peer.address,
          isolated: peer.is_isolated,
          fullEncapsulation: peer.full_encapsulation,
          is_jump: peer.is_jump,
          jump_nat_interface: peer.jump_nat_interface,
          is_isolated: peer.is_isolated,
          full_encapsulation: peer.full_encapsulation
        });

        // Connection goes through jump server
        jumpServers.forEach(jumpServer => {
          if (!graphEdges.find(e => e.from === jumpServer.id && e.to === peer.id)) {
            graphEdges.push({
              from: jumpServer.id,
              to: peer.id,
              type: 'tunnel',
              label: 'Via Jump'
            });
          }
        });
      });

      // Show network CIDR as a node for context
      graphNodes.push({
        id: 'network',
        name: `${network.name}\n${network.cidr}`,
        type: 'network'
      });
    }

    setNodes(graphNodes);
    setEdges(graphEdges);
  };

  if (loading) {
    return (
      <View style={styles.centered}>
        <ActivityIndicator size="large" />
        <Text style={styles.loadingText}>Loading network topology...</Text>
      </View>
    );
  }

  if (!currentPeer || !network) {
    return (
      <View style={styles.centered}>
        <Text>Network data not found</Text>
      </View>
    );
  }

  const getConnectivityDescription = () => {
    if (currentPeer.is_jump) {
      return `As a jump server, ${currentPeer.name} can directly communicate with all peers in the network and route traffic to the internet.`;
    } else if (currentPeer.is_isolated && currentPeer.full_encapsulation) {
      return `${currentPeer.name} is isolated from other peers but routes all traffic through jump servers to access the internet.`;
    } else if (currentPeer.is_isolated) {
      return `${currentPeer.name} is isolated from other peers and can only access the network through jump servers.`;
    } else if (currentPeer.full_encapsulation) {
      return `${currentPeer.name} can communicate with other non-isolated peers and routes all traffic through jump servers.`;
    } else {
      return `${currentPeer.name} can communicate with other non-isolated peers through jump servers.`;
    }
  };

  return (
    <ScrollView style={styles.container}>
      <Card style={styles.card}>
        <Card.Title title={`Network View: ${currentPeer.name}`} />
        <Card.Content>
          <View style={styles.peerInfo}>
            <Text style={styles.address}>{currentPeer.address}</Text>
            <View style={styles.chips}>
              {currentPeer.is_jump && <Chip mode="flat">Jump Server</Chip>}
              {currentPeer.is_isolated && <Chip mode="flat">Isolated</Chip>}
              {currentPeer.full_encapsulation && <Chip mode="flat">Full Encapsulation</Chip>}
            </View>
          </View>
          
          <Text style={styles.description}>{getConnectivityDescription()}</Text>
          
          <View style={styles.networkInfo}>
            <Text style={styles.networkLabel}>Network: {network.name}</Text>
            <Text style={styles.networkLabel}>CIDR: {network.cidr}</Text>
            <Text style={styles.networkLabel}>Domain: {network.domain}</Text>
          </View>
        </Card.Content>
      </Card>

      <Card style={styles.card}>
        <Card.Title title="Network Topology" />
        <Card.Content>
          <NetworkGraphComponent 
            nodes={nodes} 
            edges={edges} 
            currentPeerId={currentPeer.id}
          />
        </Card.Content>
      </Card>

      <Card style={styles.card}>
        <Card.Title title="Connection Summary" />
        <Card.Content>
          <Text style={styles.summaryTitle}>Direct Connections:</Text>
          {edges.filter(e => e.from === currentPeer.id).map(edge => {
            const targetNode = nodes.find(n => n.id === edge.to);
            return (
              <Text key={edge.to} style={styles.connection}>
                → {targetNode?.name} ({edge.label})
              </Text>
            );
          })}
          
          {edges.filter(e => e.from === currentPeer.id).length === 0 && (
            <Text style={styles.noConnections}>No direct connections</Text>
          )}
        </Card.Content>
      </Card>
    </ScrollView>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f5f5f5',
  },
  centered: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  loadingText: {
    marginTop: 16,
    color: '#666',
  },
  card: {
    margin: 16,
  },
  peerInfo: {
    marginBottom: 16,
  },
  address: {
    fontSize: 18,
    fontWeight: 'bold',
    marginBottom: 8,
  },
  chips: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
  },
  description: {
    fontSize: 16,
    lineHeight: 22,
    marginBottom: 16,
    color: '#666',
  },
  networkInfo: {
    backgroundColor: '#f0f0f0',
    padding: 12,
    borderRadius: 8,
  },
  networkLabel: {
    fontSize: 14,
    marginBottom: 4,
  },
  summaryTitle: {
    fontSize: 16,
    fontWeight: 'bold',
    marginBottom: 8,
  },
  connection: {
    fontSize: 14,
    marginLeft: 8,
    marginBottom: 4,
    fontFamily: 'monospace',
  },
  noConnections: {
    fontSize: 14,
    fontStyle: 'italic',
    color: '#666',
  },
});
