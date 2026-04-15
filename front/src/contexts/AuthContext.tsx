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
  authorization_endpoint: string;
  end_session_endpoint: string;
  scope: string;
}

interface AuthContextType {
  user: User | null;
  authConfig: AuthConfig | null;
  isLoading: boolean;
  oauthError: string | null;
  clearOauthError: () => void;
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
  const [oauthError, setOauthError] = useState<string | null>(null);

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
          // Server sets httpOnly cookie — no need to handle the response body
          window.history.replaceState({}, document.title, window.location.pathname);
          await fetchCurrentUser();

          // Resume a captive portal flow that was interrupted by the OIDC redirect.
          // CaptivePortalPage stores its URL in sessionStorage before calling login()
          // so we can return here after the OAuth callback completes.
          const pendingCaptivePortal = sessionStorage.getItem('captive_portal_return');
          if (pendingCaptivePortal) {
            sessionStorage.removeItem('captive_portal_return');
            window.location.href = pendingCaptivePortal;
            return;
          }
        } else {
          const body = await response.text();
          let message = `Authentication failed (HTTP ${response.status})`;
          try {
            const json = JSON.parse(body);
            if (json.error) message = json.error;
          } catch {
            if (body) message = body;
          }
          console.error('Session creation failed:', response.status, body);
          setOauthError(message);
          setIsLoading(false);
        }
      } catch (error) {
        console.error('OAuth callback error:', error);
        setOauthError(error instanceof Error ? error.message : 'Unexpected error during sign-in');
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
      setAuthConfig({ enabled: false, issuer_url: '', client_id: '', simple_auth: true, authorization_endpoint: '', end_session_endpoint: '', scope: '' });
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

    const authorizationEndpoint = authConfig.authorization_endpoint;
    if (!authorizationEndpoint) {
      console.error('Authorization endpoint not available');
      return;
    }

    const redirectUri = `${window.location.origin}/`;
    const authUrl = `${authorizationEndpoint}?` +
      `client_id=${authConfig.client_id}&` +
      `redirect_uri=${encodeURIComponent(redirectUri)}&` +
      `response_type=code&` +
      `scope=${encodeURIComponent(authConfig.scope)}`;

    window.location.href = authUrl;
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

      // Server sets the httpOnly cookie — no need to handle the response body
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

    if (authConfig && authConfig.enabled && authConfig.end_session_endpoint) {
      const redirectUri = `${window.location.origin}/`;
      window.location.href = `${authConfig.end_session_endpoint}?post_logout_redirect_uri=${encodeURIComponent(redirectUri)}`;
      return;
    }
  };

  const isAuthenticated = user !== null;

  const clearOauthError = () => setOauthError(null);

  return (
    <AuthContext.Provider value={{ user, authConfig, isLoading, oauthError, clearOauthError, login, simpleLogin, logout, isAuthenticated }}>
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
