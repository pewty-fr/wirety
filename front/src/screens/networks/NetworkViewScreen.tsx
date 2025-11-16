import React, { useState, useEffect } from 'react';
import { View, ScrollView, StyleSheet } from 'react-native';
import { Title, Card, Text, Button, ActivityIndicator } from 'react-native-paper';
import { useNavigation, useRoute } from '@react-navigation/native';
import api from '../../services/api';
import { Network } from '../../types/api';
import { formatDate } from '../../utils/validation';

export const NetworkViewScreen = () => {
  const navigation = useNavigation();
  const route = useRoute();
  const { id } = route.params as { id: string };
  const [network, setNetwork] = useState<Network | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadNetwork();
  }, [id]);

  const loadNetwork = async () => {
    setLoading(true);
    try {
      const data = await api.getNetwork(id);
      setNetwork(data);
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
          onPress={() => navigation.navigate('NetworkUpdate' as never, { id } as never)}
          style={styles.button}
        >
          Edit Network
        </Button>
        <Button
          mode="contained"
          onPress={() => navigation.navigate('PeerList' as never, { networkId: id } as never)}
          style={styles.button}
        >
          View Peers
        </Button>
        <Button
          mode="outlined"
          onPress={handleDelete}
          style={styles.button}
        >
          Delete Network
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
