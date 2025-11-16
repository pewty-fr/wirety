import React, { useState, useEffect } from 'react';
import { View, ScrollView, StyleSheet } from 'react-native';
import { Title, ActivityIndicator, Card, Text, Button } from 'react-native-paper';
import { useRoute } from '@react-navigation/native';
import api from '../../services/api';

export const PeerTokenScreen = () => {
  const route = useRoute();
  const { networkId, peerId } = route.params as { networkId: string; peerId: string };
  const [token, setToken] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadToken();
  }, []);

  const loadToken = async () => {
    setLoading(true);
    try {
      const data = await api.getPeerToken(networkId, peerId);
      setToken(data.token);
    } catch (error) {
      console.error('Failed to load token:', error);
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <View style={styles.centered}>
        <ActivityIndicator size="large" />
      </View>
    );
  }

  return (
    <ScrollView style={styles.container}>
      <Card style={styles.card}>
        <Card.Title title="Enrollment Token" />
        <Card.Content>
          <Text style={styles.token}>{token}</Text>
          <Text style={styles.info}>
            Use this token to register the agent on the peer device.
          </Text>
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
  card: {
    margin: 16,
  },
  token: {
    fontFamily: 'monospace',
    fontSize: 12,
    backgroundColor: '#f0f0f0',
    color: '#000',
    padding: 12,
    borderRadius: 4,
    marginBottom: 12,
  },
  info: {
    color: '#666',
    fontSize: 14,
  },
});
