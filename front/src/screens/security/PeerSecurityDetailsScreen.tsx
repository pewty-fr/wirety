import React, { useState, useEffect } from 'react';
import { View, ScrollView, StyleSheet, RefreshControl } from 'react-native';
import { Text, Card, List, Chip, Button, Divider } from 'react-native-paper';
import { useRoute, useNavigation } from '@react-navigation/native';
import api from '../../services/api';
import { PeerSessionStatus } from '../../types/security';
import { Peer } from '../../types/api';

export const PeerSecurityDetailsScreen = () => {
  const route = useRoute();
  const navigation = useNavigation();
  const { networkId, peerId } = route.params as { networkId: string; peerId: string };
  const [sessionStatus, setSessionStatus] = useState<PeerSessionStatus | null>(null);
  const [peer, setPeer] = useState<Peer | null>(null);
  const [loading, setLoading] = useState(false);
  const [refreshing, setRefreshing] = useState(false);

  const loadData = async (isRefreshing = false) => {
    if (isRefreshing) {
      setRefreshing(true);
    } else {
      setLoading(true);
    }
    try {
      const [statusData, peerData] = await Promise.all([
        api.getPeerSessionStatus(networkId, peerId),
        api.getPeer(networkId, peerId),
      ]);
      setSessionStatus(statusData);
      setPeer(peerData);
    } catch (error) {
      console.error('Failed to load peer security details:', error);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    loadData();
  }, [networkId, peerId]);

  if (!sessionStatus || !peer) {
    return (
      <View style={styles.container}>
        <Text>Loading...</Text>
      </View>
    );
  }

  return (
    <ScrollView
      style={styles.container}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={() => loadData(true)} />
      }
    >
      <Card style={styles.card}>
        <Card.Title title="Security Status" />
        <Card.Content>
          <View style={styles.statusRow}>
            <Text style={styles.label}>Active Agent:</Text>
            <Chip
              mode="flat"
              style={[
                styles.statusChip,
                { backgroundColor: sessionStatus.has_active_agent ? '#4caf50' : '#f44336' },
              ]}
              textStyle={styles.chipText}
            >
              {sessionStatus.has_active_agent ? 'Connected' : 'Disconnected'}
            </Chip>
          </View>
          {sessionStatus.suspicious_activity && (
            <View style={styles.statusRow}>
              <Chip
                mode="flat"
                icon="alert"
                style={[styles.statusChip, { backgroundColor: '#ff5722' }]}
                textStyle={styles.chipText}
              >
                Suspicious Activity Detected
              </Chip>
            </View>
          )}
        </Card.Content>
      </Card>

      {sessionStatus.current_session && (
        <Card style={styles.card}>
          <Card.Title title="Current Session" />
          <Card.Content>
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
          </Card.Content>
        </Card>
      )}

      {sessionStatus.conflicting_sessions && sessionStatus.conflicting_sessions.length > 0 && (
        <Card style={styles.card}>
          <Card.Title
            title="Conflicting Sessions"
            subtitle="Multiple agents detected using this peer"
          />
          <Card.Content>
            {sessionStatus.conflicting_sessions.map((session, index) => (
              <View key={session.session_id}>
                {index > 0 && <Divider style={styles.divider} />}
                <List.Item
                  title={session.hostname}
                  description={`Last seen: ${new Date(session.last_seen).toLocaleString()}`}
                  left={(props) => <List.Icon {...props} icon="alert-circle" color="#f44336" />}
                />
              </View>
            ))}
          </Card.Content>
        </Card>
      )}

      {sessionStatus.recent_endpoint_changes && sessionStatus.recent_endpoint_changes.length > 0 && (
        <Card style={styles.card}>
          <Card.Title
            title="Recent Endpoint Changes"
            subtitle="Last 24 hours"
          />
          <Card.Content>
            {sessionStatus.recent_endpoint_changes.map((change, index) => (
              <View key={index}>
                {index > 0 && <Divider style={styles.divider} />}
                <Text style={styles.changeText}>
                  {new Date(change.changed_at).toLocaleTimeString()}
                </Text>
                <Text style={styles.endpointChange}>
                  {change.old_endpoint} → {change.new_endpoint}
                </Text>
              </View>
            ))}
          </Card.Content>
        </Card>
      )}
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
  card: {
    margin: 16,
    marginBottom: 0,
  },
  statusRow: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 8,
  },
  label: {
    fontSize: 16,
    marginRight: 12,
    fontWeight: '500',
  },
  statusChip: {
    height: 28,
  },
  chipText: {
    color: 'white',
    fontSize: 12,
  },
  divider: {
    marginVertical: 8,
  },
  changeText: {
    fontSize: 12,
    color: '#666',
    marginBottom: 4,
  },
  endpointChange: {
    fontSize: 14,
    fontFamily: 'monospace',
    marginBottom: 8,
  },
});
