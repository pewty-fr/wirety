import React, { useState, useEffect, useCallback } from 'react';
import { View, FlatList, StyleSheet } from 'react-native';
import { Text, Card, Searchbar, FAB } from 'react-native-paper';
import { useNavigation, useFocusEffect } from '@react-navigation/native';
import api from '../../services/api';
import { Network } from '../../types/api';
import { Pagination } from '../../components/Pagination';
import { useDebounce } from '../../hooks/useDebounce';

export const NetworkListScreen = () => {
  const navigation = useNavigation();
  const [networks, setNetworks] = useState<Network[]>([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [searchQuery, setSearchQuery] = useState('');
  const debouncedSearchQuery = useDebounce(searchQuery, 500);

  const loadNetworks = async () => {
    setLoading(true);
    try {
      const response = await api.getNetworks(page, 20, debouncedSearchQuery);
      setNetworks(response.data);
      setTotalPages(Math.ceil(response.total / response.page_size));
    } catch (error) {
      console.error('Failed to load networks:', error);
    } finally {
      setLoading(false);
    }
  };

  useFocusEffect(
    useCallback(() => {
      loadNetworks();
    }, [page, debouncedSearchQuery])
  );

  const renderNetwork = ({ item }: { item: Network }) => (
    <Card
      style={styles.card}
      onPress={() => navigation.navigate('NetworkView' as never, { id: item.id } as never)}
    >
      <Card.Title title={item.name} subtitle={item.cidr} />
      <Card.Content>
        <Text>Domain: {item.domain}</Text>
      </Card.Content>
    </Card>
  );

  return (
    <View style={styles.container}>
      <Searchbar
        placeholder="Search networks"
        onChangeText={setSearchQuery}
        value={searchQuery}
        style={styles.searchbar}
      />
      <FlatList
        data={networks}
        renderItem={renderNetwork}
        keyExtractor={(item) => item.id}
        refreshing={loading}
        onRefresh={loadNetworks}
      />
      <Pagination
        currentPage={page}
        totalPages={totalPages}
        onPageChange={setPage}
      />
      <FAB
        style={styles.fab}
        icon="plus"
        onPress={() => navigation.navigate('NetworkAdd' as never)}
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
  fab: {
    position: 'absolute',
    margin: 16,
    right: 0,
    bottom: 0,
  },
});
