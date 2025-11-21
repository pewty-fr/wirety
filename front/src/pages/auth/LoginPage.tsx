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
      <div className="min-h-screen bg-gradient-to-br from-primary-500 to-accent-blue dark:from-dark dark:to-primary-700 flex items-center justify-center">
        <div className="text-center">
          <div className="text-white mb-2">
            {hasAuthCode ? 'Completing sign in...' : 'Loading...'}
          </div>
          <div className="text-sm text-gray-200 dark:text-gray-400">
            Please wait
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-primary-500 to-accent-blue dark:from-dark dark:to-primary-700 flex items-center justify-center px-4">
      <div className="max-w-md w-full">
        <div className="bg-gradient-to-br from-white to-gray-50 dark:from-dark dark:to-gray-800 rounded-lg shadow-2xl p-8 border-2 border-primary-300 dark:border-primary-700">
          <div className="text-center mb-8">
            <div className="inline-flex items-center justify-center w-20 h-20 bg-gradient-to-br from-primary-500 to-accent-blue rounded-full mb-4 shadow-lg">
              <FontAwesomeIcon icon={faShield} className="text-4xl text-white" />
            </div>
            <h1 className="text-3xl font-bold text-brand-gradient mb-2">Wirety</h1>
            <p className="text-gray-600 dark:text-gray-400">WireGuard Network Management</p>
          </div>

          <div className="space-y-4">
            <button
              onClick={login}
              className="w-full btn-brand font-medium py-3 px-4 rounded-lg transition-all"
            >
              Sign in with SSO
            </button>
            <p className="text-sm text-center text-gray-600 dark:text-gray-400">
              You will be redirected to your organization's login page
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
