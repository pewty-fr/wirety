import React from 'react';
import { View, StyleSheet } from 'react-native';
import { Title, Button, Text } from 'react-native-paper';
import { useNavigation, useRoute } from '@react-navigation/native';

export const PeerAddChoiceScreen = () => {
  const navigation = useNavigation();
  const route = useRoute();
  const { networkId } = route.params as { networkId: string };

  return (
    <View style={styles.container}>
      <Title style={styles.title}>Add Peer</Title>
      <Text style={styles.subtitle}>Choose the type of peer to add:</Text>
      
      <Button
        mode="contained"
        onPress={() =>
          navigation.navigate('PeerAddRegular' as never, { networkId } as never)
        }
        style={styles.button}
        icon="laptop"
      >
        Regular Peer
      </Button>
      <Text style={styles.description}>
        A regular peer that connects through jump servers
      </Text>

      <Button
        mode="contained"
        onPress={() =>
          navigation.navigate('PeerAddJump' as never, { networkId } as never)
        }
        style={styles.button}
        icon="server"
      >
        Jump Server
      </Button>
      <Text style={styles.description}>
        A hub peer that routes traffic for other peers
      </Text>

      <Button
        mode="outlined"
        onPress={() => navigation.goBack()}
        style={styles.button}
      >
        Cancel
      </Button>
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    padding: 16,
    backgroundColor: '#fff',
  },
  title: {
    marginBottom: 8,
  },
  subtitle: {
    marginBottom: 24,
    color: '#666',
  },
  button: {
    marginBottom: 8,
  },
  description: {
    marginBottom: 24,
    paddingHorizontal: 16,
    color: '#888',
    fontSize: 12,
  },
});
