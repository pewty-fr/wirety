import { useState } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faServer, faLaptop, faRocket, faCopy } from '@fortawesome/free-solid-svg-icons';
import Modal from './Modal';
import JumpPeerModal from './JumpPeerModal';
import RegularPeerModal from './RegularPeerModal';
import { usePeer, useNetwork } from '../hooks/useQueries';
import { useAuth } from '../contexts/AuthContext';
import api from '../api/client';
import type { Peer, User } from '../types';

interface PeerDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  peer: Peer | null;
  onUpdate: () => void;
  users?: User[];
}

export default function PeerDetailModal({ isOpen, onClose, peer, onUpdate, users = [] }: PeerDetailModalProps) {
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [configLoading, setConfigLoading] = useState(false);
  const [configText, setConfigText] = useState<string | null>(null);
  const [configError, setConfigError] = useState<string | null>(null);
  const [configCopied, setConfigCopied] = useState(false);
  const [tokenCopied, setTokenCopied] = useState(false);
  const { user } = useAuth();

  // Get owner name from users list
  const getOwnerName = (ownerId: string | undefined) => {
    if (!ownerId) return null;
    if (user && user?.id == ownerId) return user.name;
    const owner = users.find(u => u.id === ownerId);
    return owner?.name || ownerId;
  };

  // Use React Query to fetch peer details
  const { data: currentPeer, refetch: refetchPeer } = usePeer(
    peer?.network_id || '',
    peer?.id || '',
    isOpen // poll only when modal open
  );

  const { data: network } = useNetwork(
    peer?.network_id || '',
    isOpen && !!peer?.network_id
  );

  const handleClose = () => {
    // Reset state before closing
    setIsEditModalOpen(false);
    setConfigText(null);
    setConfigError(null);
    setConfigCopied(false);
    setTokenCopied(false);
    onClose();
  };

  if (!peer) return null;
  const displayPeer = currentPeer || peer;

  // Check if current user can edit this peer
  const canEdit = user?.role === 'administrator' || displayPeer.owner_id === user?.id;

  const handleDelete = async () => {
    if (!confirm(`Are you sure you want to delete peer "${displayPeer.name}"? This action cannot be undone.`)) {
      return;
    }
    
    setDeleting(true);
    try {
      await api.deletePeer(displayPeer.network_id!, displayPeer.id);
      onUpdate();
      onClose();
    } catch (error) {
      const err = error as { response?: { data?: { error?: string } } };
      alert(err.response?.data?.error || 'Failed to delete peer');
    } finally {
      setDeleting(false);
    }
  };

  const handleEdit = () => {
    setIsEditModalOpen(true);
  };

  const handleEditSuccess = () => {
    setIsEditModalOpen(false);
    refetchPeer(); // Reload peer details after edit
    onUpdate();
  };

  return (
    <>
      <Modal
        isOpen={isOpen}
        onClose={handleClose}
        title="Peer Details"
        size="lg"
      >
        <div className="space-y-6">
          {/* Header Info */}
          <div className="flex items-start justify-between">
            <div className="flex items-start gap-4">
              <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue">
                <FontAwesomeIcon icon={displayPeer.is_jump ? faRocket : (displayPeer.use_agent ? faServer : faLaptop)} className="text-2xl text-white" />
              </div>
              <div>
                <h3 className="text-2xl font-bold text-gray-900 dark:text-white">{displayPeer.name}</h3>
                <p className="text-sm text-gray-600 dark:text-gray-300 mt-1">ID: {displayPeer.id}</p>
              </div>
            </div>
          </div>

          {/* Network Info */}
          <div>
            <p className="text-lg text-gray-900 dark:text-gray-100">{displayPeer.network_name || displayPeer.network_id}</p>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-600 dark:text-gray-300 mb-1">IP Address</label>
              <p className="text-lg font-mono text-gray-900 dark:text-gray-100">{displayPeer.address}</p>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-600 dark:text-gray-300 mb-1">Domain</label>
              <p className="text-lg font-mono text-gray-900 dark:text-gray-100">{displayPeer.name}.{network?.name || displayPeer.network_name || 'network'}.local</p>
            </div>
            {/* Owner */}
            {displayPeer.owner_id && (
            <div>
              <label className="block text-sm font-medium text-gray-600 dark:text-gray-300 mb-1">Owner</label>
              <p className="text-sm text-gray-900 dark:text-gray-100">{getOwnerName(displayPeer.owner_id)}</p>
            </div>
            )}
          </div>

          {/* Status Information */}
          <div className="bg-gradient-to-br from-gray-50 to-primary-50 dark:from-gray-800 dark:to-gray-700 rounded-lg p-4">
            <h4 className="text-sm font-medium text-gray-700 dark:text-gray-100 mb-3">Status Information</h4>
            <div className="space-y-3">
              {/* Agent Status */}
              <div className="flex items-center justify-between">
                <span className="text-sm text-gray-600 dark:text-gray-300">Agent Status</span>
                <span className={`inline-flex items-center px-3 py-1 rounded-full text-xs font-medium ${
                  displayPeer.use_agent
                    ? (displayPeer.session_status?.has_active_agent ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200')
                    : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'
                }`}>
                  <span className={`w-2 h-2 rounded-full mr-1.5 ${
                    displayPeer.use_agent
                      ? (displayPeer.session_status?.has_active_agent ? 'bg-green-500' : 'bg-red-500')
                      : 'bg-gray-400'
                  }`}></span>
                  {displayPeer.use_agent
                    ? (displayPeer.session_status?.has_active_agent ? 'Agent On' : 'Agent Off')
                    : 'No Agent'}
                </span>
              </div>

              {/* Reported Endpoint */}
              {displayPeer.session_status?.current_session?.reported_endpoint && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-gray-600 dark:text-gray-300">Reported Endpoint</span>
                  <span className="text-sm font-mono text-gray-900 dark:text-gray-100">
                    {displayPeer.session_status.current_session.reported_endpoint}
                  </span>
                </div>
              )}

              {/* Hostname */}
              {displayPeer.session_status?.current_session?.hostname && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-gray-600 dark:text-gray-300">Hostname</span>
                  <span className="text-sm font-mono text-gray-900 dark:text-gray-100">
                    {displayPeer.session_status.current_session.hostname}
                  </span>
                </div>
              )}

              {/* Last Seen */}
              {displayPeer.session_status?.current_session?.last_seen && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-gray-600 dark:text-gray-300">Last Seen</span>
                  <span className="text-sm text-gray-900 dark:text-gray-100">
                    {new Date(displayPeer.session_status.current_session.last_seen).toLocaleString()}
                  </span>
                </div>
              )}

              {/* Last Checked */}
              {displayPeer.session_status?.last_checked && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-gray-600 dark:text-gray-300">Last Status Check</span>
                  <span className="text-sm text-gray-900 dark:text-gray-100">
                    {new Date(displayPeer.session_status.last_checked).toLocaleString()}
                  </span>
                </div>
              )}
            </div>
          </div>

          {/* Public Key intentionally hidden per request */}

          {/* WireGuard Configuration actions for non-agent peers (no inline display) */}
          {!displayPeer.use_agent && (
            <div className="space-y-2">
              <h4 className="text-sm font-medium text-gray-700 dark:text-gray-200">WireGuard Configuration</h4>
              <div className="flex gap-2">
                <button
                  disabled={configLoading || configCopied}
                  onClick={async () => {
                    if (!peer.network_id) return;
                    setConfigLoading(true);
                    setConfigError(null);
                    try {
                      if (!configText) {
                        const cfg = await api.getPeerConfig(peer.network_id, peer.id);
                        setConfigText(cfg);
                      }
                      await navigator.clipboard.writeText(configText!);
                      setConfigCopied(true);
                      setTimeout(() => setConfigCopied(false), 3000);
                    } catch (e) {
                      const error = e as { message?: string };
                      setConfigError(error?.message || 'Failed to copy config');
                    } finally {
                      setConfigLoading(false);
                    }
                  }}
                  className={`px-4 py-2 text-sm font-semibold rounded-lg disabled:opacity-50 transition-all ${configCopied ? 'bg-green-600 text-white' : 'bg-gradient-to-r from-primary-600 to-accent-blue text-white hover:scale-105 active:scale-95'}`}
                >
                  {configLoading ? 'Copying...' : configCopied ? 'Copied ✓' : 'Copy Config'}
                </button>
                <button
                  disabled={configLoading}
                  onClick={async () => {
                    if (!peer.network_id) return;
                    setConfigError(null);
                    try {
                      if (!configText) {
                        setConfigLoading(true);
                        const cfg = await api.getPeerConfig(peer.network_id, peer.id);
                        setConfigText(cfg);
                        setConfigLoading(false);
                      }
                      if (configText) {
                        const blob = new Blob([configText], { type: 'text/plain' });
                        const url = URL.createObjectURL(blob);
                        const a = document.createElement('a');
                        a.href = url;
                        
                        a.download = `${peer.network_name}.conf`;
                        document.body.appendChild(a);
                        a.click();
                        document.body.removeChild(a);
                        URL.revokeObjectURL(url);
                      }
                    } catch (e) {
                      const error = e as { message?: string };
                      setConfigError(error?.message || 'Failed to download config');
                    } finally {
                      setConfigLoading(false);
                    }
                  }}
                  className="px-4 py-2 text-sm font-semibold bg-gradient-to-r from-green-600 to-accent-green text-white rounded-lg hover:scale-105 active:scale-95 disabled:opacity-50 transition-all"
                >
                  {configLoading && !configCopied && !configText ? 'Fetching...' : 'Download .conf'}
                </button>
              </div>
              {configError && <p className="text-xs text-red-600">{configError}</p>}
            </div>
          )}

          {/* Jump Server Specific */}
          {displayPeer.is_jump && (
            <div className="bg-gradient-to-br from-gray-50 to-primary-50 dark:from-gray-800 dark:to-gray-700 rounded-lg p-4">
              <h4 className="text-sm font-medium text-gray-700 dark:text-gray-100 mb-3">Peer Configuration</h4>
              <div className="space-y-2">
                <div>
                  <label className="block text-xs font-medium text-gray-600 dark:text-gray-300 mb-1">Listen Port</label>
                  <p className="text-sm text-gray-900 dark:text-gray-100">{displayPeer.listen_port || 'N/A'}</p>
                </div>
                <div>
                  <label className="block text-xs font-medium text-gray-600 dark:text-gray-300 mb-1">Endpoint</label>
                  <p className="text-sm text-gray-900 dark:text-gray-100">{displayPeer.endpoint || 'N/A'}</p>
                </div>
              </div>
            </div>
          )}

          {/* Regular Peer Specific */}
          {!displayPeer.is_jump && (
            <div className="bg-gradient-to-br from-gray-50 to-primary-50 dark:from-gray-800 dark:to-gray-700 rounded-lg p-4">
              <h4 className="text-sm font-medium text-gray-700 dark:text-gray-100 mb-3">Peer Configuration</h4>
              <div className="space-y-2">
                <div className="flex justify-between">
                  <span className="text-sm text-gray-600 dark:text-gray-300">Isolated</span>
                  <span className="text-sm font-medium text-gray-900 dark:text-gray-100">{displayPeer.is_isolated ? 'Yes' : 'No'}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-sm text-gray-600 dark:text-gray-300">Full Encapsulation</span>
                  <span className="text-sm font-medium text-gray-900 dark:text-gray-100">{displayPeer.full_encapsulation ? 'Yes' : 'No'}</span>
                </div>
              </div>
            </div>
          )}

          {/* Additional Allowed IPs */}
          {displayPeer.additional_allowed_ips && displayPeer.additional_allowed_ips.length > 0 && (
            <div>
              <label className="block text-sm font-medium text-gray-600 dark:text-gray-300 mb-2">Additional Allowed IPs</label>
              <div className="space-y-1">
                {displayPeer.additional_allowed_ips.map((ip, index) => (
                  <div key={index} className="bg-gradient-to-br from-gray-50 to-primary-50 dark:from-gray-800 dark:to-gray-700 px-3 py-2 rounded text-sm font-mono text-gray-900 dark:text-gray-100">
                    {ip}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Agent Token (if uses agent) */}
          {displayPeer.use_agent && displayPeer.token && (
            <div>
              <div className="flex items-center justify-between">
                <label className="block text-sm font-medium text-gray-600 dark:text-gray-300">Agent Token</label>
                <button
                  onClick={async () => {
                    try {
                      await navigator.clipboard.writeText(displayPeer.token!);
                      setTokenCopied(true);
                      setTimeout(() => setTokenCopied(false), 3000);
                    } catch (error) {
                      console.error('Failed to copy token:', error);
                    }
                  }}
                  className={`px-3 py-1.5 text-sm font-semibold rounded-lg flex items-center gap-2 transition-all shadow-lg hover:shadow-xl ${
                    tokenCopied 
                      ? 'bg-green-600 text-white' 
                      : 'bg-gradient-to-r from-primary-600 to-accent-blue text-white hover:scale-105 active:scale-95'
                  }`}
                  title="Copy token to clipboard"
                >
                  <FontAwesomeIcon icon={faCopy} className="w-3 h-3" />
                  {tokenCopied ? 'Copied ✓' : 'Copy Token'}
                </button>
              </div>
            </div>
          )}

          {/* Timestamps */}
          <div className="bg-gradient-to-br from-gray-50 to-primary-50 dark:from-gray-800 dark:to-gray-700 rounded-lg p-4">
            <div className="grid grid-cols-2 gap-6">
              <div>
                <label className="block text-sm font-medium text-gray-600 dark:text-gray-300 mb-1">Created</label>
                <p className="text-sm text-gray-900 dark:text-gray-100">
                  {new Date(displayPeer.created_at).toLocaleString()}
                </p>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-600 dark:text-gray-300 mb-1">Last Updated</label>
                <p className="text-sm text-gray-900 dark:text-gray-100">
                  {new Date(displayPeer.updated_at).toLocaleString()}
                </p>
              </div>
            </div>
          </div>

          {/* Actions */}
          <div className="flex justify-between gap-3 pt-4 border-t border-gray-200 dark:border-gray-700">
            {canEdit && (
              <button
                onClick={handleDelete}
                disabled={deleting}
                title="Delete Peer"
                className="group px-4 py-2.5 bg-gradient-to-r from-red-600 to-red-500 text-white rounded-xl hover:scale-105 active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer transition-all flex items-center gap-2 font-semibold shadow-lg hover:shadow-xl"
              >
                <svg className="w-5 h-5 group-hover:scale-110 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
                {deleting ? 'Deleting...' : 'Delete'}
              </button>
            )}
            <div className="flex gap-3 ml-auto">
              <button
                onClick={handleClose}
                className="px-4 py-2.5 text-gray-700 dark:text-gray-100 bg-gray-100 dark:bg-gray-700 rounded-xl hover:bg-gray-200 dark:hover:bg-gray-600 hover:scale-105 active:scale-95 cursor-pointer transition-all font-semibold shadow hover:shadow-lg"
              >
                Close
              </button>
              {canEdit && (
                <button
                  onClick={handleEdit}
                  title="Edit Peer"
                  className="group px-4 py-2.5 bg-gradient-to-r from-primary-600 to-accent-blue text-white rounded-xl hover:scale-105 active:scale-95 cursor-pointer transition-all flex items-center gap-2 font-semibold shadow-lg hover:shadow-xl"
                >
                  <svg className="w-5 h-5 group-hover:rotate-12 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                  </svg>
                  Edit
                </button>
              )}
            </div>
          </div>
        </div>
      </Modal>

      {/* Edit Modal */}
      {peer.is_jump ? (
        <JumpPeerModal
          isOpen={isEditModalOpen}
          onClose={() => setIsEditModalOpen(false)}
          onSuccess={handleEditSuccess}
          networkId={peer.network_id!}
          peer={peer}
          users={users}
        />
      ) : (
        <RegularPeerModal
          isOpen={isEditModalOpen}
          onClose={() => setIsEditModalOpen(false)}
          onSuccess={handleEditSuccess}
          networkId={peer.network_id!}
          peer={peer}
          users={users}
        />
      )}
    </>
  );
}
