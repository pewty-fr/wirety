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
  simple_auth: boolean;
}

interface AuthContextType {
  user: User | null;
  authConfig: AuthConfig | null;
  isLoading: boolean;
  login: () => void;
  simpleLogin: (password: string) => Promise<boolean>;
  logout: () => void;
  isAuthenticated: boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

const API_BASE = '/api/v1';

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [authConfig, setAuthConfig] = useState<AuthConfig | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Fetch auth config then try to restore session from cookie
  useEffect(() => {
    fetchAuthConfig();
  }, []);

  useEffect(() => {
    if (authConfig === null) return;
    // Try to fetch current user — the browser sends the session cookie automatically
    fetchCurrentUser();
  }, [authConfig]);

  // Handle OAuth callback (code in URL query string)
  useEffect(() => {
    const handleOAuthCallback = async (code: string) => {
      try {
        const redirectUri = `${window.location.origin}/`;
        const response = await fetch(`${API_BASE}/auth/token`, {
          method: 'POST',
          credentials: 'same-origin',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ code, redirect_uri: redirectUri }),
        });

        if (response.ok) {
          // Server sets httpOnly cookie; remove code from URL then fetch user
          window.history.replaceState({}, document.title, window.location.pathname);
          await fetchCurrentUser();
        } else {
          const errorText = await response.text();
          console.error('Session creation failed:', response.status, errorText);
          setIsLoading(false);
        }
      } catch (error) {
        console.error('OAuth callback error:', error);
        setIsLoading(false);
      }
    };

    const params = new URLSearchParams(window.location.search);
    const code = params.get('code');
    if (code && authConfig && authConfig.enabled) {
      handleOAuthCallback(code);
    }
  }, [authConfig]);

  const fetchAuthConfig = async () => {
    try {
      const response = await fetch(`${API_BASE}/auth/config`);
      if (!response.ok) throw new Error(`Failed to fetch auth config: ${response.status}`);
      const config = await response.json();
      setAuthConfig(config);
    } catch (error) {
      console.error('Failed to fetch auth config:', error);
      setAuthConfig({ enabled: false, issuer_url: '', client_id: '', simple_auth: true });
    }
  };

  // Fetch the current user using the session cookie (no explicit token management needed)
  const fetchCurrentUser = async () => {
    setIsLoading(true);
    try {
      const response = await fetch(`${API_BASE}/users/me`, {
        credentials: 'same-origin',
      });

      if (response.ok) {
        const userData = await response.json();
        setUser(userData);
      } else {
        setUser(null);
      }
    } catch (error) {
      console.error('Failed to fetch current user:', error);
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  };

  const login = async () => {
    if (!authConfig || !authConfig.enabled) return;

    try {
      const discoveryUrl = `${authConfig.issuer_url}/.well-known/openid-configuration`;
      const discoveryResponse = await fetch(discoveryUrl);
      if (!discoveryResponse.ok) throw new Error('Failed to fetch OIDC discovery document');

      const discovery = await discoveryResponse.json();
      const authorizationEndpoint = discovery.authorization_endpoint;
      if (!authorizationEndpoint) throw new Error('Authorization endpoint not found');

      const redirectUri = `${window.location.origin}/`;
      const authUrl = `${authorizationEndpoint}?` +
        `client_id=${authConfig.client_id}&` +
        `redirect_uri=${encodeURIComponent(redirectUri)}&` +
        `response_type=code&` +
        `scope=openid profile email offline_access`;

      window.location.href = authUrl;
    } catch (error) {
      console.error('Failed to initiate login:', error);
    }
  };

  const simpleLogin = async (password: string): Promise<boolean> => {
    try {
      const response = await fetch(`${API_BASE}/auth/login`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: 'admin', password }),
      });

      if (!response.ok) return false;

      // Server sets the httpOnly cookie; just re-fetch the user
      await fetchCurrentUser();
      return true;
    } catch (error) {
      console.error('Simple login error:', error);
      return false;
    }
  };

  const logout = async () => {
    try {
      // Server clears the cookie and invalidates the session
      await fetch(`${API_BASE}/auth/logout`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({}),
      });
    } catch (error) {
      console.error('Failed to invalidate session:', error);
    }

    setUser(null);

    if (authConfig && authConfig.enabled) {
      try {
        const discoveryUrl = `${authConfig.issuer_url}/.well-known/openid-configuration`;
        const discoveryResponse = await fetch(discoveryUrl);
        if (discoveryResponse.ok) {
          const discovery = await discoveryResponse.json();
          const endSessionEndpoint = discovery.end_session_endpoint;
          if (endSessionEndpoint) {
            const redirectUri = `${window.location.origin}/`;
            window.location.href = `${endSessionEndpoint}?post_logout_redirect_uri=${encodeURIComponent(redirectUri)}`;
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
    <AuthContext.Provider value={{ user, authConfig, isLoading, login, simpleLogin, logout, isAuthenticated }}>
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
