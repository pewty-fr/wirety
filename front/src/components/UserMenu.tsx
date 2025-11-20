import React, { useState } from 'react';
import { View, StyleSheet } from 'react-native';
import { Menu, IconButton, Divider, Text } from 'react-native-paper';
import { useAuth } from '../contexts/AuthContext';
import { useNavigation } from '@react-navigation/native';
import { isAdministrator } from '../types/auth';

export const UserMenu = () => {
  const { user, logout, authConfig } = useAuth();
  const navigation = useNavigation();
  const [visible, setVisible] = useState(false);

  const openMenu = () => setVisible(true);
  const closeMenu = () => setVisible(false);

  if (!user) {
    return null;
  }

  const handleLogout = () => {
    closeMenu();
    logout();
  };

  const handleProfile = () => {
    closeMenu();
    // Deep navigate to the Users tab then into the UserView screen with current user's id.
    // Cast navigation to any to avoid cross-navigator typing friction.
    (navigation as any).navigate('Users', { screen: 'UserView', params: { userId: user.id } });
  };

  const handleUsers = () => {
    closeMenu();
    // Navigate to the Users tab root (UserList).
    (navigation as any).navigate('Users');
  };

  return (
    <Menu
      visible={visible}
      onDismiss={closeMenu}
      anchor={
        <IconButton
          icon="account-circle"
          size={28}
          onPress={openMenu}
        />
      }
      anchorPosition="bottom"
    >
      <View style={styles.userInfo}>
        <Text variant="titleMedium">{user.name}</Text>
        <Text variant="bodySmall" style={styles.email}>{user.email}</Text>
        <Text variant="labelSmall" style={styles.role}>
          {user.role === 'administrator' ? '👑 Administrator' : '👤 User'}
        </Text>
      </View>
      <Divider />
      
      {isAdministrator(user) && (
        <>
          <Menu.Item
            onPress={handleUsers}
            title="Manage Users"
            leadingIcon="account-multiple"
          />
          <Divider />
        </>
      )}
      
      <Menu.Item
        onPress={handleProfile}
        title="Profile"
        leadingIcon="account"
      />
      
      {authConfig?.enabled && (
        <Menu.Item
          onPress={handleLogout}
          title="Sign Out"
          leadingIcon="logout"
        />
      )}
    </Menu>
  );
};

const styles = StyleSheet.create({
  userInfo: {
    padding: 16,
    paddingBottom: 8,
    maxWidth: 250,
  },
  email: {
    color: '#666',
    marginTop: 4,
  },
  role: {
    marginTop: 8,
    color: '#2196f3',
    fontWeight: 'bold',
  },
});
