import { useState, useEffect, useMemo } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faServer, faLaptop, faRocket, faCopy, faCheckCircle, faTimesCircle, faRoute, faNetworkWired } from '@fortawesome/free-solid-svg-icons';
import type { PeerReachability } from '../types';
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
  const [activeTab, setActiveTab] = useState<'configuration' | 'access' | 'reachability'>('configuration');
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [revoking, setRevoking] = useState(false);
  const [configLoading, setConfigLoading] = useState(false);
  const [configText, setConfigText] = useState<string | null>(null);
  const [configError, setConfigError] = useState<string | null>(null);
  const [configCopied, setConfigCopied] = useState(false);
  const [tokenCopied, setTokenCopied] = useState(false);
  const [groups, setGroups] = useState<{ id: string; name: string; description?: string }[]>([]);
  const [policies, setPolicies] = useState<Array<{
    direction: string;
    action: string;
    target_type: string;
    target: string;
    description?: string;
    _policyName: string;
    _groupName: string;
    _groupPriority: number;
  }>>([]);
  const [routes, setRoutes] = useState<{ id: string; name: string; destination_cidr: string; description?: string }[]>([]);
  const [loadingDetails, setLoadingDetails] = useState(false);
  const [reachability, setReachability] = useState<PeerReachability | null>(null);
  const [loadingReachability, setLoadingReachability] = useState(false);
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

  // Determine which peer data to display.
  // The live `currentPeer` comes from the server endpoint which does not include
  // network_id (it is only a URL parameter). Preserve network_id and network_name
  // from the original `peer` prop so that config download / delete operations work.
  // useMemo prevents a new object reference on every render (which would otherwise
  // trigger the loadPeerDetails useEffect infinitely every time the modal re-renders).
  const displayPeer = useMemo(
    () => currentPeer
      ? { ...currentPeer, network_id: peer?.network_id, network_name: peer?.network_name }
      : peer,
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [currentPeer, peer?.network_id, peer?.network_name]
  );

  // Load groups, policies, and routes for this peer
  useEffect(() => {
    const loadPeerDetails = async () => {
      if (!isOpen || !peer?.network_id || !displayPeer || !displayPeer.group_ids || displayPeer.group_ids.length === 0) {
        setGroups([]);
        setPolicies([]);
        setRoutes([]);
        return;
      }

      setLoadingDetails(true);
      try {
        // Fetch all groups for the network
        const allGroups = await api.getGroups(peer.network_id);
        
        // Filter to only groups this peer belongs to and sort by priority (lower = higher priority)
        const peerGroups = allGroups
          .filter((g: { id: string; priority: number }) => displayPeer.group_ids?.includes(g.id))
          .sort((a: { priority: number }, b: { priority: number }) => a.priority - b.priority);
        setGroups(peerGroups);

        // Collect route IDs from peer's groups
        const routeIds = new Set<string>();
        peerGroups.forEach((group: { route_ids?: string[] }) => {
          group.route_ids?.forEach((id: string) => routeIds.add(id));
        });

        // Fetch all policies for the network
        const allPolicies = await api.getPolicies(peer.network_id);
        
        // Collect effective rules in the correct order:
        // 1. Groups are already sorted by priority (lower number = higher priority)
        // 2. Within each group, policies are in their defined order
        // 3. Within each policy, rules are in their defined order
        const effectiveRules: Array<{
          direction: string;
          action: string;
          target_type: string;
          target: string;
          description?: string;
          _policyName: string;
          _groupName: string;
          _groupPriority: number;
        }> = [];
        const seenPolicyIds = new Set<string>();
        
        for (const group of peerGroups) {
          if (!group.policy_ids || group.policy_ids.length === 0) continue;
          
          // Get policies for this group in order
          for (const policyId of group.policy_ids) {
            // Skip if we've already processed this policy from a higher priority group
            if (seenPolicyIds.has(policyId)) continue;
            seenPolicyIds.add(policyId);
            
            const policy = allPolicies.find((p: { id: string; name: string; rules?: Array<{
              direction: string;
              action: string;
              target_type: string;
              target: string;
              description?: string;
            }> }) => p.id === policyId);
            if (policy && policy.rules) {
              // Add each rule with metadata about which group/policy it comes from
              policy.rules.forEach((rule: {
                direction: string;
                action: string;
                target_type: string;
                target: string;
                description?: string;
              }) => {
                effectiveRules.push({
                  ...rule,
                  _policyName: policy.name,
                  _groupName: group.name,
                  _groupPriority: group.priority,
                });
              });
            }
          }
        }
        
        setPolicies(effectiveRules);

        // Fetch routes
        if (routeIds.size > 0) {
          const allRoutes = await api.getRoutes(peer.network_id);
          const peerRoutes = allRoutes.filter((r: { id: string }) => routeIds.has(r.id));
          setRoutes(peerRoutes);
        } else {
          setRoutes([]);
        }
      } catch (error) {
        console.error('Failed to load peer details:', error);
      } finally {
        setLoadingDetails(false);
      }
    };

    void loadPeerDetails();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, peer?.network_id, displayPeer?.id, displayPeer?.group_ids?.join(',')]);

  // Load reachability data when the reachability tab is opened
  useEffect(() => {
    if (activeTab !== 'reachability' || !isOpen || !peer?.network_id || !peer?.id) return;
    const load = async () => {
      setLoadingReachability(true);
      try {
        const data = await api.getPeerReachability(peer.network_id!, peer.id);
        setReachability(data);
      } catch (error) {
        console.error('Failed to load reachability:', error);
        setReachability(null);
      } finally {
        setLoadingReachability(false);
      }
    };
    void load();
  }, [activeTab, isOpen, peer?.network_id, peer?.id]);

  const handleClose = () => {
    setActiveTab('configuration');
    setIsEditModalOpen(false);
    setConfigText(null);
    setConfigError(null);
    setConfigCopied(false);
    setTokenCopied(false);
    setReachability(null);
    onClose();
  };

  if (!peer || !displayPeer) return null;

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

  const handleRevoke = async () => {
    if (!confirm(
      `Revoke captive-portal authentication for "${displayPeer.name}"?\n\n` +
      `The peer will remain in the network, but the next request from it will be redirected to the captive portal for re-authentication via SSO. ` +
      `Use this when you suspect a peer's WireGuard config is being shared or stolen.`
    )) {
      return;
    }
    setRevoking(true);
    try {
      await api.revokePeerAuthentication(displayPeer.network_id!, displayPeer.id);
      onUpdate();
    } catch (error) {
      const err = error as { response?: { data?: { error?: string } } };
      alert(err.response?.data?.error || 'Failed to revoke authentication');
    } finally {
      setRevoking(false);
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
                <h3 className="text-2xl font-bold bg-gradient-to-r from-gray-900 to-gray-600 dark:from-gray-100 dark:to-gray-300 bg-clip-text text-transparent">{displayPeer.name}</h3>
                <p className="text-sm text-gray-600 dark:text-gray-300 mt-1">ID: {displayPeer.id}</p>
              </div>
            </div>
          </div>

          {/* Tabs */}
          <div className="flex border-b border-gray-200 dark:border-gray-700">
            <button
              type="button"
              onClick={() => setActiveTab('configuration')}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                activeTab === 'configuration'
                  ? 'border-primary-600 text-primary-600 dark:text-primary-400'
                  : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
              }`}
            >
              Configuration
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('access')}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                activeTab === 'access'
                  ? 'border-primary-600 text-primary-600 dark:text-primary-400'
                  : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
              }`}
            >
              Access Control
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('reachability')}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                activeTab === 'reachability'
                  ? 'border-primary-600 text-primary-600 dark:text-primary-400'
                  : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
              }`}
            >
              Reachability
            </button>
          </div>

          {/* Configuration Tab */}
          {activeTab === 'configuration' && (
          <div className="space-y-6">
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
              <p className="text-lg font-mono text-gray-900 dark:text-gray-100">{displayPeer.name}.{network?.name || displayPeer.network_name || 'network'}.{network?.domain_suffix || 'internal' }</p>
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
              {!displayPeer.owner_id && (
                <p className="text-sm text-amber-600 dark:text-amber-400">
                  Config download is disabled — this peer has no owner. Assign an owner to enable it.
                </p>
              )}
              <div className="flex gap-2">
                <button
                  disabled={configLoading || configCopied || !displayPeer.owner_id}
                  onClick={async () => {
                    if (!peer.network_id) return;
                    setConfigLoading(true);
                    setConfigError(null);
                    try {
                      let cfg = configText;
                      if (!cfg) {
                        cfg = await api.getPeerConfig(peer.network_id, peer.id);
                        setConfigText(cfg);
                      }
                      await navigator.clipboard.writeText(cfg);
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
                  disabled={configLoading || !displayPeer.owner_id}
                  onClick={async () => {
                    if (!peer.network_id) return;
                    setConfigError(null);
                    setConfigLoading(true);
                    try {
                      let cfg = configText;
                      if (!cfg) {
                        cfg = await api.getPeerConfig(peer.network_id, peer.id);
                        setConfigText(cfg);
                      }
                      const blob = new Blob([cfg], { type: 'text/plain' });
                      const url = URL.createObjectURL(blob);
                      const a = document.createElement('a');
                      a.href = url;
                      
                      a.download = `${displayPeer.name}.${network?.name || displayPeer.network_name || 'network'}.${network?.domain_suffix || 'internal' }.conf`;
                      document.body.appendChild(a);
                      a.click();
                      document.body.removeChild(a);
                      URL.revokeObjectURL(url);
                    } catch (e) {
                      const error = e as { message?: string };
                      setConfigError(error?.message || 'Failed to download config');
                    } finally {
                      setConfigLoading(false);
                    }
                  }}
                  className="px-4 py-2 text-sm font-semibold bg-gradient-to-r from-green-600 to-accent-green text-white rounded-lg hover:scale-105 active:scale-95 disabled:opacity-50 transition-all"
                >
                  {configLoading ? 'Fetching...' : 'Download .conf'}
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
          </div>
          )}

          {/* Access Control Tab */}
          {activeTab === 'access' && (
          <div className="space-y-6">
          {/* Group Memberships */}
          {displayPeer.group_ids && displayPeer.group_ids.length > 0 && (
            <div className="bg-gradient-to-br from-gray-50 to-primary-50 dark:from-gray-800 dark:to-gray-700 rounded-lg p-4">
              <h4 className="text-sm font-medium text-gray-700 dark:text-gray-100 mb-3">Group Memberships</h4>
              {loadingDetails ? (
                <div className="text-sm text-gray-500">Loading groups...</div>
              ) : groups.length > 0 ? (
                <div className="space-y-2">
                  {groups.map((group) => (
                    <div key={group.id} className="bg-white dark:bg-gray-700 px-3 py-2 rounded">
                      <div className="text-sm font-medium text-gray-900 dark:text-gray-100">{group.name}</div>
                      {group.description && (
                        <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">{group.description}</div>
                      )}
                    </div>
                  ))}
                </div>
              ) : (
                <div className="text-sm text-gray-500">No groups found</div>
              )}
            </div>
          )}

          {/* Effective Rules */}
          {displayPeer.group_ids && displayPeer.group_ids.length > 0 && (
            <div className="bg-gradient-to-br from-gray-50 to-primary-50 dark:from-gray-800 dark:to-gray-700 rounded-lg p-4">
              <h4 className="text-sm font-medium text-gray-700 dark:text-gray-100 mb-3">
                Effective Rules ({policies.length})
                {policies.length > 0 && (
                  <span className="text-xs font-normal text-gray-500 dark:text-gray-400 ml-2">
                    (Applied in order by jump server)
                  </span>
                )}
              </h4>
              {loadingDetails ? (
                <div className="text-sm text-gray-500">Loading rules...</div>
              ) : policies.length > 0 ? (
                <div className="space-y-2">
                  {policies.map((rule, index) => (
                    <div key={`${rule._policyName}-${index}`} className="bg-white dark:bg-gray-700 px-3 py-2 rounded">
                      <div className="flex items-start gap-2">
                        <span className="text-xs font-mono text-gray-400 dark:text-gray-500 mt-0.5">#{index + 1}</span>
                        <div className="flex-1">
                          <div className="flex items-center gap-2 flex-wrap">
                            <span className={`px-2 py-0.5 text-xs font-semibold rounded ${
                              rule.action === 'allow' 
                                ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                                : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                            }`}>
                              {rule.action.toUpperCase()}
                            </span>
                            <span className="px-2 py-0.5 text-xs font-semibold rounded bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">
                              {rule.direction.toUpperCase()}
                            </span>
                            <span className="text-xs font-mono text-gray-900 dark:text-gray-100">
                              {rule.target_type}: {rule.target}
                            </span>
                          </div>
                          {rule.description && (
                            <div className="text-xs text-gray-600 dark:text-gray-400 mt-1">{rule.description}</div>
                          )}
                          <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                            From: <span className="font-medium">{rule._policyName}</span>
                            {' '} in <span className="font-medium">{rule._groupName}</span>
                            {' '}(priority: {rule._groupPriority})
                          </div>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="text-sm text-gray-500">No rules attached to this peer's groups</div>
              )}
            </div>
          )}

          {/* Effective Routes */}
          {displayPeer.group_ids && displayPeer.group_ids.length > 0 && (
            <div className="bg-gradient-to-br from-gray-50 to-primary-50 dark:from-gray-800 dark:to-gray-700 rounded-lg p-4">
              <h4 className="text-sm font-medium text-gray-700 dark:text-gray-100 mb-3">Effective Routes</h4>
              {loadingDetails ? (
                <div className="text-sm text-gray-500">Loading routes...</div>
              ) : routes.length > 0 ? (
                <div className="space-y-2">
                  {routes.map((route) => (
                    <div key={route.id} className="bg-white dark:bg-gray-700 px-3 py-2 rounded">
                      <div className="text-sm font-medium text-gray-900 dark:text-gray-100">{route.name}</div>
                      <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                        {route.destination_cidr}
                      </div>
                      {route.description && (
                        <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">{route.description}</div>
                      )}
                    </div>
                  ))}
                </div>
              ) : (
                <div className="text-sm text-gray-500">No routes attached to this peer's groups</div>
              )}
            </div>
          )}
          </div>
          )}

          {/* Reachability Tab */}
          {activeTab === 'reachability' && (
          <div className="space-y-6">
            {loadingReachability ? (
              <div className="text-sm text-gray-500 dark:text-gray-400 text-center py-8">Computing reachability...</div>
            ) : !reachability ? (
              <div className="text-sm text-gray-500 dark:text-gray-400 text-center py-8">Failed to load reachability data.</div>
            ) : (
              <>
                {/* Jump peer note */}
                {reachability.is_jump && (
                  <div className="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-700 rounded-lg px-4 py-3 text-sm text-blue-700 dark:text-blue-300">
                    This is a jump peer. It acts as the gateway and enforces iptables firewall rules for all other peers in the network.
                  </div>
                )}

                {/* Peer Access (ACL layer) */}
                <div className="bg-gradient-to-br from-gray-50 to-primary-50 dark:from-gray-800 dark:to-gray-700 rounded-lg p-4">
                  <h4 className="text-sm font-medium text-gray-700 dark:text-gray-100 mb-3 flex items-center gap-2">
                    <FontAwesomeIcon icon={faNetworkWired} className="text-primary-500" />
                    Peer Access ({(reachability.peer_access ?? []).filter(p => p.allowed).length} allowed, {(reachability.peer_access ?? []).filter(p => !p.allowed).length} denied)
                  </h4>
                  {(reachability.peer_access ?? []).length === 0 ? (
                    <div className="text-sm text-gray-500 dark:text-gray-400">No other peers in this network.</div>
                  ) : (
                    <div className="space-y-1.5">
                      {(reachability.peer_access ?? []).map(pa => (
                        <div key={pa.peer_id} className="flex items-center justify-between bg-white dark:bg-gray-700 px-3 py-2 rounded gap-3">
                          <div className="flex items-center gap-2 min-w-0">
                            <FontAwesomeIcon
                              icon={pa.allowed ? faCheckCircle : faTimesCircle}
                              className={pa.allowed ? 'text-green-500 shrink-0' : 'text-red-500 shrink-0'}
                            />
                            <div className="min-w-0">
                              <span className="text-sm font-medium text-gray-900 dark:text-gray-100 truncate block">{pa.peer_name}</span>
                              <span className="text-xs font-mono text-gray-500 dark:text-gray-400">{pa.address}</span>
                            </div>
                            {pa.is_jump && (
                              <span className="px-1.5 py-0.5 text-xs rounded bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300 shrink-0">jump</span>
                            )}
                          </div>
                          <span className={`text-xs font-medium px-2 py-0.5 rounded shrink-0 ${
                            pa.allowed
                              ? 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
                              : 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300'
                          }`}>
                            {pa.reason === 'acl_disabled' ? 'allowed (no ACL)' :
                             pa.reason === 'blocked' ? 'blocked' :
                             pa.reason === 'deny_rule' ? 'deny rule' :
                             pa.reason === 'allow_rule' ? 'allow rule' : 'default allow'}
                          </span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                {/* Policy Rules (firewall layer on jump peer) */}
                {(reachability.rules ?? []).length > 0 && (
                  <div className="bg-gradient-to-br from-gray-50 to-primary-50 dark:from-gray-800 dark:to-gray-700 rounded-lg p-4">
                    <h4 className="text-sm font-medium text-gray-700 dark:text-gray-100 mb-3">
                      Firewall Rules ({(reachability.rules ?? []).length})
                      <span className="text-xs font-normal text-gray-500 dark:text-gray-400 ml-2">enforced on jump peer</span>
                    </h4>
                    <div className="space-y-2">
                      {(reachability.rules ?? []).map((rule, i) => (
                        <div key={i} className="bg-white dark:bg-gray-700 px-3 py-2 rounded">
                          <div className="flex items-start gap-2">
                            <FontAwesomeIcon
                              icon={rule.action === 'allow' ? faCheckCircle : faTimesCircle}
                              className={`mt-0.5 shrink-0 ${rule.action === 'allow' ? 'text-green-500' : 'text-red-500'}`}
                            />
                            <div className="flex-1 min-w-0">
                              <div className="flex flex-wrap items-center gap-1.5 mb-1">
                                <span className={`px-1.5 py-0.5 text-xs font-semibold rounded ${
                                  rule.action === 'allow'
                                    ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                                    : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                                }`}>{rule.action.toUpperCase()}</span>
                                <span className="px-1.5 py-0.5 text-xs font-semibold rounded bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">
                                  {rule.direction.toUpperCase()}
                                </span>
                                <span className="text-xs font-mono text-gray-700 dark:text-gray-200">
                                  {rule.target_type}: {rule.target}
                                </span>
                              </div>
                              {rule.addresses && rule.addresses.length > 0 && rule.target_type !== 'cidr' && (
                                <div className="flex flex-wrap gap-1 mb-1">
                                  {rule.addresses.map(addr => (
                                    <span key={addr} className="px-1.5 py-0.5 text-xs font-mono bg-gray-100 dark:bg-gray-600 text-gray-700 dark:text-gray-200 rounded">
                                      {addr}
                                    </span>
                                  ))}
                                </div>
                              )}
                              {rule.description && (
                                <div className="text-xs text-gray-500 dark:text-gray-400">{rule.description}</div>
                              )}
                              <div className="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
                                {rule.policy_name} · {rule.group_name}
                              </div>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* External Routes */}
                {(reachability.routes ?? []).length > 0 && (
                  <div className="bg-gradient-to-br from-gray-50 to-primary-50 dark:from-gray-800 dark:to-gray-700 rounded-lg p-4">
                    <h4 className="text-sm font-medium text-gray-700 dark:text-gray-100 mb-3 flex items-center gap-2">
                      <FontAwesomeIcon icon={faRoute} className="text-primary-500" />
                      External Routes ({(reachability.routes ?? []).length})
                    </h4>
                    <div className="space-y-1.5">
                      {(reachability.routes ?? []).map(route => (
                        <div key={route.route_id} className="bg-white dark:bg-gray-700 px-3 py-2 rounded">
                          <div className="flex items-center justify-between gap-2">
                            <div>
                              <span className="text-sm font-medium text-gray-900 dark:text-gray-100">{route.route_name}</span>
                              <span className="text-xs font-mono text-gray-500 dark:text-gray-400 ml-2">{route.destination_cidr}</span>
                            </div>
                            <div className="text-xs text-gray-500 dark:text-gray-400 shrink-0">
                              via <span className="font-medium text-gray-700 dark:text-gray-200">{route.jump_peer_name}</span>
                            </div>
                          </div>
                          <div className="text-xs text-gray-400 dark:text-gray-500 mt-0.5">{route.group_name}</div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* No DB services note */}
                {(reachability.rules ?? []).length === 0 && (reachability.routes ?? []).length === 0 && !reachability.is_jump && (
                  <div className="text-sm text-gray-500 dark:text-gray-400 text-center">
                    No policy rules or routes configured for this peer.
                  </div>
                )}
              </>
            )}
          </div>
          )}

          {/* Actions */}
          <div className="flex justify-between gap-3 pt-4 border-t border-gray-200 dark:border-gray-700">
            {canEdit && (
              <div className="flex gap-2">
                <button
                  onClick={handleDelete}
                  disabled={deleting || revoking}
                  title="Delete Peer"
                  className="group px-4 py-2.5 bg-gradient-to-r from-red-600 to-red-500 text-white rounded-xl hover:scale-105 active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer transition-all flex items-center gap-2 font-semibold shadow-lg hover:shadow-xl"
                >
                  <svg className="w-5 h-5 group-hover:scale-110 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                  {deleting ? 'Deleting...' : 'Delete'}
                </button>
                <button
                  onClick={handleRevoke}
                  disabled={deleting || revoking}
                  title="Force re-authentication: removes the peer from the captive-portal whitelist so the next request from it is redirected to SSO. Use if a config is suspected of being shared/stolen."
                  className="group px-4 py-2.5 bg-gradient-to-r from-amber-600 to-orange-500 text-white rounded-xl hover:scale-105 active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer transition-all flex items-center gap-2 font-semibold shadow-lg hover:shadow-xl"
                >
                  <svg className="w-5 h-5 group-hover:scale-110 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                  </svg>
                  {revoking ? 'Revoking...' : 'Revoke Auth'}
                </button>
              </div>
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
