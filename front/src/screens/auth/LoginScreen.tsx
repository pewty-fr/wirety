import React, { useEffect, useState } from 'react';
import { View, StyleSheet, Platform } from 'react-native';
import { ActivityIndicator, Text, Button } from 'react-native-paper';
import { useAuth } from '../../contexts/AuthContext';
import api from '../../services/api';

interface OIDCDiscovery {
  issuer: string;
  authorization_endpoint: string;
  token_endpoint: string;
  userinfo_endpoint: string;
  jwks_uri: string;
}

export const LoginScreen = () => {
  const { authConfig, login } = useAuth();
  const [error, setError] = useState<string | null>(null);
  const [isProcessing, setIsProcessing] = useState(false);
  const [discovery, setDiscovery] = useState<OIDCDiscovery | null>(null);

  useEffect(() => {
    // Fetch OIDC discovery document
    const fetchDiscovery = async () => {
      if (authConfig?.enabled && authConfig.issuer_url) {
        try {
          const discoveryUrl = `${authConfig.issuer_url.replace(/\/$/, '')}/.well-known/openid-configuration`;
          const response = await fetch(discoveryUrl);
          const data = await response.json();
          setDiscovery(data);
        } catch (err) {
          console.error('Failed to fetch OIDC discovery:', err);
          setError('Failed to load authentication configuration');
        }
      }
    };

    fetchDiscovery();

    // Check if we're returning from OIDC redirect
    if (Platform.OS === 'web' && authConfig?.enabled) {
      const params = new URLSearchParams(window.location.search);
      const code = params.get('code');
      const state = params.get('state');
      const storedState = sessionStorage.getItem('oidc_state');

      if (code && state) {
        // Validate state parameter
        if (state !== storedState) {
          setError('Invalid state parameter - possible CSRF attack');
          window.history.replaceState({}, document.title, window.location.pathname);
          return;
        }

        handleOIDCCallback(code);
      }
    }
  }, [authConfig]);

  const handleOIDCCallback = async (code: string) => {
    setIsProcessing(true);
    setError(null);

    try {
      const redirectUri = `${window.location.origin}/`;
      
      // Exchange authorization code for access token
      const tokenResponse = await api.exchangeToken(code, redirectUri);
      
      // Store token and authenticate
      await login(tokenResponse.access_token);
      
      // Clear URL parameters and session storage
      window.history.replaceState({}, document.title, window.location.pathname);
      sessionStorage.removeItem('oidc_state');
      sessionStorage.removeItem('oidc_nonce');
    } catch (error) {
      console.error('Failed to handle OIDC callback:', error);
      setError('Authentication failed. Please try again.');
      setIsProcessing(false);
      
      // Clear URL parameters
      window.history.replaceState({}, document.title, window.location.pathname);
    }
  };

  const handleLogin = () => {
    if (!authConfig?.enabled || !discovery) {
      setError('Authentication not properly configured');
      return;
    }

    const redirectUri = `${window.location.origin}/`;
    const state = Math.random().toString(36).substring(7);
    const nonce = Math.random().toString(36).substring(7);

    // Store state for validation
    sessionStorage.setItem('oidc_state', state);
    sessionStorage.setItem('oidc_nonce', nonce);

    // Build OIDC authorization URL using discovered endpoint
    const authUrl = new URL(discovery.authorization_endpoint);
    authUrl.searchParams.set('client_id', authConfig.client_id);
    authUrl.searchParams.set('redirect_uri', redirectUri);
    authUrl.searchParams.set('response_type', 'code');
    authUrl.searchParams.set('scope', 'openid profile email');
    authUrl.searchParams.set('state', state);
    authUrl.searchParams.set('nonce', nonce);

    // Redirect to OIDC provider
    window.location.href = authUrl.toString();
  };

  if (!authConfig) {
    return (
      <View style={styles.container}>
        <ActivityIndicator size="large" />
        <Text style={styles.text}>Loading...</Text>
      </View>
    );
  }

  if (authConfig.enabled && !discovery) {
    return (
      <View style={styles.container}>
        <ActivityIndicator size="large" />
        <Text style={styles.text}>Loading authentication...</Text>
      </View>
    );
  }

  if (isProcessing) {
    return (
      <View style={styles.container}>
        <ActivityIndicator size="large" />
        <Text style={styles.text}>Authenticating...</Text>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <View style={styles.content}>
        <Text variant="headlineMedium" style={styles.title}>
          Wirety
        </Text>
        <Text variant="bodyLarge" style={styles.subtitle}>
          WireGuard Network Management
        </Text>
        
        {error && (
          <Text style={styles.error}>{error}</Text>
        )}
        
        <Button
          mode="contained"
          onPress={handleLogin}
          style={styles.button}
          icon="login"
        >
          Sign In with SSO
        </Button>
      </View>
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#f5f5f5',
  },
  content: {
    alignItems: 'center',
    padding: 32,
    maxWidth: 400,
  },
  title: {
    marginBottom: 8,
    fontWeight: 'bold',
  },
  subtitle: {
    marginBottom: 48,
    color: '#666',
  },
  button: {
    marginTop: 16,
    minWidth: 200,
  },
  text: {
    marginTop: 16,
  },
  error: {
    color: '#d32f2f',
    marginBottom: 16,
    textAlign: 'center',
  },
});
