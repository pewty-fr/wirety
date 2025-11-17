import React, { useState, useEffect, useCallback } from 'react';
import { View, FlatList, StyleSheet } from 'react-native';
import { Text, Card, Searchbar, Chip } from 'react-native-paper';
import { useFocusEffect } from '@react-navigation/native';
import api from '../../services/api';
import { IPAMAllocation } from '../../types/api';
import { Pagination } from '../../components/Pagination';
import { useDebounce } from '../../hooks/useDebounce';

export const IPAMListScreen = () => {
  const [allocations, setAllocations] = useState<IPAMAllocation[]>([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [searchQuery, setSearchQuery] = useState('');
  const debouncedSearchQuery = useDebounce(searchQuery, 500);

  const loadAllocations = async () => {
    setLoading(true);
    try {
      const response = await api.getIPAM(page, 20, debouncedSearchQuery);
      setAllocations(response.data);
      setTotalPages(Math.ceil(response.total / response.page_size));
    } catch (error) {
      console.error('Failed to load IPAM:', error);
    } finally {
      setLoading(false);
    }
  };

  useFocusEffect(
    useCallback(() => {
      loadAllocations();
    }, [page, debouncedSearchQuery])
  );

  const renderAllocation = ({ item }: { item: IPAMAllocation }) => (
    <Card style={styles.card}>
      <Card.Content>
        <View style={styles.row}>
          <Text style={styles.label}>Network:</Text>
          <Text>{item.network_name} ({item.network_cidr})</Text>
        </View>
        <View style={styles.row}>
          <Text style={styles.label}>IP:</Text>
          <Text style={styles.ip}>{item.ip}</Text>
        </View>
        {item.peer_name && (
          <View style={styles.row}>
            <Text style={styles.label}>Peer:</Text>
            <Text>{item.peer_name}</Text>
          </View>
        )}
        <View style={styles.chips}>
          <Chip mode="flat">{item.allocated ? 'Allocated' : 'Available'}</Chip>
        </View>
      </Card.Content>
    </Card>
  );

  return (
    <View style={styles.container}>
      <Searchbar
        placeholder="Search IP or network"
        onChangeText={setSearchQuery}
        value={searchQuery}
        style={styles.searchbar}
      />
      <FlatList
        data={allocations}
        renderItem={renderAllocation}
        keyExtractor={(item, index) => `${item.network_id}-${item.ip}-${index}`}
        refreshing={loading}
        onRefresh={loadAllocations}
      />
      <Pagination currentPage={page} totalPages={totalPages} onPageChange={setPage} />
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
  row: {
    flexDirection: 'row',
    marginBottom: 8,
  },
  label: {
    fontWeight: 'bold',
    marginRight: 8,
    width: 80,
  },
  ip: {
    fontFamily: 'monospace',
  },
  chips: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
    marginTop: 8,
  },
});
