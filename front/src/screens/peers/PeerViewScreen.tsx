import React, { useState, useEffect } from 'react';
import { View, ScrollView, StyleSheet, Alert } from 'react-native';
import { Title, Card, Text, Button, ActivityIndicator, Chip } from 'react-native-paper';
import { useNavigation, useRoute } from '@react-navigation/native';
import api from '../../services/api';
import { Peer } from '../../types/api';
import { formatDate } from '../../utils/validation';

export const PeerViewScreen = () => {
  const navigation = useNavigation();
  const route = useRoute();
  const { networkId, peerId } = route.params as { networkId: string; peerId: string };
  const [peer, setPeer] = useState<Peer | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadPeer();
  }, [peerId]);

  const loadPeer = async () => {
    setLoading(true);
    try {
      const data = await api.getPeer(networkId, peerId);
      setPeer(data);
    } catch (error) {
      console.error('Failed to load peer:', error);
    } finally {
      setLoading(false);
    }
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
            {peer.is_isolated && <Chip mode="flat">Isolated</Chip>}
            {peer.full_encapsulation && <Chip mode="flat">Full Encapsulation</Chip>}
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

      <View style={styles.actions}>
        <Button
          mode="contained"
          onPress={() =>
            navigation.navigate(
              peer.is_jump ? ('PeerUpdateJump' as never) : ('PeerUpdateRegular' as never),
              { networkId, peerId } as never
            )
          }
          style={styles.button}
        >
          Edit Peer
        </Button>
        <Button
          mode="contained"
          onPress={() =>
            navigation.navigate('PeerToken' as never, { networkId, peerId } as never)
          }
          style={styles.button}
        >
          View Token
        </Button>
        <Button
          mode="contained"
          onPress={() =>
            navigation.navigate('PeerConfig' as never, { networkId, peerId } as never)
          }
          style={styles.button}
        >
          View Config
        </Button>
        <Button mode="outlined" onPress={handleDelete} style={styles.button}>
          Delete Peer
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
  actions: {
    padding: 16,
  },
  button: {
    marginBottom: 12,
  },
});
