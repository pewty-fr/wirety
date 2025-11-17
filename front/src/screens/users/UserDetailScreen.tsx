import React, { useState, useEffect } from 'react';
import { View, ScrollView, StyleSheet } from 'react-native';
import { 
  Text, 
  Card, 
  Button, 
  TextInput, 
  Chip,
  Menu,
  ActivityIndicator,
  Divider,
} from 'react-native-paper';
import api from '../../services/api';
import { User, UserUpdateRequest } from '../../types/auth';
import { Network } from '../../types/api';

export const UserDetailScreen = ({ route, navigation }: any) => {
  const { userId } = route.params;
  const [user, setUser] = useState<User | null>(null);
  const [networks, setNetworks] = useState<Network[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Edit state
  const [isEditing, setIsEditing] = useState(false);
  const [editName, setEditName] = useState('');
  const [editRole, setEditRole] = useState<'administrator' | 'user'>('user');
  const [editNetworks, setEditNetworks] = useState<string[]>([]);
  const [roleMenuVisible, setRoleMenuVisible] = useState(false);
  const [networkMenuVisible, setNetworkMenuVisible] = useState(false);

  useEffect(() => {
    loadData();
  }, [userId]);

  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);
      const [userData, networksResponse] = await Promise.all([
        api.getUser(userId),
        api.getNetworks(),
      ]);
      setUser(userData);
      setNetworks(networksResponse.data);
      
      // Initialize edit state
      setEditName(userData.name);
      setEditRole(userData.role);
      setEditNetworks(userData.authorized_networks);
    } catch (err: any) {
      setError(err.message || 'Failed to load user data');
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async () => {
    if (!user) return;

    try {
      setSaving(true);
      setError(null);

      const updates: UserUpdateRequest = {};
      if (editName !== user.name) updates.name = editName;
      if (editRole !== user.role) updates.role = editRole;
      if (JSON.stringify(editNetworks) !== JSON.stringify(user.authorized_networks)) {
        updates.authorized_networks = editNetworks;
      }

      const updatedUser = await api.updateUser(userId, updates);
      setUser(updatedUser);
      setIsEditing(false);
    } catch (err: any) {
      setError(err.message || 'Failed to update user');
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    if (user) {
      setEditName(user.name);
      setEditRole(user.role);
      setEditNetworks(user.authorized_networks);
    }
    setIsEditing(false);
  };

  const handleDelete = async () => {
    if (!user) return;

    // TODO: Add confirmation dialog
    try {
      setSaving(true);
      await api.deleteUser(userId);
      navigation.goBack();
    } catch (err: any) {
      setError(err.message || 'Failed to delete user');
      setSaving(false);
    }
  };

  const toggleNetwork = (networkId: string) => {
    if (editNetworks.includes(networkId)) {
      setEditNetworks(editNetworks.filter(id => id !== networkId));
    } else {
      setEditNetworks([...editNetworks, networkId]);
    }
  };

  if (loading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" />
      </View>
    );
  }

  if (!user) {
    return (
      <View style={styles.center}>
        <Text>User not found</Text>
      </View>
    );
  }

  return (
    <ScrollView style={styles.container}>
      <Card style={styles.card}>
        <Card.Content>
          <View style={styles.header}>
            <Text variant="headlineSmall">{user.name}</Text>
            <Chip 
              mode="flat"
              style={[
                styles.roleChip,
                user.role === 'administrator' ? styles.adminChip : styles.userChip
              ]}
            >
              {user.role === 'administrator' ? '👑 Administrator' : '👤 User'}
            </Chip>
          </View>

          {error && (
            <Text style={styles.error}>{error}</Text>
          )}

          <Divider style={styles.divider} />

          {/* Basic Information */}
          <Text variant="titleMedium" style={styles.sectionTitle}>Basic Information</Text>
          
          {isEditing ? (
            <TextInput
              label="Name"
              value={editName}
              onChangeText={setEditName}
              style={styles.input}
              mode="outlined"
            />
          ) : (
            <View style={styles.field}>
              <Text variant="bodySmall" style={styles.label}>Name</Text>
              <Text variant="bodyLarge">{user.name}</Text>
            </View>
          )}

          <View style={styles.field}>
            <Text variant="bodySmall" style={styles.label}>Email</Text>
            <Text variant="bodyLarge">{user.email}</Text>
          </View>

          <View style={styles.field}>
            <Text variant="bodySmall" style={styles.label}>User ID</Text>
            <Text variant="bodyMedium" style={styles.mono}>{user.id}</Text>
          </View>

          <Divider style={styles.divider} />

          {/* Role */}
          <Text variant="titleMedium" style={styles.sectionTitle}>Role & Permissions</Text>
          
          {isEditing ? (
            <Menu
              visible={roleMenuVisible}
              onDismiss={() => setRoleMenuVisible(false)}
              anchor={
                <Button 
                  mode="outlined" 
                  onPress={() => setRoleMenuVisible(true)}
                  style={styles.menuButton}
                >
                  {editRole === 'administrator' ? 'Administrator' : 'User'}
                </Button>
              }
            >
              <Menu.Item 
                onPress={() => { 
                  setEditRole('administrator'); 
                  setRoleMenuVisible(false); 
                }} 
                title="Administrator" 
              />
              <Menu.Item 
                onPress={() => { 
                  setEditRole('user'); 
                  setRoleMenuVisible(false); 
                }} 
                title="User" 
              />
            </Menu>
          ) : (
            <View style={styles.field}>
              <Text variant="bodySmall" style={styles.label}>Role</Text>
              <Text variant="bodyLarge">
                {user.role === 'administrator' ? 'Administrator' : 'User'}
              </Text>
            </View>
          )}

          <Divider style={styles.divider} />

          {/* Network Access */}
          <Text variant="titleMedium" style={styles.sectionTitle}>Network Access</Text>
          
          {user.role === 'administrator' ? (
            <Text variant="bodyMedium" style={styles.adminNote}>
              Administrators have access to all networks
            </Text>
          ) : (
            <>
              {isEditing ? (
                <View style={styles.networksList}>
                  {networks.map(network => (
                    <Chip
                      key={network.id}
                      selected={editNetworks.includes(network.id)}
                      onPress={() => toggleNetwork(network.id)}
                      style={styles.networkChip}
                    >
                      {network.name}
                    </Chip>
                  ))}
                  {networks.length === 0 && (
                    <Text variant="bodySmall">No networks available</Text>
                  )}
                </View>
              ) : (
                <View style={styles.networksList}>
                  {user.authorized_networks.length > 0 ? (
                    user.authorized_networks.map(networkId => {
                      const network = networks.find(n => n.id === networkId);
                      return (
                        <Chip key={networkId} style={styles.networkChip}>
                          {network?.name || networkId}
                        </Chip>
                      );
                    })
                  ) : (
                    <Text variant="bodySmall">No network access</Text>
                  )}
                </View>
              )}
            </>
          )}

          <Divider style={styles.divider} />

          {/* Timestamps */}
          <Text variant="titleMedium" style={styles.sectionTitle}>Activity</Text>
          
          <View style={styles.field}>
            <Text variant="bodySmall" style={styles.label}>Created</Text>
            <Text variant="bodyMedium">
              {new Date(user.created_at).toLocaleString()}
            </Text>
          </View>

          <View style={styles.field}>
            <Text variant="bodySmall" style={styles.label}>Last Updated</Text>
            <Text variant="bodyMedium">
              {new Date(user.updated_at).toLocaleString()}
            </Text>
          </View>

          <View style={styles.field}>
            <Text variant="bodySmall" style={styles.label}>Last Login</Text>
            <Text variant="bodyMedium">
              {user.last_login_at 
                ? new Date(user.last_login_at).toLocaleString() 
                : 'Never'}
            </Text>
          </View>
        </Card.Content>

        <Card.Actions style={styles.actions}>
          {isEditing ? (
            <>
              <Button onPress={handleCancel} disabled={saving}>
                Cancel
              </Button>
              <Button 
                mode="contained" 
                onPress={handleSave} 
                loading={saving}
                disabled={saving}
              >
                Save Changes
              </Button>
            </>
          ) : (
            <>
              <Button 
                mode="outlined" 
                onPress={handleDelete}
                textColor="#d32f2f"
                disabled={saving}
              >
                Delete User
              </Button>
              <Button 
                mode="contained" 
                onPress={() => setIsEditing(true)}
                disabled={saving}
              >
                Edit
              </Button>
            </>
          )}
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
  center: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  card: {
    margin: 16,
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 16,
  },
  roleChip: {
    height: 28,
  },
  adminChip: {
    backgroundColor: '#e3f2fd',
  },
  userChip: {
    backgroundColor: '#e8f5e9',
  },
  error: {
    color: '#d32f2f',
    marginBottom: 16,
  },
  divider: {
    marginVertical: 16,
  },
  sectionTitle: {
    marginBottom: 12,
    fontWeight: 'bold',
  },
  field: {
    marginBottom: 16,
  },
  label: {
    color: '#666',
    marginBottom: 4,
  },
  mono: {
    fontFamily: 'monospace',
  },
  input: {
    marginBottom: 16,
  },
  menuButton: {
    marginBottom: 16,
    justifyContent: 'flex-start',
  },
  adminNote: {
    color: '#666',
    fontStyle: 'italic',
  },
  networksList: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
  },
  networkChip: {
    marginRight: 8,
    marginBottom: 8,
  },
  actions: {
    justifyContent: 'flex-end',
    padding: 16,
  },
});
