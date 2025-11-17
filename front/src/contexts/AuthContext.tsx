import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import { User, AuthConfig } from '../types/auth';
import api from '../services/api';

interface AuthContextType {
  user: User | null;
  authConfig: AuthConfig | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  login: (accessToken: string) => Promise<void>;
  logout: () => void;
  refreshUser: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};

interface AuthProviderProps {
  children: ReactNode;
}

export const AuthProvider: React.FC<AuthProviderProps> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null);
  const [authConfig, setAuthConfig] = useState<AuthConfig | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const loadAuthConfig = async () => {
    try {
      const config = await api.getAuthConfig();
      setAuthConfig(config);
      return config;
    } catch (error) {
      console.error('Failed to load auth config:', error);
      // Default to no-auth mode if config fetch fails
      setAuthConfig({ enabled: false, issuer_url: '', client_id: '' });
      return { enabled: false, issuer_url: '', client_id: '' };
    }
  };

  const loadUser = async () => {
    try {
      const userData = await api.getCurrentUser();
      setUser(userData);
    } catch (error) {
      console.error('Failed to load user:', error);
      setUser(null);
    }
  };

  const login = async (accessToken: string) => {
    api.setAccessToken(accessToken);
    localStorage.setItem('access_token', accessToken);
    await loadUser();
  };

  const logout = () => {
    api.setAccessToken(null);
    localStorage.removeItem('access_token');
    setUser(null);
  };

  const refreshUser = async () => {
    await loadUser();
  };

  useEffect(() => {
    const initialize = async () => {
      setIsLoading(true);
      
      const config = await loadAuthConfig();
      
      if (!config.enabled) {
        // No-auth mode: user is automatically authenticated as admin
        setUser({
          id: 'local-admin',
          email: 'admin@local',
          name: 'Administrator',
          role: 'administrator',
          authorized_networks: [],
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
          last_login_at: new Date().toISOString(),
        });
        setIsLoading(false);
        return;
      }

      // OIDC mode: check for existing token
      const storedToken = localStorage.getItem('access_token');
      if (storedToken) {
        api.setAccessToken(storedToken);
        await loadUser();
      }
      
      setIsLoading(false);
    };

    initialize();
  }, []);

  const isAuthenticated = user !== null;

  return (
    <AuthContext.Provider
      value={{
        user,
        authConfig,
        isLoading,
        isAuthenticated,
        login,
        logout,
        refreshUser,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};
