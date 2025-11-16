import React, { useState, useEffect } from 'react';
import { View, FlatList, StyleSheet } from 'react-native';
import { Text, Card, Searchbar, FAB, Chip } from 'react-native-paper';
import { useNavigation, useRoute } from '@react-navigation/native';
import api from '../../services/api';
import { Peer } from '../../types/api';
import { Pagination } from '../../components/Pagination';

export const PeerListScreen = () => {
  const navigation = useNavigation();
  const route = useRoute();
  const { networkId } = route.params as { networkId: string };
  const [peers, setPeers] = useState<Peer[]>([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [searchQuery, setSearchQuery] = useState('');

  const loadPeers = async () => {
    setLoading(true);
    try {
      const response = await api.getPeers(networkId, page, 20, searchQuery);
      setPeers(response.data);
      setTotalPages(Math.ceil(response.total / response.page_size));
    } catch (error) {
      console.error('Failed to load peers:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadPeers();
  }, [page, searchQuery]);

  const renderPeer = ({ item }: { item: Peer }) => (
    <Card
      style={styles.card}
      onPress={() =>
        navigation.navigate(
          'PeerView' as never,
          { networkId, peerId: item.id, isJump: item.is_jump } as never
        )
      }
    >
      <Card.Title title={item.name} subtitle={item.address} />
      <Card.Content>
        <View style={styles.chips}>
          {item.is_jump && <Chip mode="flat">Jump Server</Chip>}
          {item.is_isolated && <Chip mode="flat">Isolated</Chip>}
          {item.full_encapsulation && <Chip mode="flat">Full Encap</Chip>}
        </View>
      </Card.Content>
    </Card>
  );

  return (
    <View style={styles.container}>
      <Searchbar
        placeholder="Search peers"
        onChangeText={setSearchQuery}
        value={searchQuery}
        style={styles.searchbar}
      />
      <FlatList
        data={peers}
        renderItem={renderPeer}
        keyExtractor={(item) => item.id}
        refreshing={loading}
        onRefresh={loadPeers}
      />
      <Pagination currentPage={page} totalPages={totalPages} onPageChange={setPage} />
      <FAB
        style={styles.fab}
        icon="plus"
        onPress={() =>
          navigation.navigate('PeerAddChoice' as never, { networkId } as never)
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
  },
  card: {
    margin: 8,
    marginHorizontal: 16,
  },
  chips: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
  },
  fab: {
    position: 'absolute',
    margin: 16,
    right: 0,
    bottom: 0,
  },
});
