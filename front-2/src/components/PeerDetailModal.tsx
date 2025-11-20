import { useState, useEffect, useRef } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faServer, faLaptop, faRocket } from '@fortawesome/free-solid-svg-icons';
import Modal from './Modal';
import JumpPeerModal from './JumpPeerModal';
import RegularPeerModal from './RegularPeerModal';
import NetworkTopology from './NetworkTopology';
import api from '../api/client';
import type { Peer } from '../types';

interface PeerDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  peer: Peer | null;
  onUpdate: () => void;
}

export default function PeerDetailModal({ isOpen, onClose, peer, onUpdate }: PeerDetailModalProps) {
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [currentPeer, setCurrentPeer] = useState<Peer | null>(peer);
  const [networkPeers, setNetworkPeers] = useState<Peer[]>([]);
  const [loadingPeers, setLoadingPeers] = useState(false);
  const [configLoading, setConfigLoading] = useState(false);
  const [configText, setConfigText] = useState<string | null>(null);
  const [configError, setConfigError] = useState<string | null>(null);
  const [configCopied, setConfigCopied] = useState(false);

  const peerDetailsLoadedRef = useRef(false);
  const networkPeersLoadedRef = useRef(false);

  useEffect(() => {
    if (isOpen && peer?.network_id && peer?.id) {
      if (!peerDetailsLoadedRef.current) {
        peerDetailsLoadedRef.current = true;
        loadPeerDetails();
      }
      if (!networkPeersLoadedRef.current) {
        networkPeersLoadedRef.current = true;
        loadNetworkPeers();
      }
    }
  }, [isOpen, peer?.network_id, peer?.id]);

  const loadPeerDetails = async () => {
    if (!peer?.network_id || !peer?.id) return;
    
    try {
      const updatedPeer = await api.getPeer(peer.network_id, peer.id);
      setCurrentPeer(updatedPeer);
    } catch (error) {
      console.error('Failed to load peer details:', error);
      setCurrentPeer(peer);
    }
  };

  const loadNetworkPeers = async () => {
    if (!peer?.network_id) return;
    
    setLoadingPeers(true);
    try {
      const peers = await api.getAllNetworkPeers(peer.network_id);
      setNetworkPeers(peers);
    } catch (error) {
      console.error('Failed to load network peers:', error);
      setNetworkPeers([]);
    } finally {
      setLoadingPeers(false);
    }
  };

  const handleClose = () => {
    // Reset state before closing
    setIsEditModalOpen(false);
    setConfigText(null);
    setConfigError(null);
    setConfigCopied(false);
    setCurrentPeer(null);
    onClose();
  };

  if (!peer) return null;
  const displayPeer = currentPeer || peer;

  const handleDelete = async () => {
    if (!confirm(`Are you sure you want to delete peer "${displayPeer.name}"? This action cannot be undone.`)) {
      return;
    }
    
    setDeleting(true);
    try {
      await api.deletePeer(displayPeer.network_id!, displayPeer.id);
      onUpdate();
      onClose();
    } catch (error: any) {
      alert(error.response?.data?.error || 'Failed to delete peer');
    } finally {
      setDeleting(false);
    }
  };

  const handleEdit = () => {
    setIsEditModalOpen(true);
  };

  const handleEditSuccess = () => {
    setIsEditModalOpen(false);
    loadPeerDetails(); // Reload peer details after edit
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
              <div className="text-5xl text-primary-600 dark:text-primary-400">
                <FontAwesomeIcon icon={displayPeer.is_jump ? faRocket : (displayPeer.use_agent ? faServer : faLaptop)} />
              </div>
              <div>
                <h3 className="text-2xl font-bold text-gray-900 dark:text-white">{displayPeer.name}</h3>
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">ID: {displayPeer.id}</p>
                <div className="flex items-center gap-2 mt-2">
                  {/* WireGuard status dot */}
                  <span className={`w-3 h-3 rounded-full ${
                    displayPeer.session_status?.current_session?.reported_endpoint ? 'bg-green-500' : 'bg-red-500'
                  }`} title={displayPeer.session_status?.current_session?.reported_endpoint ? 'WireGuard Up' : 'WireGuard Down'}></span>
                  {/* Type badge (Jump / Regular) */}
                  {displayPeer.is_jump ? (
                    <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-purple-100 text-purple-800">
                      Jump
                    </span>
                  ) : (
                    <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                      Regular
                    </span>
                  )}
                  {/* Agent badge dot */}
                  <span className="flex items-center gap-1">
                    <span className={`w-3 h-3 rounded-full ${
                      !displayPeer.use_agent ? 'bg-gray-400' : (displayPeer.session_status?.has_active_agent ? 'bg-green-500' : 'bg-red-500')
                    }`} title={!displayPeer.use_agent ? 'No Agent' : (displayPeer.session_status?.has_active_agent ? 'Agent Connected' : 'Agent Disconnected')}></span>
                  </span>
                </div>
              </div>
            </div>
          </div>

          {/* Network Info */}
          <div>
            <label className="block text-sm font-medium text-gray-500 mb-1">Network</label>
            <p className="text-lg text-gray-900 dark:text-white">{displayPeer.network_name || displayPeer.network_id}</p>
          </div>

          {/* Connection Info */}
          {displayPeer.is_jump ? (
            <div className="grid grid-cols-2 gap-6">
              <div>
                <label className="block text-sm font-medium text-gray-500 mb-1">IP Address</label>
                <p className="text-lg font-mono text-gray-900 dark:text-white">{displayPeer.address}</p>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-500 mb-1">Configured Endpoint</label>
                <p className="text-lg font-mono text-gray-900 dark:text-white">{displayPeer.endpoint || 'N/A'}</p>
              </div>
            </div>
          ) : (
            <div>
              <label className="block text-sm font-medium text-gray-500 mb-1">IP Address</label>
              <p className="text-lg font-mono text-gray-900 dark:text-white">{displayPeer.address}</p>
            </div>
          )}

          {/* Status Information */}
          <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
            <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Status Information</h4>
            <div className="space-y-3">
              {/* Agent Status */}
              <div className="flex items-center justify-between">
                <span className="text-sm text-gray-600 dark:text-gray-400">Agent Status</span>
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
              
              {/* Peer Up/Down */}
              <div className="flex items-center justify-between">
                <span className="text-sm text-gray-600 dark:text-gray-400">Peer Status</span>
                <span className={`inline-flex items-center px-3 py-1 rounded-full text-xs font-medium ${
                  displayPeer.session_status?.current_session?.reported_endpoint
                    ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                    : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'
                }`}>
                  <span className={`w-2 h-2 rounded-full mr-1.5 ${
                    displayPeer.session_status?.current_session?.reported_endpoint ? 'bg-green-500' : 'bg-gray-400'
                  }`}></span>
                  {displayPeer.session_status?.current_session?.reported_endpoint ? 'Up' : 'Down'}
                </span>
              </div>

              {/* Reported Endpoint */}
              {displayPeer.session_status?.current_session?.reported_endpoint && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-gray-600 dark:text-gray-400">Reported Endpoint</span>
                  <span className="text-sm font-mono text-gray-900 dark:text-white">
                    {displayPeer.session_status.current_session.reported_endpoint}
                  </span>
                </div>
              )}

              {/* Last Seen */}
              {displayPeer.session_status?.current_session?.last_seen && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-gray-600 dark:text-gray-400">Last Seen</span>
                  <span className="text-sm text-gray-900 dark:text-white">
                    {new Date(displayPeer.session_status.current_session.last_seen).toLocaleString()}
                  </span>
                </div>
              )}

              {/* Hostname */}
              {displayPeer.session_status?.current_session?.hostname && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-gray-600 dark:text-gray-400">Hostname</span>
                  <span className="text-sm font-mono text-gray-900 dark:text-white">
                    {displayPeer.session_status.current_session.hostname}
                  </span>
                </div>
              )}

              {/* Last Checked */}
              {displayPeer.session_status?.last_checked && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-gray-600 dark:text-gray-400">Last Status Check</span>
                  <span className="text-sm text-gray-900 dark:text-white">
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
              <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300">WireGuard Configuration</h4>
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
                    } catch (e: any) {
                      setConfigError(e?.message || 'Failed to copy config');
                    } finally {
                      setConfigLoading(false);
                    }
                  }}
                  className={`px-3 py-1 text-xs font-medium rounded disabled:opacity-50 ${configCopied ? 'bg-green-600 text-white' : 'bg-primary-600 text-white hover:bg-primary-700'}`}
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
                        a.download = `${peer.name || 'peer'}-${peer.id}.conf`;
                        document.body.appendChild(a);
                        a.click();
                        document.body.removeChild(a);
                        URL.revokeObjectURL(url);
                      }
                    } catch (e: any) {
                      setConfigError(e?.message || 'Failed to download config');
                    } finally {
                      setConfigLoading(false);
                    }
                  }}
                  className="px-3 py-1 text-xs font-medium bg-green-600 text-white rounded hover:bg-green-700 disabled:opacity-50"
                >
                  {configLoading && !configCopied && !configText ? 'Fetching...' : 'Download .conf'}
                </button>
              </div>
              {configError && <p className="text-xs text-red-600">{configError}</p>}
            </div>
          )}

          {/* Jump Server Specific */}
          {displayPeer.is_jump && (
            <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
              <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Jump Server Configuration</h4>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">Listen Port</label>
                  <p className="text-sm text-gray-900 dark:text-white">{displayPeer.listen_port || 'N/A'}</p>
                </div>
                <div>
                  <label className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">NAT Interface</label>
                  <p className="text-sm text-gray-900 dark:text-white">{displayPeer.jump_nat_interface || 'N/A'}</p>
                </div>
              </div>
            </div>
          )}

          {/* Regular Peer Specific */}
          {!displayPeer.is_jump && (
            <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
              <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Peer Configuration</h4>
              <div className="space-y-2">
                <div className="flex justify-between">
                  <span className="text-sm text-gray-600 dark:text-gray-400">Isolated</span>
                  <span className="text-sm font-medium text-gray-900 dark:text-white">{displayPeer.is_isolated ? 'Yes' : 'No'}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-sm text-gray-600 dark:text-gray-400">Full Encapsulation</span>
                  <span className="text-sm font-medium text-gray-900 dark:text-white">{displayPeer.full_encapsulation ? 'Yes' : 'No'}</span>
                </div>
              </div>
            </div>
          )}

          {/* Additional Allowed IPs */}
          {displayPeer.additional_allowed_ips && displayPeer.additional_allowed_ips.length > 0 && (
            <div>
              <label className="block text-sm font-medium text-gray-500 mb-2">Additional Allowed IPs</label>
              <div className="space-y-1">
                {displayPeer.additional_allowed_ips.map((ip, index) => (
                  <div key={index} className="bg-gray-50 dark:bg-gray-700 px-3 py-2 rounded text-sm font-mono text-gray-900 dark:text-gray-100">
                    {ip}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Agent Token (if uses agent) */}
          {displayPeer.use_agent && displayPeer.token && (
            <div>
              <label className="block text-sm font-medium text-gray-500 mb-1">Agent Token</label>
              <p className="text-sm font-mono text-gray-900 bg-yellow-50 p-3 rounded break-all border border-yellow-200">
                {displayPeer.token}
              </p>
            </div>
          )}

          {/* Owner */}
          {displayPeer.owner_id && (
            <div>
              <label className="block text-sm font-medium text-gray-500 mb-1">Owner</label>
              <p className="text-sm text-gray-900">{displayPeer.owner_id}</p>
            </div>
          )}

          {/* Timestamps */}
          <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
            <div className="grid grid-cols-2 gap-6">
              <div>
                <label className="block text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">Created</label>
                <p className="text-sm text-gray-900 dark:text-white">
                  {new Date(displayPeer.created_at).toLocaleString()}
                </p>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">Last Updated</label>
                <p className="text-sm text-gray-900 dark:text-white">
                  {new Date(displayPeer.updated_at).toLocaleString()}
                </p>
              </div>
            </div>
          </div>

          {/* Network Topology */}
          <div className="pt-4 border-t border-gray-200 dark:border-gray-700">
            {loadingPeers ? (
              <div className="bg-gray-50 dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg p-8 text-center">
                <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
                <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">Loading network topology...</p>
              </div>
            ) : networkPeers.length > 0 ? (
              <NetworkTopology 
                peer={peer} 
                allPeers={networkPeers}
              />
            ) : (
              <div className="bg-gray-50 dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg p-8 text-center">
                <p className="text-sm text-gray-600 dark:text-gray-400">No peers available in this network</p>
              </div>
            )}
          </div>

          {/* Actions */}
          <div className="flex justify-between gap-3 pt-4 border-t border-gray-200 dark:border-gray-700">
            <button
              onClick={handleDelete}
              disabled={deleting}
              title="Delete Peer"
              className="px-4 py-2 text-red-600 bg-red-50 rounded-lg hover:bg-red-100 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer transition-colors flex items-center gap-2"
            >
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
              {deleting ? 'Deleting...' : 'Delete'}
            </button>
            <div className="flex gap-3">
              <button
                onClick={handleClose}
                className="px-4 py-2 text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 cursor-pointer transition-colors"
              >
                Close
              </button>
              <button
                onClick={handleEdit}
                title="Edit Peer"
                className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 cursor-pointer transition-colors flex items-center gap-2"
              >
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
                Edit
              </button>
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
        />
      ) : (
        <RegularPeerModal
          isOpen={isEditModalOpen}
          onClose={() => setIsEditModalOpen(false)}
          onSuccess={handleEditSuccess}
          networkId={peer.network_id!}
          peer={peer}
        />
      )}
    </>
  );
}
