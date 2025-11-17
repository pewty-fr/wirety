import React, { useState, useEffect, useCallback } from 'react';
import { View, FlatList, StyleSheet, RefreshControl } from 'react-native';
import { Text, Card, Chip, Button, Searchbar, FAB } from 'react-native-paper';
import { useNavigation, useFocusEffect } from '@react-navigation/native';
import api from '../../services/api';
import { User, isAdministrator } from '../../types/auth';
import { useAuth } from '../../contexts/AuthContext';
import { useDebounce } from '../../hooks/useDebounce';

export const UserListScreen = () => {
  const navigation = useNavigation();
  const { user: currentUser } = useAuth();
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const debouncedSearchQuery = useDebounce(searchQuery, 500);

  const loadUsers = async (isRefreshing = false) => {
    if (isRefreshing) {
      setRefreshing(true);
    } else {
      setLoading(true);
    }
    try {
      const data = await api.listUsers();
      
      // Filter by search query
      let filtered = data;
      if (debouncedSearchQuery) {
        const query = debouncedSearchQuery.toLowerCase();
        filtered = data.filter(
          (user) =>
            user.name.toLowerCase().includes(query) ||
            user.email.toLowerCase().includes(query)
        );
      }
      
      setUsers(filtered);
    } catch (error) {
      console.error('Failed to load users:', error);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useFocusEffect(
    useCallback(() => {
      loadUsers();
    }, [debouncedSearchQuery])
  );

  const renderUser = ({ item }: { item: User }) => (
    <Card
      style={styles.card}
      onPress={() => navigation.navigate('UserView' as never, { userId: item.id } as never)}
    >
      <Card.Title
        title={item.name}
        subtitle={item.email}
        right={() => (
          <Chip
            mode="flat"
            style={[
              styles.roleChip,
              { backgroundColor: item.role === 'administrator' ? '#2196f3' : '#4caf50' },
            ]}
            textStyle={styles.chipText}
          >
            {item.role}
          </Chip>
        )}
      />
      <Card.Content>
        {item.authorized_networks.length > 0 ? (
          <Text style={styles.detail}>
            Access to {item.authorized_networks.length} network(s)
          </Text>
        ) : (
          <Text style={styles.detail}>No network access</Text>
        )}
        <Text style={styles.detail}>
          Last login: {new Date(item.last_login_at).toLocaleDateString()}
        </Text>
      </Card.Content>
    </Card>
  );

  if (!isAdministrator(currentUser)) {
    return (
      <View style={styles.centered}>
        <Text>Access denied. Administrator privileges required.</Text>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <View style={styles.toolbar}>
        <Searchbar
          placeholder="Search users..."
          onChangeText={setSearchQuery}
          value={searchQuery}
          style={styles.searchbar}
        />
        <Button
          mode="outlined"
          onPress={() => navigation.navigate('DefaultPermissions' as never)}
          style={styles.defaultsButton}
          icon="shield-account"
        >
          Default Permissions
        </Button>
      </View>

      <FlatList
        data={users}
        renderItem={renderUser}
        keyExtractor={(item) => item.id}
        contentContainerStyle={styles.list}
        refreshControl={
          <RefreshControl refreshing={refreshing} onRefresh={() => loadUsers(true)} />
        }
        ListEmptyComponent={
          <View style={styles.empty}>
            <Text style={styles.emptyText}>No users found</Text>
            <Text style={styles.emptySubtext}>
              Users are automatically created when they first sign in via SSO
            </Text>
          </View>
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
  centered: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  toolbar: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 16,
    gap: 8,
  },
  searchbar: {
    flex: 1,
  },
  defaultsButton: {
    marginLeft: 8,
  },
  list: {
    padding: 16,
    paddingTop: 0,
  },
  card: {
    marginBottom: 16,
  },
  roleChip: {
    marginRight: 16,
  },
  chipText: {
    color: 'white',
    fontSize: 12,
    textTransform: 'capitalize',
  },
  detail: {
    fontSize: 14,
    color: '#666',
    marginBottom: 4,
  },
  empty: {
    padding: 32,
    alignItems: 'center',
  },
  emptyText: {
    fontSize: 16,
    color: '#666',
    marginBottom: 8,
  },
  emptySubtext: {
    fontSize: 14,
    color: '#999',
    textAlign: 'center',
  },
});
