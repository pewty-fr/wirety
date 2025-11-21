import { useAuth } from '../../contexts/AuthContext';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faUser, faEnvelope, faShield, faNetworkWired, faSignOutAlt } from '@fortawesome/free-solid-svg-icons';
import PageHeader from '../../components/PageHeader';

export default function ProfilePage() {
  const { user, logout, authConfig } = useAuth();

  if (!user) {
    return (
      <div className="p-8">
        <div className="text-center text-gray-500 dark:text-gray-400">Loading...</div>
      </div>
    );
  }

  const isAdmin = user.role === 'administrator';

  return (
    <div className="p-8">
      <PageHeader
        title="Profile"
        subtitle="Manage your account and preferences"
      />

      <div className="max-w-3xl">
        {/* User Info Card */}
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow border border-gray-200 dark:border-gray-700 p-6 mb-6">
          <div className="flex items-start justify-between mb-6">
            <div className="flex items-center gap-4">
              <div className="inline-flex items-center justify-center w-12 h-12 rounded-xl bg-gradient-to-br from-primary-500 to-accent-blue">
                  <FontAwesomeIcon icon={faUser} className="text-lg text-white" />
              </div>
              <div>
                <h2 className="text-2xl font-bold text-gray-900 dark:text-white">{user.name}</h2>
                <p className="text-gray-500 dark:text-gray-400">{user.email}</p>
              </div>
            </div>
            {authConfig?.enabled && (
              <button
                onClick={logout}
                className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
              >
                <FontAwesomeIcon icon={faSignOutAlt} />
                <span>Sign Out</span>
              </button>
            )}
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Email */}
            <div>
              <div className="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">
                <FontAwesomeIcon icon={faEnvelope} />
                <span>Email</span>
              </div>
              <p className="text-gray-900 dark:text-white">{user.email}</p>
            </div>

            {/* Role */}
            <div>
              <div className="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">
                <FontAwesomeIcon icon={faShield} />
                <span>Role</span>
              </div>
              <span
                className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium ${
                  isAdmin
                    ? 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200'
                    : 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
                }`}
              >
                {isAdmin ? 'Administrator' : 'User'}
              </span>
            </div>
          </div>
        </div>

        {/* Authorized Networks Card */}
        {!isAdmin && (
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow border border-gray-200 dark:border-gray-700 p-6">
            <div className="flex items-center gap-2 mb-4">
              <FontAwesomeIcon icon={faNetworkWired} className="text-primary-600 dark:text-primary-400" />
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Authorized Networks</h3>
            </div>
            {user.authorized_networks && user.authorized_networks.length > 0 ? (
              <div className="space-y-2">
                {user.authorized_networks.map((networkId) => (
                  <div
                    key={networkId}
                    className="flex items-center gap-2 px-3 py-2 bg-gray-50 dark:bg-gray-700 rounded-lg"
                  >
                    <FontAwesomeIcon icon={faNetworkWired} className="text-gray-400 dark:text-gray-500" />
                    <span className="text-sm font-mono text-gray-900 dark:text-white">{networkId}</span>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-gray-500 dark:text-gray-400">No networks authorized</p>
            )}
          </div>
        )}

        {isAdmin && (
          <div className="bg-primary-50 dark:bg-primary-900/20 border border-primary-200 dark:border-primary-800 rounded-lg p-4">
            <p className="text-sm text-primary-800 dark:text-primary-200">
              <strong>Administrator Access:</strong> You have full access to all networks and administrative functions.
            </p>
          </div>
        )}

        {/* Authentication Info */}
        {authConfig && (
          <div className="mt-6 bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
            <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Authentication</h4>
            <div className="text-sm text-gray-600 dark:text-gray-400">
              {authConfig.enabled ? (
                <>
                  <p className="mb-1"><strong>Mode:</strong> SSO (OpenID Connect)</p>
                  <p><strong>Provider:</strong> {authConfig.issuer_url}</p>
                </>
              ) : (
                <p><strong>Mode:</strong> No Authentication (Development Mode)</p>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
