import React, { useState, useEffect } from 'react';
import { View, ScrollView, StyleSheet, Alert, Clipboard } from 'react-native';
import { Title, ActivityIndicator, Card, Text, Button, IconButton } from 'react-native-paper';
import { useRoute } from '@react-navigation/native';
import api from '../../services/api';

export const PeerConfigScreen = () => {
  const route = useRoute();
  const { networkId, peerId } = route.params as { networkId: string; peerId: string };
  const [config, setConfig] = useState('');
  const [loading, setLoading] = useState(true);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    loadConfig();
  }, []);

  const loadConfig = async () => {
    setLoading(true);
    try {
      const data = await api.getPeerConfig(networkId, peerId);
      setConfig(data.config);
    } catch (error) {
      console.error('Failed to load config:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCopy = () => {
    try {
      Clipboard.setString(config);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (error) {
      console.error('Failed to copy:', error);
      Alert.alert('Error', 'Failed to copy to clipboard');
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
        <Card.Title 
          title="WireGuard Configuration"
          right={(props) => (
            <IconButton
              {...props}
              icon={copied ? 'check' : 'content-copy'}
              onPress={handleCopy}
            />
          )}
        />
        <Card.Content>
          <Text style={styles.config}>{config}</Text>
          <Text style={styles.info}>
            Save this configuration to /etc/wireguard/wg0.conf on the peer device.
          </Text>
        </Card.Content>
        <Card.Actions>
          <Button 
            mode="contained" 
            onPress={handleCopy}
            icon={copied ? 'check' : 'content-copy'}
          >
            {copied ? 'Copied!' : 'Copy to Clipboard'}
          </Button>
        </Card.Actions>
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
  config: {
    fontFamily: 'monospace',
    fontSize: 10,
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
