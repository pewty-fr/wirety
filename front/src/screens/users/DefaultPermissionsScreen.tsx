import React, { useState, useEffect } from 'react';
import { View, ScrollView, StyleSheet } from 'react-native';
import { 
  Text, 
  Card, 
  Button, 
  Menu,
  ActivityIndicator,
  Divider,
  Chip,
} from 'react-native-paper';
import api from '../../services/api';
import { DefaultNetworkPermissions } from '../../types/auth';
import { Network } from '../../types/api';
import { useAuth } from '../../contexts/AuthContext';
import { isAdministrator } from '../../types/auth';

export const DefaultPermissionsScreen = ({ navigation }: any) => {
  const { user: currentUser } = useAuth();
  const [permissions, setPermissions] = useState<DefaultNetworkPermissions | null>(null);
  const [networks, setNetworks] = useState<Network[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Edit state
  const [isEditing, setIsEditing] = useState(false);
  const [editRole, setEditRole] = useState<'administrator' | 'user'>('user');
  const [editNetworks, setEditNetworks] = useState<string[]>([]);
  const [roleMenuVisible, setRoleMenuVisible] = useState(false);

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);
      const [permsData, networksResponse] = await Promise.all([
        api.getDefaultPermissions(),
        api.getNetworks(),
      ]);
      setPermissions(permsData);
      setNetworks(networksResponse.data);
      
      // Initialize edit state
      setEditRole(permsData.default_role);
      setEditNetworks(permsData.default_authorized_networks);
    } catch (err: any) {
      setError(err.message || 'Failed to load default permissions');
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async () => {
    if (!permissions) return;

    try {
      setSaving(true);
      setError(null);

      const updates: DefaultNetworkPermissions = {
        default_role: editRole,
        default_authorized_networks: editNetworks,
      };

      const updatedPerms = await api.updateDefaultPermissions(updates);
      setPermissions(updatedPerms);
      setIsEditing(false);
    } catch (err: any) {
      setError(err.message || 'Failed to update default permissions');
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    if (permissions) {
      setEditRole(permissions.default_role);
      setEditNetworks(permissions.default_authorized_networks);
    }
    setIsEditing(false);
  };

  const toggleNetwork = (networkId: string) => {
    if (editNetworks.includes(networkId)) {
      setEditNetworks(editNetworks.filter(id => id !== networkId));
    } else {
      setEditNetworks([...editNetworks, networkId]);
    }
  };

  if (!isAdministrator(currentUser)) {
    return (
      <View style={styles.centered}>
        <Text>Access denied. Administrator privileges required.</Text>
      </View>
    );
  }

  if (loading) {
    return (
      <View style={styles.centered}>
        <ActivityIndicator size="large" />
      </View>
    );
  }

  if (!permissions) {
    return (
      <View style={styles.centered}>
        <Text>Failed to load default permissions</Text>
      </View>
    );
  }

  return (
    <ScrollView style={styles.container}>
      <Card style={styles.card}>
        <Card.Content>
          <Text variant="headlineSmall">Default User Permissions</Text>
          <Text variant="bodyMedium" style={styles.description}>
            These settings apply to users when they first sign in via SSO. Existing users will not be affected.
          </Text>

          {error && (
            <Text style={styles.error}>{error}</Text>
          )}

          <Divider style={styles.divider} />

          {/* Default Role */}
          <Text variant="titleMedium" style={styles.sectionTitle}>Default Role</Text>
          
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
              <Chip 
                mode="flat"
                style={[
                  styles.roleChip,
                  permissions.default_role === 'administrator' ? styles.adminChip : styles.userChip
                ]}
              >
                {permissions.default_role === 'administrator' ? '👑 Administrator' : '👤 User'}
              </Chip>
            </View>
          )}

          <Divider style={styles.divider} />

          {/* Default Network Access */}
          <Text variant="titleMedium" style={styles.sectionTitle}>Default Network Access</Text>
          
          {permissions.default_role === 'administrator' ? (
            <Text variant="bodyMedium" style={styles.adminNote}>
              Administrators have access to all networks by default
            </Text>
          ) : (
            <>
              <Text variant="bodySmall" style={styles.hint}>
                Select which networks new users can access by default
              </Text>
              
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
                  {permissions.default_authorized_networks.length > 0 ? (
                    permissions.default_authorized_networks.map(networkId => {
                      const network = networks.find(n => n.id === networkId);
                      return (
                        <Chip key={networkId} style={styles.networkChip}>
                          {network?.name || networkId}
                        </Chip>
                      );
                    })
                  ) : (
                    <Text variant="bodySmall">No default network access</Text>
                  )}
                </View>
              )}
            </>
          )}
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
            <Button 
              mode="contained" 
              onPress={() => setIsEditing(true)}
              disabled={saving}
            >
              Edit Defaults
            </Button>
          )}
        </Card.Actions>
      </Card>

      <Card style={styles.infoCard}>
        <Card.Content>
          <Text variant="titleMedium" style={styles.sectionTitle}>About Default Permissions</Text>
          <Text variant="bodySmall" style={styles.infoText}>
            • Users are automatically created when they first sign in via SSO
          </Text>
          <Text variant="bodySmall" style={styles.infoText}>
            • Default role determines the permission level for new users
          </Text>
          <Text variant="bodySmall" style={styles.infoText}>
            • Default networks specify which networks new users can access
          </Text>
          <Text variant="bodySmall" style={styles.infoText}>
            • Changes only affect users created after the update
          </Text>
          <Text variant="bodySmall" style={styles.infoText}>
            • You can modify individual user permissions in the user management screen
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
  infoCard: {
    margin: 16,
    marginTop: 0,
    backgroundColor: '#e3f2fd',
  },
  description: {
    color: '#666',
    marginTop: 8,
  },
  error: {
    color: '#d32f2f',
    marginTop: 16,
  },
  divider: {
    marginVertical: 16,
  },
  sectionTitle: {
    marginBottom: 12,
    fontWeight: 'bold',
  },
  field: {
    marginBottom: 8,
  },
  roleChip: {
    alignSelf: 'flex-start',
  },
  adminChip: {
    backgroundColor: '#e3f2fd',
  },
  userChip: {
    backgroundColor: '#e8f5e9',
  },
  menuButton: {
    marginBottom: 8,
    justifyContent: 'flex-start',
  },
  adminNote: {
    color: '#666',
    fontStyle: 'italic',
  },
  hint: {
    color: '#666',
    marginBottom: 12,
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
  infoText: {
    marginBottom: 8,
    color: '#1976d2',
  },
});
