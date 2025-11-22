import { createContext, useContext, useState, useEffect } from 'react';
import type { ReactNode } from 'react';

export interface User {
  id: string;
  email: string;
  name: string;
  role: string;
  authorized_networks: string[];
}

export interface AuthConfig {
  enabled: boolean;
  issuer_url: string;
  client_id: string;
}

interface AuthContextType {
  user: User | null;
  authConfig: AuthConfig | null;
  isLoading: boolean;
  login: () => void;
  logout: () => void;
  isAuthenticated: boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [authConfig, setAuthConfig] = useState<AuthConfig | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Fetch auth config on mount
  useEffect(() => {
    fetchAuthConfig();
  }, []);

  // Check for token and fetch user on mount
  useEffect(() => {
    if (authConfig === null) {
      // Still loading config
      return;
    }

    const token = localStorage.getItem('access_token');
    if (token) {
      fetchCurrentUser(token);
    } else if (!authConfig.enabled) {
      // No-auth mode: create default admin user immediately
      setUser({
        id: 'default-admin',
        email: 'admin@localhost',
        name: 'Administrator',
        role: 'administrator',
        authorized_networks: [],
      });
      setIsLoading(false);
    } else {
      // Auth enabled but no token - show login page
      setIsLoading(false);
    }
  }, [authConfig]);

  // Handle OAuth callback
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const code = params.get('code');
    
    if (code && authConfig && authConfig.enabled) {
      handleOAuthCallback(code);
    }
  }, [authConfig]);

  const API_BASE = (typeof import.meta !== 'undefined' && import.meta.env && import.meta.env.VITE_REACT_APP_API_URL) || (typeof window !== 'undefined' && (window as any).REACT_APP_API_URL) || 'http://localhost:8080/api/v1';
  const fetchAuthConfig = async () => {
    try {
      const response = await fetch(`${API_BASE}/auth/config`);
      if (!response.ok) {
        throw new Error(`Failed to fetch auth config: ${response.status}`);
      }
      const config = await response.json();
      console.log('Auth config loaded:', config);
      setAuthConfig(config);
    } catch (error) {
      console.error('Failed to fetch auth config:', error);
      // Default to no-auth mode if config fetch fails
      console.log('Defaulting to no-auth mode');
      setAuthConfig({ enabled: false, issuer_url: '', client_id: '' });
    }
  };

  const fetchCurrentUser = async (token: string) => {
    setIsLoading(true);
    try {
      const response = await fetch(`${API_BASE}/users/me`, {
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });

      if (response.ok) {
        const userData = await response.json();
        setUser(userData);
        setIsLoading(false);
      } else {
        console.warn('Failed to fetch user, status:', response.status);
        // Token invalid, remove it
        localStorage.removeItem('access_token');
        setUser(null);
        setIsLoading(false);
      }
    } catch (error) {
      console.error('Failed to fetch current user:', error);
      localStorage.removeItem('access_token');
      setUser(null);
      setIsLoading(false);
    }
  };

  const handleOAuthCallback = async (code: string) => {
    try {
      const redirectUri = `${window.location.origin}/`;
      
      const response = await fetch(`${API_BASE}/auth/token`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          code,
          redirect_uri: redirectUri,
        }),
      });

      if (response.ok) {
        const tokenData = await response.json();
        console.log('Token exchange successful');
        localStorage.setItem('access_token', tokenData.access_token);
        
        // Remove code from URL
        window.history.replaceState({}, document.title, window.location.pathname);
        
        // Fetch user data
        console.log('Fetching user data...');
        await fetchCurrentUser(tokenData.access_token);
      } else {
        const errorText = await response.text();
        console.error('Token exchange failed:', response.status, errorText);
        setIsLoading(false);
      }
    } catch (error) {
      console.error('OAuth callback error:', error);
      setIsLoading(false);
    }
  };

  const login = async () => {
    if (!authConfig || !authConfig.enabled) {
      return;
    }

    try {
      // Discover OIDC endpoints
      const discoveryUrl = `${authConfig.issuer_url}/.well-known/openid-configuration`;
      const discoveryResponse = await fetch(discoveryUrl);
      
      if (!discoveryResponse.ok) {
        throw new Error('Failed to fetch OIDC discovery document');
      }

      const discovery = await discoveryResponse.json();
      const authorizationEndpoint = discovery.authorization_endpoint;

      if (!authorizationEndpoint) {
        throw new Error('Authorization endpoint not found in discovery document');
      }

      const redirectUri = `${window.location.origin}/`;
      const authUrl = `${authorizationEndpoint}?` +
        `client_id=${authConfig.client_id}&` +
        `redirect_uri=${encodeURIComponent(redirectUri)}&` +
        `response_type=code&` +
        `scope=openid profile email`;

      window.location.href = authUrl;
    } catch (error) {
      console.error('Failed to initiate login:', error);
    }
  };

  const logout = async () => {
    localStorage.removeItem('access_token');
    setUser(null);

    if (authConfig && authConfig.enabled) {
      try {
        // Discover OIDC endpoints
        const discoveryUrl = `${authConfig.issuer_url}/.well-known/openid-configuration`;
        const discoveryResponse = await fetch(discoveryUrl);
        
        if (discoveryResponse.ok) {
          const discovery = await discoveryResponse.json();
          const endSessionEndpoint = discovery.end_session_endpoint;

          if (endSessionEndpoint) {
            // Redirect to OIDC logout endpoint
            const redirectUri = `${window.location.origin}/`;
            const logoutUrl = `${endSessionEndpoint}?` +
              `post_logout_redirect_uri=${encodeURIComponent(redirectUri)}`;
            window.location.href = logoutUrl;
            return;
          }
        }
      } catch (error) {
        console.error('Failed to discover logout endpoint:', error);
      }
    }
  };

  const isAuthenticated = user !== null;

  return (
    <AuthContext.Provider value={{ user, authConfig, isLoading, login, logout, isAuthenticated }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
