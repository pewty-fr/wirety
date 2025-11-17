export interface User {
  id: string;
  email: string;
  name: string;
  role: 'administrator' | 'user';
  authorized_networks: string[];
  created_at: string;
  updated_at: string;
  last_login_at: string;
}

// Helper functions for User type
export const isAdministrator = (user: User | null): boolean => {
  return user?.role === 'administrator';
};

export const hasNetworkAccess = (user: User | null, networkId: string): boolean => {
  if (!user) return false;
  if (isAdministrator(user)) return true;
  return user.authorized_networks.includes(networkId);
};

export interface AuthConfig {
  enabled: boolean;
  issuer_url: string;
  client_id: string;
}

export interface UserUpdateRequest {
  name?: string;
  role?: 'administrator' | 'user';
  authorized_networks?: string[];
}

export interface DefaultNetworkPermissions {
  default_role: 'administrator' | 'user';
  default_authorized_networks: string[];
}
