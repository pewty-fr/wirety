import { useEffect } from 'react';
import { useAuth } from '../../contexts/AuthContext';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faShield } from '@fortawesome/free-solid-svg-icons';

export default function LoginPage() {
  const { login, authConfig, isLoading } = useAuth();

  // Check if we're in the middle of OAuth callback
  const hasAuthCode = new URLSearchParams(window.location.search).has('code');

  useEffect(() => {
    // If auth is not enabled (no-auth mode), redirect to dashboard
    if (authConfig && !authConfig.enabled) {
      window.location.href = '/dashboard';
    }
  }, [authConfig]);

  // Show loading state during OAuth callback processing
  if (isLoading || hasAuthCode) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-900 flex items-center justify-center">
        <div className="text-center">
          <div className="text-gray-500 dark:text-gray-400 mb-2">
            {hasAuthCode ? 'Completing sign in...' : 'Loading...'}
          </div>
          <div className="text-sm text-gray-400 dark:text-gray-500">
            Please wait
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900 flex items-center justify-center px-4">
      <div className="max-w-md w-full">
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow-lg p-8">
          <div className="text-center mb-8">
            <div className="inline-flex items-center justify-center w-20 h-20 bg-primary-100 dark:bg-primary-900 rounded-full mb-4">
              <FontAwesomeIcon icon={faShield} className="text-4xl text-primary-600 dark:text-primary-400" />
            </div>
            <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-2">Wirety</h1>
            <p className="text-gray-500 dark:text-gray-400">WireGuard Network Management</p>
          </div>

          <div className="space-y-4">
            <button
              onClick={login}
              className="w-full bg-primary-600 hover:bg-primary-700 text-white font-medium py-3 px-4 rounded-lg transition-colors"
            >
              Sign in with SSO
            </button>
            <p className="text-sm text-center text-gray-500 dark:text-gray-400">
              You will be redirected to your organization's login page
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
