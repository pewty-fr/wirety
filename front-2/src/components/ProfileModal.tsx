import { useAuth } from '../contexts/AuthContext';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faUser, faEnvelope, faShield, faNetworkWired, faSignOutAlt, faTimes } from '@fortawesome/free-solid-svg-icons';

interface ProfileModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export default function ProfileModal({ isOpen, onClose }: ProfileModalProps) {
  const { user, logout, authConfig } = useAuth();

  if (!isOpen) return null;

  const isAdmin = user?.role === 'administrator';

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      {/* Backdrop */}
      <div 
        className="fixed inset-0 backdrop-blur-sm bg-white/10 dark:bg-black/30 transition-all"
        onClick={onClose}
      />
      
      {/* Modal */}
      <div className="flex items-center justify-center min-h-screen p-4">
        <div className="relative bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-2xl w-full mx-auto">
          {/* Header */}
          <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h2 className="text-xl font-bold text-gray-900 dark:text-white">Profile</h2>
            <button
              onClick={onClose}
              className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
            >
              <FontAwesomeIcon icon={faTimes} className="text-xl" />
            </button>
          </div>

          {/* Content */}
          <div className="p-6 space-y-6">
            {!user ? (
              <div className="text-center text-gray-500 dark:text-gray-400">Loading...</div>
            ) : (
              <>
                {/* User Info Card */}
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-4">
                    <div className="w-16 h-16 bg-primary-100 dark:bg-primary-900 rounded-full flex items-center justify-center">
                      <FontAwesomeIcon icon={faUser} className="text-2xl text-primary-600 dark:text-primary-400" />
                    </div>
                    <div>
                      <h3 className="text-xl font-bold text-gray-900 dark:text-white">{user.name}</h3>
                      <p className="text-gray-500 dark:text-gray-400">{user.email}</p>
                    </div>
                  </div>
                  {authConfig?.enabled && (
                    <button
                      onClick={() => {
                        logout();
                        onClose();
                      }}
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

                {/* Authorized Networks Card */}
                {!isAdmin && (
                  <div className="border-t border-gray-200 dark:border-gray-700 pt-6">
                    <div className="flex items-center gap-2 mb-4">
                      <FontAwesomeIcon icon={faNetworkWired} className="text-primary-600 dark:text-primary-400" />
                      <h4 className="text-lg font-semibold text-gray-900 dark:text-white">Authorized Networks</h4>
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
                  <div className="bg-gray-50 dark:bg-gray-900 rounded-lg p-4 border-t border-gray-200 dark:border-gray-700">
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
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
