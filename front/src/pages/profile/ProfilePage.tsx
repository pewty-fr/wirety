import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useAuth } from '../../contexts/AuthContext';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import {
  faUser, faEnvelope, faShield, faNetworkWired, faSignOutAlt,
  faKey, faPlus, faTrash, faCopy, faCheck, faEye, faEyeSlash,
  faExclamationTriangle, faSpinner,
} from '@fortawesome/free-solid-svg-icons';
import PageHeader from '../../components/PageHeader';
import api from '../../api/client';
import type { APIToken } from '../../types';

function formatDate(iso?: string) {
  if (!iso) return '—';
  return new Date(iso).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

function timeAgo(iso?: string) {
  if (!iso) return 'Never';
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 2) return 'Just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

// ---------- Create Token Modal ----------
interface CreateTokenModalProps {
  onClose: () => void;
  onCreated: (token: APIToken) => void;
}

function CreateTokenModal({ onClose, onCreated }: CreateTokenModalProps) {
  const [name, setName] = useState('');
  const [expiresAt, setExpiresAt] = useState('');
  const [error, setError] = useState('');

  const mutation = useMutation({
    mutationFn: () => api.createAPIToken({
      name: name.trim(),
      ...(expiresAt ? { expires_at: new Date(expiresAt).toISOString() } : {}),
    }),
    onSuccess: (token) => onCreated(token),
    onError: () => setError('Failed to create token. Please try again.'),
  });

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      <div className="fixed inset-0 backdrop-blur-sm bg-white/10 dark:bg-black/30" onClick={onClose} />
      <div className="flex items-center justify-center min-h-screen p-4">
        <div className="relative bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-md mx-auto">
          <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
              <FontAwesomeIcon icon={faKey} className="text-primary-500" />
              New API Token
            </h3>
            <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 text-xl">×</button>
          </div>

          <div className="p-6 space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Token name <span className="text-red-500">*</span>
              </label>
              <input
                type="text"
                value={name}
                onChange={e => setName(e.target.value)}
                placeholder="e.g. CI/CD pipeline, MCP server"
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-transparent outline-none"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Expiration <span className="text-gray-400 font-normal">(optional — leave blank for no expiry)</span>
              </label>
              <input
                type="date"
                value={expiresAt}
                min={new Date().toISOString().split('T')[0]}
                onChange={e => setExpiresAt(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-transparent outline-none"
              />
            </div>

            {error && (
              <p className="text-sm text-red-600 dark:text-red-400 flex items-center gap-1">
                <FontAwesomeIcon icon={faExclamationTriangle} />
                {error}
              </p>
            )}
          </div>

          <div className="flex justify-end gap-3 px-6 py-4 border-t border-gray-200 dark:border-gray-700">
            <button
              onClick={onClose}
              className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={() => mutation.mutate()}
              disabled={!name.trim() || mutation.isPending}
              className="flex items-center gap-2 px-4 py-2 text-sm font-medium bg-primary-600 hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg transition-colors"
            >
              {mutation.isPending && <FontAwesomeIcon icon={faSpinner} spin />}
              Generate Token
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

// ---------- Token Created Banner ----------
interface TokenCreatedBannerProps {
  token: APIToken;
  onDismiss: () => void;
}

function TokenCreatedBanner({ token, onDismiss }: TokenCreatedBannerProps) {
  const [copied, setCopied] = useState(false);
  const [visible, setVisible] = useState(false);

  const copy = () => {
    navigator.clipboard.writeText(token.token!);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="bg-green-50 dark:bg-green-900/20 border border-green-300 dark:border-green-700 rounded-lg p-4 space-y-3">
      <div className="flex items-center gap-2 text-green-800 dark:text-green-200 font-medium">
        <FontAwesomeIcon icon={faCheck} />
        Token <strong>{token.name}</strong> created — copy it now, it won't be shown again.
      </div>

      <div className="flex items-center gap-2">
        <div className="flex-1 font-mono text-sm bg-white dark:bg-gray-900 border border-green-300 dark:border-green-700 rounded px-3 py-2 text-gray-900 dark:text-gray-100 overflow-x-auto">
          {visible ? token.token : '••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••'}
        </div>
        <button
          onClick={() => setVisible(v => !v)}
          title={visible ? 'Hide' : 'Show'}
          className="p-2 text-gray-500 hover:text-gray-700 dark:hover:text-gray-300 transition-colors"
        >
          <FontAwesomeIcon icon={visible ? faEyeSlash : faEye} />
        </button>
        <button
          onClick={copy}
          className="flex items-center gap-1.5 px-3 py-2 text-sm font-medium bg-green-600 hover:bg-green-700 text-white rounded-lg transition-colors"
        >
          <FontAwesomeIcon icon={copied ? faCheck : faCopy} />
          {copied ? 'Copied!' : 'Copy'}
        </button>
      </div>

      <button
        onClick={onDismiss}
        className="text-sm text-green-700 dark:text-green-400 hover:underline"
      >
        I've saved my token — dismiss
      </button>
    </div>
  );
}

// ---------- API Tokens Section ----------
function APITokensSection() {
  const queryClient = useQueryClient();
  const [showCreate, setShowCreate] = useState(false);
  const [newToken, setNewToken] = useState<APIToken | null>(null);
  const [revokingId, setRevokingId] = useState<string | null>(null);

  const { data: tokens = [], isLoading } = useQuery({
    queryKey: ['apiTokens'],
    queryFn: () => api.listAPITokens(),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.deleteAPIToken(id),
    onMutate: (id) => setRevokingId(id),
    onSettled: () => {
      setRevokingId(null);
      queryClient.invalidateQueries({ queryKey: ['apiTokens'] });
    },
  });

  const handleCreated = (token: APIToken) => {
    setShowCreate(false);
    setNewToken(token);
    queryClient.invalidateQueries({ queryKey: ['apiTokens'] });
  };

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow border border-gray-200 dark:border-gray-700 p-6">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <FontAwesomeIcon icon={faKey} className="text-primary-600 dark:text-primary-400" />
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white">API Tokens</h3>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-2 px-3 py-1.5 text-sm font-medium bg-primary-600 hover:bg-primary-700 text-white rounded-lg transition-colors"
        >
          <FontAwesomeIcon icon={faPlus} />
          New Token
        </button>
      </div>

      <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
        Tokens carry your permissions and can be used with the{' '}
        <code className="bg-gray-100 dark:bg-gray-700 px-1 rounded text-xs">Authorization: Bearer wirety_…</code> header.
      </p>

      {newToken && (
        <div className="mb-4">
          <TokenCreatedBanner token={newToken} onDismiss={() => setNewToken(null)} />
        </div>
      )}

      {isLoading ? (
        <div className="flex items-center gap-2 text-gray-500 dark:text-gray-400 py-4">
          <FontAwesomeIcon icon={faSpinner} spin />
          <span className="text-sm">Loading tokens…</span>
        </div>
      ) : tokens.length === 0 ? (
        <div className="text-center py-8 text-gray-500 dark:text-gray-400">
          <FontAwesomeIcon icon={faKey} className="text-3xl mb-2 opacity-30" />
          <p className="text-sm">No API tokens yet. Create one to get started.</p>
        </div>
      ) : (
        <div className="space-y-2">
          {tokens.map(token => (
            <div
              key={token.id}
              className="flex items-center justify-between px-4 py-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg border border-gray-200 dark:border-gray-600"
            >
              <div className="min-w-0">
                <p className="text-sm font-medium text-gray-900 dark:text-white truncate">{token.name}</p>
                <div className="flex items-center gap-3 mt-0.5">
                  <span className="text-xs text-gray-500 dark:text-gray-400">
                    Created {formatDate(token.created_at)}
                  </span>
                  {token.expires_at && (
                    <span className={`text-xs ${new Date(token.expires_at) < new Date() ? 'text-red-500' : 'text-gray-500 dark:text-gray-400'}`}>
                      Expires {formatDate(token.expires_at)}
                    </span>
                  )}
                  <span className="text-xs text-gray-500 dark:text-gray-400">
                    Last used: {timeAgo(token.last_used_at)}
                  </span>
                </div>
              </div>
              <button
                onClick={() => deleteMutation.mutate(token.id)}
                disabled={revokingId === token.id}
                title="Revoke token"
                className="ml-4 flex-shrink-0 p-2 text-gray-400 hover:text-red-500 dark:hover:text-red-400 disabled:opacity-50 transition-colors"
              >
                {revokingId === token.id
                  ? <FontAwesomeIcon icon={faSpinner} spin />
                  : <FontAwesomeIcon icon={faTrash} />
                }
              </button>
            </div>
          ))}
        </div>
      )}

      {showCreate && (
        <CreateTokenModal
          onClose={() => setShowCreate(false)}
          onCreated={handleCreated}
        />
      )}
    </div>
  );
}

// ---------- Main Profile Page ----------
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

      <div className="max-w-3xl space-y-6">
        {/* User Info Card */}
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow border border-gray-200 dark:border-gray-700 p-6">
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

        {/* API Tokens */}
        <APITokensSection />

        {/* Authentication Info */}
        {authConfig && (
          <div className="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
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
