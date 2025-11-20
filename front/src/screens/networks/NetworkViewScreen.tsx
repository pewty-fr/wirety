import React, { useState, useEffect, useCallback } from 'react';
import { View, ScrollView, StyleSheet } from 'react-native';
import { Title, Card, Text, Button, ActivityIndicator, IconButton } from 'react-native-paper';
import { UserMenu } from '../../components/UserMenu';
import { useNavigation, useRoute, useFocusEffect } from '@react-navigation/native';
import { NativeStackNavigationProp } from '@react-navigation/native-stack';
import { RootStackParamList } from '../../../App';
import api from '../../services/api';
import { Network, Peer } from '../../types/api';
import { computeCapacityFromCIDR } from '../../utils/networkCapacity';
import { formatDate } from '../../utils/validation';

export const NetworkViewScreen = () => {
  const navigation = useNavigation<NativeStackNavigationProp<RootStackParamList>>();
  const route = useRoute();
  const { id } = route.params as { id: string };
  const [network, setNetwork] = useState<Network | null>(null);
  const [loading, setLoading] = useState(true);
  const [peerCount, setPeerCount] = useState<number | null>(null);
  const [capacity, setCapacity] = useState<number | null>(null);

  useFocusEffect(
    useCallback(() => {
      loadNetwork();
    }, [id])
  );

  useEffect(() => {
    if (network) {
      navigation.setOptions({
        headerRight: () => (
          <View style={{ flexDirection: 'row', alignItems: 'center' }}>
            <IconButton
              icon="pencil"
              onPress={() => navigation.navigate('NetworkUpdate', { id })}
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
  }, [network, navigation]);

  const loadNetwork = async () => {
    setLoading(true);
    try {
      const data = await api.getNetwork(id);
      setNetwork(data);
      // Compute capacity from CIDR
      setCapacity(computeCapacityFromCIDR(data.cidr));
      // Use aggregated peer_count if present
      setPeerCount(data.peer_count ?? null);
    } catch (error) {
      console.error('Failed to load network:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async () => {
    try {
      await api.deleteNetwork(id);
      navigation.goBack();
    } catch (error) {
      console.error('Failed to delete network:', error);
    }
  };

  if (loading) {
    return (
      <View style={styles.centered}>
        <ActivityIndicator size="large" />
      </View>
    );
  }

  if (!network) {
    return (
      <View style={styles.centered}>
        <Text>Network not found</Text>
      </View>
    );
  }

  return (
    <ScrollView style={styles.container}>
      <Card style={styles.card}>
        <Card.Title title={network.name} />
        <Card.Content>
          <View style={styles.row}>
            <Text style={styles.label}>CIDR:</Text>
            <Text>{network.cidr}</Text>
          </View>
          <View style={styles.row}>
            <Text style={styles.label}>Domain:</Text>
            <Text>{network.domain}</Text>
          </View>
          <View style={styles.row}>
            <Text style={styles.label}>Peers:</Text>
            <Text>
              {peerCount == null ? '…' : peerCount.toLocaleString()} / {capacity == null ? 'N/A' : capacity.toLocaleString()}{' '}
              {peerCount != null && capacity != null ? `(left ${(capacity - peerCount).toLocaleString()})` : ''}
            </Text>
          </View>
          <View style={styles.row}>
            <Text style={styles.label}>Created:</Text>
            <Text>{formatDate(network.created_at)}</Text>
          </View>
          <View style={styles.row}>
            <Text style={styles.label}>Updated:</Text>
            <Text>{formatDate(network.updated_at)}</Text>
          </View>
        </Card.Content>
      </Card>

      <View style={styles.actions}>
        <Button
          mode="contained"
          onPress={() => navigation.navigate('PeerList', { networkId: id })}
          style={styles.button}
        >
          View Peers
        </Button>
      </View>
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
  card: {
    margin: 16,
  },
  row: {
    flexDirection: 'row',
    marginBottom: 8,
  },
  label: {
    fontWeight: 'bold',
    marginRight: 8,
    width: 80,
  },
  actions: {
    padding: 16,
  },
  button: {
    marginBottom: 12,
  },
});
