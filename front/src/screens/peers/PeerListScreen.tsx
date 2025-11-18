import React, { useState, useEffect, useCallback } from 'react';
import { View, FlatList, StyleSheet } from 'react-native';
import { Text, Card, Searchbar, FAB, Chip, Menu, Button } from 'react-native-paper';
import { useNavigation, useRoute, useFocusEffect } from '@react-navigation/native';
import api from '../../services/api';
import { Peer, Network } from '../../types/api';
import { PeerSessionStatus } from '../../types/security';
import { Pagination } from '../../components/Pagination';
import { useDebounce } from '../../hooks/useDebounce';

export const PeerListScreen = () => {
  const navigation = useNavigation();
  const route = useRoute();
  const params = route.params as { networkId?: string } | undefined;
  const networkId = params?.networkId;
  const [peers, setPeers] = useState<Peer[]>([]);
  const [allPeers, setAllPeers] = useState<Peer[]>([]);
  const [networks, setNetworks] = useState<Network[]>([]);
  const [sessionStatuses, setSessionStatuses] = useState<Map<string, PeerSessionStatus>>(new Map());
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [searchQuery, setSearchQuery] = useState('');
  const debouncedSearchQuery = useDebounce(searchQuery, 500);
  const [filterNetworkId, setFilterNetworkId] = useState<string | null>(null);
  const [menuVisible, setMenuVisible] = useState(false);

  const loadPeers = async () => {
    setLoading(true);
    try {
      let loadedPeers: Peer[] = [];
      let loadedNetworks: Network[] = [];
      
      if (networkId) {
        // Load peers for specific network
        const networkResponse = await api.getNetwork(networkId);
        const response = await api.getPeers(networkId, 1, 1000, '');
        loadedPeers = response.data.map(peer => ({ 
          ...peer, 
          network_id: networkId,
          network_name: networkResponse.name 
        }));
        loadedNetworks = [networkResponse];
      } else {
        // Load all networks first, then load peers for each
        const networksResponse = await api.getNetworks(1, 1000, '');
        loadedNetworks = networksResponse.data;
        
        // Load peers for each network in parallel
        const peersPromises = loadedNetworks.map(network => 
          api.getPeers(network.id, 1, 1000, '').then(response => ({
            networkId: network.id,
            networkName: network.name,
            peers: response.data
          })).catch(err => {
            console.error(`Failed to load peers for network ${network.id}:`, err);
            return { networkId: network.id, networkName: network.name, peers: [] };
          })
        );
        
        const peersResponses = await Promise.all(peersPromises);
        loadedPeers = peersResponses.flatMap(response => 
          response.peers.map(peer => ({ 
            ...peer, 
            network_id: response.networkId,
            network_name: response.networkName 
          }))
        );
      }
      
      setNetworks(loadedNetworks);
      setAllPeers(loadedPeers);
      
      // Load session statuses for all peers
      loadSessionStatuses(loadedPeers);
      
      // Apply network filter
      let filtered = loadedPeers;
      if (filterNetworkId) {
        filtered = filtered.filter(p => p.network_id === filterNetworkId);
      }
      
      // Apply search filter
      if (searchQuery) {
        const query = searchQuery.toLowerCase();
        filtered = filtered.filter(p => 
          p.name.toLowerCase().includes(query) ||
          p.address.toLowerCase().includes(query) ||
          p.id.toLowerCase().includes(query) ||
          (p.network_name && p.network_name.toLowerCase().includes(query))
        );
      }
      
      // Apply local pagination
      const total = filtered.length;
      const start = (page - 1) * 20;
      const end = start + 20;
      const paginated = filtered.slice(start, end);
      
      setPeers(paginated);
      setTotalPages(Math.ceil(total / 20));
    } catch (error) {
      console.error('Failed to load peers:', error);
    } finally {
      setLoading(false);
    }
  };

  const loadSessionStatuses = async (peerList: Peer[]) => {
    const statusMap = new Map<string, PeerSessionStatus>();
    
    // Load session statuses in parallel
    const statusPromises = peerList.map(async (peer) => {
      if (peer.network_id) {
        try {
          const status = await api.getPeerSessionStatus(peer.network_id, peer.id);
          return { peerId: peer.id, status };
        } catch (error) {
          // Session status might not be available for all peers
          return null;
        }
      }
      return null;
    });
    
    const results = await Promise.all(statusPromises);
    results.forEach(result => {
      if (result) {
        statusMap.set(result.peerId, result.status);
      }
    });
    
    setSessionStatuses(statusMap);
  };

  // Re-apply filters when filter changes without reloading from server
  const applyFilters = useCallback(() => {
    let filtered = allPeers;
    
    // Apply network filter
    if (filterNetworkId) {
      filtered = filtered.filter(p => p.network_id === filterNetworkId);
    }
    
    // Apply search filter
    if (debouncedSearchQuery) {
      const query = debouncedSearchQuery.toLowerCase();
      filtered = filtered.filter(p => 
        p.name.toLowerCase().includes(query) ||
        p.address.toLowerCase().includes(query) ||
        p.id.toLowerCase().includes(query) ||
        (p.network_name && p.network_name.toLowerCase().includes(query))
      );
    }
    
    // Apply local pagination
    const total = filtered.length;
    const start = (page - 1) * 20;
    const end = start + 20;
    const paginated = filtered.slice(start, end);
    
    setPeers(paginated);
    setTotalPages(Math.ceil(total / 20));
  }, [allPeers, filterNetworkId, debouncedSearchQuery, page]);

  useEffect(() => {
    if (allPeers.length > 0) {
      applyFilters();
    }
  }, [filterNetworkId, debouncedSearchQuery, page, applyFilters]);

  useFocusEffect(
    useCallback(() => {
      loadPeers();
    }, [networkId])
  );

  const renderPeer = ({ item }: { item: Peer }) => {
    const sessionStatus = sessionStatuses.get(item.id);
    
    return (
      <Card
        style={styles.card}
        onPress={() =>
          navigation.navigate(
            'PeerView' as never,
            { networkId: item.network_id || networkId, peerId: item.id, isJump: item.is_jump } as never
          )
        }
      >
        <Card.Title 
          title={item.name} 
          subtitle={`${item.address}${item.network_name ? ` • ${item.network_name}` : ''}`} 
        />
        <Card.Content>
          <View style={styles.chips}>
            {item.is_jump && <Chip mode="flat">Jump Server</Chip>}
            {item.is_isolated && <Chip mode="flat">Isolated</Chip>}
            {item.full_encapsulation && <Chip mode="flat">Full Encapsulation</Chip>}
            {sessionStatus?.has_active_agent && (
              <Chip mode="flat" icon="check-circle" style={{ backgroundColor: '#4caf50' }} textStyle={{ color: 'white' }}>
                Connected
              </Chip>
            )}
            {sessionStatus?.suspicious_activity && (
              <Chip mode="flat" icon="alert" style={{ backgroundColor: '#ff5722' }} textStyle={{ color: 'white' }}>
                Alert
              </Chip>
            )}
            {sessionStatus?.conflicting_sessions && sessionStatus.conflicting_sessions.length > 0 && (
              <Chip mode="flat" icon="alert-circle" style={{ backgroundColor: '#f44336' }} textStyle={{ color: 'white' }}>
                Conflict
              </Chip>
            )}
          </View>
        </Card.Content>
      </Card>
    );
  };

  const getFilterLabel = () => {
    if (!filterNetworkId) return 'All Networks';
    const network = networks.find(n => n.id === filterNetworkId);
    return network ? network.name : 'Filter by Network';
  };

  return (
    <View style={styles.container}>
      <View style={styles.searchContainer}>
        <Searchbar
          placeholder="Search peers"
          onChangeText={setSearchQuery}
          value={searchQuery}
          style={styles.searchbar}
        />
        {!networkId && networks.length > 0 && (
          <Menu
            visible={menuVisible}
            onDismiss={() => setMenuVisible(false)}
            anchor={
              <Button 
                mode="outlined" 
                onPress={() => setMenuVisible(true)}
                style={styles.filterButton}
              >
                {getFilterLabel()}
              </Button>
            }
          >
            <Menu.Item 
              onPress={() => {
                setFilterNetworkId(null);
                setMenuVisible(false);
                setPage(1);
              }} 
              title="All Networks" 
            />
            {networks.map(network => (
              <Menu.Item 
                key={network.id}
                onPress={() => {
                  setFilterNetworkId(network.id);
                  setMenuVisible(false);
                  setPage(1);
                }} 
                title={network.name} 
              />
            ))}
          </Menu>
        )}
      </View>
      <FlatList
        data={peers}
        renderItem={renderPeer}
        keyExtractor={(item) => item.id}
        refreshing={loading}
        onRefresh={loadPeers}
      />
      <Pagination currentPage={page} totalPages={totalPages} onPageChange={setPage} />
      {(networkId || filterNetworkId) && (
        <FAB
          style={styles.fab}
          icon="plus"
          onPress={() =>
            navigation.navigate('PeerAddChoice' as never, { networkId: networkId || filterNetworkId } as never)
          }
        />
      )}
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f5f5f5',
  },
  searchContainer: {
    padding: 16,
    paddingBottom: 8,
  },
  searchbar: {
    marginBottom: 8,
  },
  filterButton: {
    marginTop: 8,
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
