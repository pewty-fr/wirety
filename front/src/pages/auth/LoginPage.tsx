import { useEffect, useState } from 'react';
import { useAuth } from '../../contexts/AuthContext';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faShield, faEye, faEyeSlash, faTriangleExclamation } from '@fortawesome/free-solid-svg-icons';

export default function LoginPage() {
  const { login, simpleLogin, authConfig, isLoading, oauthError, clearOauthError } = useAuth();
  const [password, setPassword] = useState('');
  const [loginError, setLoginError] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const [showPassword, setShowPassword] = useState(false);
  const sessionExpired = new URLSearchParams(window.location.search).get('session_expired') === '1';

  // Check if we're in the middle of OAuth callback
  const hasAuthCode = new URLSearchParams(window.location.search).has('code');

  useEffect(() => {
    // If already authenticated via session in localStorage, AuthContext handles redirect
  }, [authConfig]);

  const handleSimpleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoginError('');
    setIsSubmitting(true);
    const ok = await simpleLogin(password);
    if (!ok) {
      setLoginError('Invalid password. Please try again.');
      setPassword('');
    }
    setIsSubmitting(false);
  };

  // Show error screen when OAuth callback failed
  if (hasAuthCode && oauthError) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-primary-500 to-accent-blue dark:from-dark dark:to-primary-700 flex items-center justify-center px-4">
        <div className="max-w-md w-full">
          <div className="bg-gradient-to-br from-white to-gray-50 dark:from-dark dark:to-gray-800 rounded-lg shadow-2xl p-8 border-2 border-red-400 dark:border-red-700">
            <div className="text-center mb-6">
              <div className="inline-flex items-center justify-center w-16 h-16 bg-red-100 dark:bg-red-900/30 rounded-full mb-4">
                <FontAwesomeIcon icon={faTriangleExclamation} className="text-3xl text-red-500 dark:text-red-400" />
              </div>
              <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100 mb-2">Sign-in failed</h2>
              <p className="text-sm text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg px-4 py-3 text-left break-words">
                {oauthError}
              </p>
            </div>
            <button
              onClick={() => {
                window.history.replaceState({}, document.title, window.location.pathname);
                clearOauthError();
              }}
              className="w-full btn-brand font-medium py-3 px-4 rounded-lg transition-all"
            >
              Back to login
            </button>
          </div>
        </div>
      </div>
    );
  }

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

          {sessionExpired && (
            <div className="mb-4 px-3 py-2 bg-amber-50 dark:bg-amber-900/20 border border-amber-300 dark:border-amber-700 rounded-lg text-sm text-amber-700 dark:text-amber-400 text-center">
              Your session has expired. Please sign in again.
            </div>
          )}

          {authConfig?.simple_auth ? (
            <form onSubmit={handleSimpleLogin} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Username
                </label>
                <input
                  type="text"
                  value="admin"
                  readOnly
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 cursor-not-allowed"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Password
                </label>
                <div className="relative">
                  <input
                    type={showPassword ? 'text' : 'password'}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="Enter admin password"
                    required
                    autoFocus
                    className="w-full px-3 py-2 pr-10 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-primary-500"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute inset-y-0 right-0 pr-3 flex items-center text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                    tabIndex={-1}
                  >
                    <FontAwesomeIcon icon={showPassword ? faEyeSlash : faEye} className="text-sm" />
                  </button>
                </div>
              </div>
              {loginError && (
                <p className="text-sm text-red-500 dark:text-red-400">{loginError}</p>
              )}
              <button
                type="submit"
                disabled={isSubmitting || !password}
                className="w-full btn-brand font-medium py-3 px-4 rounded-lg transition-all disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {isSubmitting ? 'Signing in...' : 'Sign in'}
              </button>
            </form>
          ) : (
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
          )}
        </div>
      </div>
    </div>
  );
}
