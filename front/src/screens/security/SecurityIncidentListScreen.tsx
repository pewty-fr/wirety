import React, { useState, useEffect, useCallback } from 'react';
import { View, FlatList, StyleSheet, RefreshControl } from 'react-native';
import { Text, Card, Chip, Button, Searchbar, SegmentedButtons } from 'react-native-paper';
import { useNavigation, useFocusEffect } from '@react-navigation/native';
import api from '../../services/api';
import { SecurityIncident } from '../../types/security';
import { useDebounce } from '../../hooks/useDebounce';

export const SecurityIncidentListScreen = () => {
  const navigation = useNavigation();
  const [incidents, setIncidents] = useState<SecurityIncident[]>([]);
  const [loading, setLoading] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const debouncedSearchQuery = useDebounce(searchQuery, 500);
  const [filter, setFilter] = useState<'all' | 'active' | 'resolved'>('active');

  const loadIncidents = async (isRefreshing = false) => {
    if (isRefreshing) {
      setRefreshing(true);
    } else {
      setLoading(true);
    }
    try {
      const resolvedFilter = filter === 'all' ? undefined : filter === 'resolved';
      const data = await api.getSecurityIncidents(resolvedFilter);
      
      // Filter by search query
      let filtered = data;
      if (debouncedSearchQuery) {
        const query = debouncedSearchQuery.toLowerCase();
        filtered = data.filter(
          (incident) =>
            incident.peer_name.toLowerCase().includes(query) ||
            incident.network_name.toLowerCase().includes(query) ||
            incident.public_key.toLowerCase().includes(query)
        );
      }
      
      setIncidents(filtered);
    } catch (error) {
      console.error('Failed to load security incidents:', error);
      // For now, use mock data if backend not ready
      setIncidents([]);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useFocusEffect(
    useCallback(() => {
      loadIncidents();
    }, [filter, debouncedSearchQuery])
  );

  const handleReconnect = async (incident: SecurityIncident) => {
    try {
      await api.reconnectPeer(incident.network_id, incident.peer_id);
      if (incident.id) {
        await api.resolveSecurityIncident(incident.id);
      }
      loadIncidents();
    } catch (error) {
      console.error('Failed to reconnect peer:', error);
    }
  };

  const getIncidentColor = (type: string) => {
    switch (type) {
      case 'shared_config':
        return '#f44336';
      case 'session_conflict':
        return '#ff9800';
      case 'suspicious_activity':
        return '#ff5722';
      default:
        return '#9e9e9e';
    }
  };

  const getIncidentLabel = (type: string) => {
    switch (type) {
      case 'shared_config':
        return 'Shared Config';
      case 'session_conflict':
        return 'Session Conflict';
      case 'suspicious_activity':
        return 'Suspicious Activity';
      default:
        return type;
    }
  };

  const renderIncident = ({ item }: { item: SecurityIncident }) => (
    <Card style={styles.card}>
      <Card.Title
        title={item.peer_name}
        subtitle={`Network: ${item.network_name}`}
        right={() => (
          <Chip
            mode="flat"
            style={[styles.chip, { backgroundColor: getIncidentColor(item.incident_type) }]}
            textStyle={styles.chipText}
          >
            {getIncidentLabel(item.incident_type)}
          </Chip>
        )}
      />
      <Card.Content>
        <Text style={styles.detail}>
          <Text style={styles.label}>Detected: </Text>
          {new Date(item.detected_at).toLocaleString()}
        </Text>
        <Text style={styles.detail}>
          <Text style={styles.label}>Public Key: </Text>
          {item.public_key.substring(0, 16)}...
        </Text>
        {item.endpoints && item.endpoints.length > 0 && (
          <Text style={styles.detail}>
            <Text style={styles.label}>Endpoints: </Text>
            {item.endpoints.join(', ')}
          </Text>
        )}
        <Text style={styles.detail}>
          <Text style={styles.label}>Details: </Text>
          {item.details}
        </Text>
        {item.resolved && (
          <Text style={[styles.detail, styles.resolved]}>
            <Text style={styles.label}>Resolved: </Text>
            {new Date(item.resolved_at!).toLocaleString()}
            {item.resolved_by && ` by ${item.resolved_by}`}
          </Text>
        )}
      </Card.Content>
      {!item.resolved && (
        <Card.Actions>
          <Button
            mode="outlined"
            onPress={() =>
              navigation.navigate('PeerView' as never, {
                networkId: item.network_id,
                id: item.peer_id,
              } as never)
            }
          >
            View Peer
          </Button>
          <Button
            mode="contained"
            onPress={() => handleReconnect(item)}
          >
            Reconnect
          </Button>
        </Card.Actions>
      )}
    </Card>
  );

  return (
    <View style={styles.container}>
      <Searchbar
        placeholder="Search incidents..."
        onChangeText={setSearchQuery}
        value={searchQuery}
        style={styles.searchbar}
      />
      
      <SegmentedButtons
        value={filter}
        onValueChange={(value) => setFilter(value as 'all' | 'active' | 'resolved')}
        buttons={[
          { value: 'active', label: 'Active' },
          { value: 'all', label: 'All' },
          { value: 'resolved', label: 'Resolved' },
        ]}
        style={styles.segmented}
      />

      <FlatList
        data={incidents}
        renderItem={renderIncident}
        keyExtractor={(item) => item.id}
        contentContainerStyle={styles.list}
        refreshControl={
          <RefreshControl refreshing={refreshing} onRefresh={() => loadIncidents(true)} />
        }
        ListEmptyComponent={
          <View style={styles.empty}>
            <Text style={styles.emptyText}>
              {filter === 'active' ? 'No active security incidents' : 'No security incidents found'}
            </Text>
          </View>
        }
      />
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f5f5f5',
  },
  searchbar: {
    margin: 16,
    marginBottom: 8,
  },
  segmented: {
    marginHorizontal: 16,
    marginBottom: 16,
  },
  list: {
    padding: 16,
    paddingTop: 0,
  },
  card: {
    marginBottom: 16,
  },
  chip: {
    marginRight: 16,
  },
  chipText: {
    color: 'white',
    fontSize: 12,
  },
  detail: {
    marginBottom: 8,
    fontSize: 14,
  },
  label: {
    fontWeight: 'bold',
  },
  resolved: {
    color: '#4caf50',
  },
  empty: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: 32,
  },
  emptyText: {
    fontSize: 16,
    color: '#666',
    textAlign: 'center',
  },
});
