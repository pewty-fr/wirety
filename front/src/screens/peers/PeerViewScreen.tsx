import React, { useState, useEffect, useCallback } from 'react';
import { View, ScrollView, StyleSheet, Alert } from 'react-native';
import { Title, Card, Text, Button, ActivityIndicator, Chip, IconButton, List, Divider } from 'react-native-paper';
import { NetworkGraphComponent } from '../../components/NetworkGraphComponent';
import { UserMenu } from '../../components/UserMenu';
import { useNavigation, useRoute, useFocusEffect } from '@react-navigation/native';
import { NativeStackNavigationProp } from '@react-navigation/native-stack';
import { RootStackParamList } from '../../../App';
import api from '../../services/api';
import { Peer } from '../../types/api';
import { PeerSessionStatus, SecurityIncident } from '../../types/security';
import { formatDate } from '../../utils/validation';

export const PeerViewScreen = () => {
  const navigation = useNavigation<NativeStackNavigationProp<RootStackParamList>>();
  const route = useRoute();
  const { networkId, peerId } = route.params as { networkId: string; peerId: string };
  const [peer, setPeer] = useState<Peer | null>(null);
  const [sessionStatus, setSessionStatus] = useState<PeerSessionStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [networkPeers, setNetworkPeers] = useState<Peer[]>([]);
  const [incidents, setIncidents] = useState<SecurityIncident[]>([]);
  const [nodes, setNodes] = useState<any[]>([]);
  const [edges, setEdges] = useState<any[]>([]);

  useFocusEffect(
    useCallback(() => {
      loadPeer();
      loadSessionStatus();
    }, [networkId, peerId])
  );

  const loadSessionStatus = async () => {
    try {
      const data = await api.getPeerSessionStatus(networkId, peerId);
      setSessionStatus(data);
    } catch (error) {
      // Session status might not be available for all peers
      console.log('Session status not available');
    }
  };

  useEffect(() => {
    if (peer) {
      navigation.setOptions({
        headerRight: () => (
          <View style={{ flexDirection: 'row', alignItems: 'center' }}>
            <IconButton
              icon="pencil"
              onPress={() =>
                navigation.navigate(
                  peer.is_jump ? 'PeerUpdateJump' : 'PeerUpdateRegular',
                  { networkId, peerId }
                )
              }
            />
            <IconButton
              icon="delete"
              onPress={handleDelete}
            />
            <UserMenu />
          </View>
        ),
      });
    }
  }, [peer, navigation]);

  const loadPeer = async () => {
    setLoading(true);
    try {
      // Always fetch peer first; if this fails we can't render anything.
      const data = await api.getPeer(networkId, peerId);
      setPeer(data);

      // Optimistically seed graph with current peer so UI never shows "Topology unavailable".
      setNodes([
        {
          id: data.id,
            name: data.name,
            type: 'current',
            address: data.address,
            endpoint: data.endpoint,
            is_jump: data.is_jump,
            is_isolated: data.is_isolated,
            full_encapsulation: data.full_encapsulation,
        },
      ]);
      setEdges([]);

      // Fetch peers & incidents independently; tolerate failures.
      let peers: Peer[] = [];
      try {
        const peersResp = await api.getPeers(networkId, 1, 1000, '');
        peers = peersResp.data;
        setNetworkPeers(peers);
      } catch (e) {
        console.warn('Failed to load network peers; proceeding with single-node topology');
      }
      let inc: SecurityIncident[] = [];
      try {
        inc = await api.getNetworkSecurityIncidents(networkId, false);
        setIncidents(inc);
      } catch (e) {
        console.warn('Failed to load incidents; continuing without incident overlays');
      }

      if (peers.length > 0 || inc.length > 0) {
        generateGraph(data, peers, inc);
      }
    } catch (error) {
      console.error('Failed to load peer:', error);
    } finally {
      setLoading(false);
    }
  };

  const generateGraph = (currentPeer: Peer, allPeers: Peer[], incidents: SecurityIncident[]) => {
    const incidentPeerIds = new Set(incidents.filter(i => !i.resolved).map(i => i.peer_id));
    const graphNodes: any[] = [];
    const graphEdges: any[] = [];

    graphNodes.push({
      id: currentPeer.id,
      name: currentPeer.name,
      type: 'current',
      address: currentPeer.address,
      endpoint: currentPeer.endpoint,
      is_jump: currentPeer.is_jump,
      is_isolated: currentPeer.is_isolated,
      full_encapsulation: currentPeer.full_encapsulation,
      incident: incidentPeerIds.has(currentPeer.id)
    });

    const jumpServers = allPeers.filter(p => p.is_jump);
    const otherPeers = allPeers.filter(p => !p.is_jump && p.id !== currentPeer.id);

    if (currentPeer.is_jump) {
      otherPeers.forEach(p => {
        graphNodes.push({
          id: p.id,
          name: p.name,
          type: 'regular',
          address: p.address,
          is_isolated: p.is_isolated,
          full_encapsulation: p.full_encapsulation,
          incident: incidentPeerIds.has(p.id)
        });
        const blocked = p.is_isolated || incidentPeerIds.has(p.id);
        graphEdges.push({
          from: currentPeer.id,
          to: p.id,
          type: blocked ? 'blocked' : 'direct',
          label: blocked ? (p.is_isolated ? 'Isolated' : 'Incident') : 'Direct'
        });
      });
    } else {
      jumpServers.forEach(j => {
        graphNodes.push({ id: j.id, name: j.name, type: 'jump', address: j.address, incident: incidentPeerIds.has(j.id) });
        const blocked = incidentPeerIds.has(j.id) || incidentPeerIds.has(currentPeer.id);
        graphEdges.push({ from: currentPeer.id, to: j.id, type: blocked ? 'blocked' : 'tunnel', label: blocked ? 'Incident' : 'Tunnel' });
      });
      otherPeers.forEach(p => {
        graphNodes.push({ id: p.id, name: p.name, type: 'regular', address: p.address, incident: incidentPeerIds.has(p.id), is_isolated: p.is_isolated });
        jumpServers.forEach(j => {
          const blocked = p.is_isolated || incidentPeerIds.has(p.id) || incidentPeerIds.has(j.id);
          graphEdges.push({ from: j.id, to: p.id, type: blocked ? 'blocked' : 'tunnel', label: blocked ? (p.is_isolated ? 'Isolated' : 'Incident') : 'Via Jump' });
        });
      });
    }
    setNodes(graphNodes);
    setEdges(graphEdges);
  };

  const handleDelete = async () => {
    Alert.alert('Delete Peer', 'Are you sure you want to delete this peer?', [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Delete',
        style: 'destructive',
        onPress: async () => {
          try {
            await api.deletePeer(networkId, peerId);
            navigation.goBack();
          } catch (error) {
            console.error('Failed to delete peer:', error);
          }
        },
      },
    ]);
  };

  if (loading) {
    return (
      <View style={styles.centered}>
        <ActivityIndicator size="large" />
      </View>
    );
  }

  if (!peer) {
    return (
      <View style={styles.centered}>
        <Text>Peer not found</Text>
      </View>
    );
  }

  return (
    <ScrollView style={styles.container}>
      <Card style={styles.card}>
        <Card.Title title={peer.name} />
        <Card.Content>
          <View style={styles.chips}>
            {peer.is_jump && <Chip mode="flat">Jump Server</Chip>}
            {!peer.is_jump && peer.use_agent && <Chip mode="flat" icon="cloud">Agent-Based</Chip>}
            {!peer.is_jump && !peer.use_agent && <Chip mode="flat" icon="cog">Static Config</Chip>}
            {peer.is_isolated && <Chip mode="flat">Isolated</Chip>}
            {peer.full_encapsulation && <Chip mode="flat">Full Encapsulation</Chip>}
            {sessionStatus?.has_active_agent && (
              <Chip mode="flat" icon="check-circle" style={{ backgroundColor: '#4caf50' }} textStyle={{ color: 'white' }}>
                Connected
              </Chip>
            )}
            {sessionStatus?.suspicious_activity && (
              <Chip mode="flat" icon="alert" style={{ backgroundColor: '#ff5722' }} textStyle={{ color: 'white' }}>
                Security Alert
              </Chip>
            )}
            {sessionStatus?.conflicting_sessions && sessionStatus.conflicting_sessions.length > 0 && (
              <Chip mode="flat" icon="alert-circle" style={{ backgroundColor: '#f44336' }} textStyle={{ color: 'white' }}>
                Session Conflict
              </Chip>
            )}
          </View>

          <View style={styles.row}>
            <Text style={styles.label}>Address:</Text>
            <Text>{peer.address}</Text>
          </View>
          <View style={styles.row}>
            <Text style={styles.label}>Endpoint:</Text>
            <Text>{peer.endpoint || 'N/A'}</Text>
          </View>
          {peer.listen_port && (
            <View style={styles.row}>
              <Text style={styles.label}>Listen Port:</Text>
              <Text>{peer.listen_port}</Text>
            </View>
          )}
          {peer.jump_nat_interface && (
            <View style={styles.row}>
              <Text style={styles.label}>NAT Interface:</Text>
              <Text>{peer.jump_nat_interface}</Text>
            </View>
          )}
          {peer.additional_allowed_ips && peer.additional_allowed_ips.length > 0 && (
            <View style={styles.row}>
              <Text style={styles.label}>Additional IPs:</Text>
              <Text>{peer.additional_allowed_ips.join(', ')}</Text>
            </View>
          )}
          <View style={styles.row}>
            <Text style={styles.label}>Created:</Text>
            <Text>{formatDate(peer.created_at)}</Text>
          </View>
          <View style={styles.row}>
            <Text style={styles.label}>Updated:</Text>
            <Text>{formatDate(peer.updated_at)}</Text>
          </View>
        </Card.Content>
      </Card>

      {sessionStatus && (
        <Card style={styles.card}>
          <Card.Title title="Security Details" />
          <Card.Content>
            <View style={styles.securityStatus}>
              <Text style={styles.label}>Active Agent:</Text>
              <Chip
                mode="flat"
                style={[
                  styles.statusChip,
                  { backgroundColor: sessionStatus.has_active_agent ? '#4caf50' : '#f44336' },
                ]}
                textStyle={styles.statusChipText}
              >
                {sessionStatus.has_active_agent ? 'Connected' : 'Disconnected'}
              </Chip>
            </View>

            {sessionStatus.current_session && (
              <>
                <Divider style={styles.divider} />
                <Text style={styles.sectionTitle}>Current Session</Text>
                <List.Item
                  title="Hostname"
                  description={sessionStatus.current_session.hostname}
                  left={(props) => <List.Icon {...props} icon="laptop" />}
                />
                <List.Item
                  title="Endpoint"
                  description={sessionStatus.current_session.reported_endpoint || 'N/A'}
                  left={(props) => <List.Icon {...props} icon="ip-network" />}
                />
                <List.Item
                  title="System Uptime"
                  description={formatUptime(sessionStatus.current_session.system_uptime)}
                  left={(props) => <List.Icon {...props} icon="clock-outline" />}
                />
                <List.Item
                  title="Last Seen"
                  description={new Date(sessionStatus.current_session.last_seen).toLocaleString()}
                  left={(props) => <List.Icon {...props} icon="update" />}
                />
              </>
            )}

            {sessionStatus.conflicting_sessions && sessionStatus.conflicting_sessions.length > 0 && (
              <>
                <Divider style={styles.divider} />
                <Text style={styles.sectionTitle}>Conflicting Sessions</Text>
                <Text style={styles.warningText}>Multiple agents detected using this peer</Text>
                {sessionStatus.conflicting_sessions.map((session, index) => (
                  <List.Item
                    key={session.session_id}
                    title={session.hostname}
                    description={`Last seen: ${new Date(session.last_seen).toLocaleString()}`}
                    left={(props) => <List.Icon {...props} icon="alert-circle" color="#f44336" />}
                  />
                ))}
              </>
            )}

            {sessionStatus.recent_endpoint_changes && sessionStatus.recent_endpoint_changes.length > 0 && (
              <>
                <Divider style={styles.divider} />
                <Text style={styles.sectionTitle}>Recent Endpoint Changes (24h)</Text>
                {sessionStatus.recent_endpoint_changes.map((change, index) => (
                  <View key={index} style={styles.endpointChange}>
                    <Text style={styles.changeTime}>
                      {new Date(change.changed_at).toLocaleTimeString()}
                    </Text>
                    <Text style={styles.changeEndpoints}>
                      {change.old_endpoint} → {change.new_endpoint}
                    </Text>
                  </View>
                ))}
              </>
            )}
          </Card.Content>
        </Card>
      )}

      <Card style={styles.card}>
        <Card.Title title="Network Topology" />
        <Card.Content>
          {nodes.length === 0 ? (
            <Text style={{ color: '#666' }}>Topology unavailable</Text>
          ) : (
            <NetworkGraphComponent nodes={nodes} edges={edges} currentPeerId={peer.id} />
          )}
        </Card.Content>
      </Card>

      <View style={styles.actions}>
        {/* Embedded network graph replaces separate navigation */}
        
        {/* Jump peers: only show token (always use agent) */}
        {peer.is_jump && (
          <Button
            mode="contained"
            onPress={() => navigation.navigate('PeerToken', { networkId, peerId })}
            style={styles.button}
          >
            View Token
          </Button>
        )}
        
        {/* Regular peers: show token for agent-based, config for static */}
        {!peer.is_jump && peer.use_agent && (
          <Button
            mode="contained"
            onPress={() => navigation.navigate('PeerToken', { networkId, peerId })}
            style={styles.button}
          >
            View Token
          </Button>
        )}
        
        {!peer.is_jump && !peer.use_agent && (
          <Button
            mode="contained"
            onPress={() => navigation.navigate('PeerConfig', { networkId, peerId })}
            style={styles.button}
          >
            View Config
          </Button>
        )}
      </View>
    </ScrollView>
  );
};

function formatUptime(seconds: number): string {
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  const parts = [];
  if (days > 0) parts.push(`${days}d`);
  if (hours > 0) parts.push(`${hours}h`);
  if (minutes > 0) parts.push(`${minutes}m`);

  return parts.length > 0 ? parts.join(' ') : '< 1m';
}

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
  card: {
    margin: 16,
  },
  chips: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
    marginBottom: 16,
  },
  row: {
    flexDirection: 'row',
    marginBottom: 8,
    flexWrap: 'wrap',
  },
  label: {
    fontWeight: 'bold',
    marginRight: 8,
    width: 120,
  },
  securityStatus: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 8,
  },
  statusChip: {
    height: 28,
  },
  statusChipText: {
    color: 'white',
    fontSize: 12,
  },
  divider: {
    marginVertical: 12,
  },
  sectionTitle: {
    fontSize: 16,
    fontWeight: 'bold',
    marginBottom: 8,
  },
  warningText: {
    fontSize: 14,
    color: '#666',
    marginBottom: 8,
  },
  endpointChange: {
    marginBottom: 12,
  },
  changeTime: {
    fontSize: 12,
    color: '#666',
    marginBottom: 4,
  },
  changeEndpoints: {
    fontSize: 14,
    fontFamily: 'monospace',
  },
  actions: {
    padding: 16,
  },
  button: {
    marginBottom: 12,
  },
});
