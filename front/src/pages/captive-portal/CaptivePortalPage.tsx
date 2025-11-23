import { useEffect, useState } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { useAuth } from '../../contexts/AuthContext';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faWifi, faShieldAlt, faCheckCircle, faExclamationTriangle } from '@fortawesome/free-solid-svg-icons';

export default function CaptivePortalPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { login, user, authConfig, isAuthenticated } = useAuth();
  const [status, setStatus] = useState<'initial' | 'authenticating' | 'success' | 'error'>('initial');
  const [errorMessage, setErrorMessage] = useState('');

  const captiveToken = searchParams.get('token'); // Temporary captive portal token (NOT agent token)
  const redirectUrl = searchParams.get('redirect');

  useEffect(() => {
    // If user is already authenticated, proceed with captive portal authentication
    if (isAuthenticated && user && captiveToken) {
      authenticateCaptivePortal();
    }
  }, [isAuthenticated, user, captiveToken]);

  const authenticateCaptivePortal = async () => {
    if (!user || !captiveToken) return;

    setStatus('authenticating');

    try {
      // Get the user's session hash (access token)
      const sessionHash = localStorage.getItem('session_hash');
      if (!sessionHash) {
        throw new Error('No session found');
      }

      // Get client IP from a simple API call
      // In production, the server will extract the real IP from the request
      const peerIP = await getClientIP();

      // Send authentication request to server
      const response = await fetch('/api/v1/captive-portal/authenticate', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          captive_token: captiveToken,
          user_token: sessionHash,
          peer_ip: peerIP,
        }),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Authentication failed');
      }

      setStatus('success');

      // Redirect after a short delay
      setTimeout(() => {
        if (redirectUrl) {
          window.location.href = redirectUrl;
        } else {
          window.location.href = 'http://example.com';
        }
      }, 2000);
    } catch (error) {
      console.error('Captive portal authentication error:', error);
      setStatus('error');
      setErrorMessage(error instanceof Error ? error.message : 'Authentication failed');
    }
  };

  const getClientIP = async (): Promise<string> => {
    // In a real scenario, the server would extract the IP from the request
    // For now, we'll use a placeholder
    try {
      const response = await fetch('https://api.ipify.org?format=json');
      const data = await response.json();
      return data.ip;
    } catch {
      // Fallback - server will use the actual client IP
      return '0.0.0.0';
    }
  };

  const handleLogin = () => {
    // Store the current URL to return after login
    sessionStorage.setItem('captive_portal_return', window.location.href);
    login();
  };

  if (!captiveToken) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-primary-500 to-accent-blue dark:from-dark dark:to-primary-700 flex items-center justify-center px-4">
        <div className="max-w-md w-full bg-white dark:bg-gray-800 rounded-lg shadow-2xl p-8">
          <div className="text-center">
            <FontAwesomeIcon icon={faExclamationTriangle} className="text-6xl text-yellow-500 mb-4" />
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-4">Invalid Request</h1>
            <p className="text-gray-600 dark:text-gray-400">
              Missing authentication token. Please try accessing the network again.
            </p>
          </div>
        </div>
      </div>
    );
  }

  if (status === 'success') {
    return (
      <div className="min-h-screen bg-gradient-to-br from-green-500 to-green-600 flex items-center justify-center px-4">
        <div className="max-w-md w-full bg-white dark:bg-gray-800 rounded-lg shadow-2xl p-8">
          <div className="text-center">
            <FontAwesomeIcon icon={faCheckCircle} className="text-6xl text-green-500 mb-4" />
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-4">Access Granted!</h1>
            <p className="text-gray-600 dark:text-gray-400 mb-4">
              You now have internet access through this network.
            </p>
            <p className="text-sm text-gray-500 dark:text-gray-500">
              Redirecting you to your destination...
            </p>
          </div>
        </div>
      </div>
    );
  }

  if (status === 'error') {
    return (
      <div className="min-h-screen bg-gradient-to-br from-red-500 to-red-600 flex items-center justify-center px-4">
        <div className="max-w-md w-full bg-white dark:bg-gray-800 rounded-lg shadow-2xl p-8">
          <div className="text-center">
            <FontAwesomeIcon icon={faExclamationTriangle} className="text-6xl text-red-500 mb-4" />
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-4">Authentication Failed</h1>
            <p className="text-gray-600 dark:text-gray-400 mb-4">
              {errorMessage}
            </p>
            <button
              onClick={() => navigate('/dashboard')}
              className="btn-brand"
            >
              Go to Dashboard
            </button>
          </div>
        </div>
      </div>
    );
  }

  if (status === 'authenticating') {
    return (
      <div className="min-h-screen bg-gradient-to-br from-primary-500 to-accent-blue dark:from-dark dark:to-primary-700 flex items-center justify-center px-4">
        <div className="max-w-md w-full bg-white dark:bg-gray-800 rounded-lg shadow-2xl p-8">
          <div className="text-center">
            <div className="animate-spin rounded-full h-16 w-16 border-b-2 border-primary-600 mx-auto mb-4"></div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-4">Authenticating...</h1>
            <p className="text-gray-600 dark:text-gray-400">
              Please wait while we verify your access.
            </p>
          </div>
        </div>
      </div>
    );
  }

  // Initial state - need to authenticate
  return (
    <div className="min-h-screen bg-gradient-to-br from-primary-500 to-accent-blue dark:from-dark dark:to-primary-700 flex items-center justify-center px-4">
      <div className="max-w-md w-full bg-white dark:bg-gray-800 rounded-lg shadow-2xl p-8">
        <div className="text-center mb-6">
          <div className="inline-flex items-center justify-center w-20 h-20 bg-gradient-to-br from-primary-500 to-accent-blue rounded-full mb-4 shadow-lg">
            <FontAwesomeIcon icon={faWifi} className="text-4xl text-white" />
          </div>
          <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-2">Network Access Required</h1>
          <p className="text-gray-600 dark:text-gray-400">
            Authenticate to access the internet through this network
          </p>
        </div>

        <div className="bg-gray-50 dark:bg-gray-900 rounded-lg p-6 mb-6">
          <div className="flex items-start gap-3 mb-4">
            <FontAwesomeIcon icon={faShieldAlt} className="text-primary-600 dark:text-primary-400 mt-1" />
            <div>
              <h3 className="font-semibold text-gray-900 dark:text-white mb-1">Secure Authentication</h3>
              <p className="text-sm text-gray-600 dark:text-gray-400">
                This network requires authentication through your organization's identity provider.
              </p>
            </div>
          </div>

          <div className="flex items-start gap-3">
            <FontAwesomeIcon icon={faWifi} className="text-primary-600 dark:text-primary-400 mt-1" />
            <div>
              <h3 className="font-semibold text-gray-900 dark:text-white mb-1">Internet Access</h3>
              <p className="text-sm text-gray-600 dark:text-gray-400">
                Once authenticated, you'll have full internet access through the jump server.
              </p>
            </div>
          </div>
        </div>

        {!isAuthenticated ? (
          <button
            onClick={handleLogin}
            className="w-full btn-brand font-medium py-3 px-4 rounded-lg transition-all"
          >
            Sign In to Continue
          </button>
        ) : (
          <div className="text-center">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600 mx-auto mb-4"></div>
            <p className="text-gray-600 dark:text-gray-400">Verifying access...</p>
          </div>
        )}

        <p className="text-xs text-center text-gray-500 dark:text-gray-500 mt-6">
          Powered by Wirety
        </p>
      </div>
    </div>
  );
}
