import { useState, useMemo, useEffect } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faServer, faLaptop, faRocket, faPencil, faTrash } from '@fortawesome/free-solid-svg-icons';
import PageHeader from '../../components/PageHeader';
import JumpPeerModal from '../../components/JumpPeerModal';
import RegularPeerModal from '../../components/RegularPeerModal';
import PeerDetailModal from '../../components/PeerDetailModal';
import SearchableSelect from '../../components/SearchableSelect';
import { useNetworks, usePeers, useACLs } from '../../hooks/useQueries';
import { useAuth } from '../../contexts/AuthContext';
import api from '../../api/client';
import type { Peer, User } from '../../types';

export default function PeersPage() {
  const [page, setPage] = useState(1);
  const [isJumpModalOpen, setIsJumpModalOpen] = useState(false);
  const [isRegularModalOpen, setIsRegularModalOpen] = useState(false);
  const [editingPeer, setEditingPeer] = useState<Peer | null>(null);
  const [selectedNetworkId, setSelectedNetworkId] = useState<string>('');
  const [selectedPeer, setSelectedPeer] = useState<Peer | null>(null);
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false);
  const [users, setUsers] = useState<User[]>([]);
  const { user: currentUser } = useAuth();

  const isAdmin = currentUser?.role === 'administrator';

  // Fetch users for owner mapping (admin only)
  useEffect(() => {
    const loadUsers = async () => {
      try {
        const usersData = await api.getUsers();
        setUsers(usersData);
      } catch (error) {
        console.error('Failed to load users:', error);
      }
    };

    if (isAdmin) {
      void loadUsers();
    }
  }, [isAdmin]);

  // Create a map of user ID to user name
  const userMap = useMemo(() => {
    const map = new Map<string, string>();
    users.forEach(user => map.set(user.id, user.name));
    return map;
  }, [users]);
  
  // Filters
  const [filterNetwork, setFilterNetwork] = useState<string>('');
  const [filterType, setFilterType] = useState<string>('');
  const [filterStatus, setFilterStatus] = useState<string>('');
  const [filterIP, setFilterIP] = useState<string>('');

  const pageSize = 20;

  // React Query hooks
  const { data: networks = [], isLoading: networksLoading } = useNetworks();
  const { data: peersData, isLoading: peersLoading, refetch: refetchPeers } = usePeers(page, pageSize);
  const { data: networkACLs = {}, isLoading: aclsLoading } = useACLs(networks);

  const peers = useMemo(() => peersData?.peers || [], [peersData]);
  const total = peersData?.total || 0;
  const loading = networksLoading || peersLoading || aclsLoading;

  // Tick-driven "now" so the Connected column re-evaluates every 30 s without
  // a full refetch.  Calling Date.now() during render is impure (lint rule
  // react-hooks/purity), so we capture it in state.
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 30 * 1000);
    return () => clearInterval(id);
  }, []);

  // Calculate blocked peers from ACLs
  const blockedPeers = useMemo(() => {
    const blocked = new Set<string>();
    peers.forEach(peer => {
      if (peer.network_id && networkACLs[peer.network_id]) {
        const acl = networkACLs[peer.network_id];
        if (acl.enabled && acl.blocked_peers && acl.blocked_peers[peer.id]) {
          blocked.add(peer.id);
        }
      }
    });
    return blocked;
  }, [peers, networkACLs]);

  const handlePeerClick = (peer: Peer) => {
    setSelectedPeer(peer);
    setIsDetailModalOpen(true);
  };

  const handleCreateJump = () => {
    if (networks.length === 0) {
      alert('Please create a network first');
      return;
    }
    setEditingPeer(null);
    setSelectedNetworkId(networks[0].id);
    setIsJumpModalOpen(true);
  };

  const handleCreateRegular = () => {
    if (networks.length === 0) {
      alert('Please create a network first');
      return;
    }
    setEditingPeer(null);
    setSelectedNetworkId(networks[0].id);
    setIsRegularModalOpen(true);
  };

  const handleModalClose = () => {
    setIsJumpModalOpen(false);
    setIsRegularModalOpen(false);
    setEditingPeer(null);
    setSelectedNetworkId('');
  };

  const handleModalSuccess = () => {
    refetchPeers();
  };

  // Create options for SearchableSelect components
  const networkOptions = useMemo(() => 
    networks.map(network => ({
      value: network.id,
      label: network.name
    })), [networks]
  );

  // Apply filters to peers
  const filteredPeers = peers.filter(peer => {
    // Network filter
    if (filterNetwork && peer.network_id !== filterNetwork) return false;

    // Type filter (only jump or regular)
    if (filterType === 'jump' && !peer.is_jump) return false;
    if (filterType === 'regular' && peer.is_jump) return false;

    // Agent filter (connected/disconnected/none)
    const agentConnected = peer.use_agent && peer.session_status?.has_active_agent;
    if (filterStatus === 'agent-connected' && !agentConnected) return false;
    if (filterStatus === 'agent-disconnected' && (!peer.use_agent || agentConnected)) return false;
    if (filterStatus === 'agent-none' && peer.use_agent) return false;

    // IP filter
    if (filterIP && !peer.address.includes(filterIP)) return false;

    return true;
  });

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div>
      <PageHeader 
        title="Peers" 
        subtitle={`${total} peer${total !== 1 ? 's' : ''} total`}
        action={
          <div className="flex gap-2">
            <button
              onClick={handleCreateJump}
              className="group px-4 py-2.5 bg-gradient-to-r from-purple-600 to-accent-blue text-white rounded-xl hover:scale-105 active:scale-95 shadow-lg hover:shadow-xl flex items-center gap-2 cursor-pointer transition-all font-semibold"
            >
              <svg className="w-5 h-5 group-hover:rotate-90 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              Jump Peer
            </button>
            <button
              onClick={handleCreateRegular}
              className="group px-4 py-2.5 bg-gradient-to-r from-primary-600 to-accent-blue text-white rounded-xl hover:scale-105 active:scale-95 shadow-lg hover:shadow-xl flex items-center gap-2 cursor-pointer transition-all font-semibold"
            >
              <svg className="w-5 h-5 group-hover:rotate-90 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              Regular Peer
            </button>
          </div>
        }
      />

      <div className="p-8">
        {/* Filters */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 mb-6">
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
            {/* Network Filter */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Network</label>
              <SearchableSelect
                options={networkOptions}
                value={filterNetwork}
                onChange={setFilterNetwork}
                placeholder="All Networks"
              />
            </div>

            {/* Type Filter */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Type</label>
              <select
                value={filterType}
                onChange={(e) => setFilterType(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              >
                <option value="">All Types</option>
                <option value="jump">Jump</option>
                <option value="regular">Regular</option>
              </select>
            </div>

            {/* Status / Agent Filter (combined) */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Filter</label>
              <select
                value={filterStatus}
                onChange={(e) => setFilterStatus(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              >
                <option value="">All</option>
                <option value="agent-connected">Agent Connected</option>
                <option value="agent-disconnected">Agent Disconnected</option>
                <option value="agent-none">No Agent</option>
              </select>
            </div>

            {/* IP Filter */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">IP Address</label>
              <input
                type="text"
                value={filterIP}
                onChange={(e) => setFilterIP(e.target.value)}
                placeholder="Filter by IP..."
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400"
              />
            </div>
          </div>
          
          {/* Clear Filters */}
          {(filterNetwork || filterType || filterStatus || filterIP) && (
            <div className="mt-4 flex justify-end">
              <button
                onClick={() => {
                  setFilterNetwork('');
                  setFilterType('');
                  setFilterStatus('');
                  setFilterIP('');
                }}
                className="text-sm text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
              >
                Clear all filters
              </button>
            </div>
          )}
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-gray-500">Loading peers...</div>
          </div>
        ) : filteredPeers.length === 0 ? (
          <div className="bg-gradient-to-br from-white via-gray-50 to-white dark:from-gray-800 dark:via-gray-800/50 dark:to-gray-800 rounded-2xl border border-gray-200 dark:border-gray-700 p-16 text-center shadow-sm">
            <div className="inline-flex items-center justify-center w-20 h-20 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue mb-6">
              <FontAwesomeIcon icon={faServer} className="text-3xl text-white" />
            </div>
            <h3 className="text-xl font-bold text-gray-900 dark:text-white mb-2">No peers found</h3>
            <p className="text-gray-600 dark:text-gray-300">
              {peers.length === 0 
                ? 'Peers will appear here once they are created' 
                : 'Try adjusting your filters'}
            </p>
          </div>
        ) : (
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead className="bg-gray-50 dark:bg-gray-700">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Name</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Network</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Address</th>
                  {isAdmin && (
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Owner</th>
                  )}
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Status</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Type</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Actions</th>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                {filteredPeers.map((peer) => {
                  const hasActiveAgent = peer.session_status?.has_active_agent;
                  const isBlocked = blockedPeers.has(peer.id);

                  // "Connected" derives from the freshness of the last heartbeat
                  // (3-minute window — matches WireGuard's own ~180s threshold).
                  // For agent peers, has_active_agent already factors that in.
                  // For non-agent peers, we look at last_seen on their session
                  // (populated by jump peers reporting the peer's endpoint).
                  const lastSeenStr = peer.session_status?.current_session?.last_seen;
                  const lastSeenAgeMs = lastSeenStr ? now - new Date(lastSeenStr).getTime() : Infinity;
                  const isConnected = hasActiveAgent || lastSeenAgeMs <= 3 * 60 * 1000;

                  return (
                    <tr
                      key={peer.id}
                      onClick={() => handlePeerClick(peer)}
                      className={`hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer ${
                        isBlocked ? 'bg-orange-100 dark:bg-yellow-900/20' : ''
                      }`}
                    >
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="flex items-center">
                          <div className="inline-flex items-center justify-center w-10 h-10 rounded-xl bg-gradient-to-br from-primary-500 to-accent-blue mr-3">
                            <FontAwesomeIcon icon={peer.is_jump ? faRocket : peer.use_agent ? faServer : faLaptop} className="text-lg text-white" />
                          </div>
                          <div>
                            <div className="text-sm font-medium text-gray-900 dark:text-white">{peer.name}</div>
                          </div>
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                        {peer.network_name || peer.network_id}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-mono text-gray-900 dark:text-white">
                        <div>{peer.address}</div>
                        {peer.address_v6 && (
                          <div className="text-xs text-gray-500 dark:text-gray-400">{peer.address_v6}</div>
                        )}
                      </td>
                      {isAdmin && (
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                          {peer.owner_id ? (userMap.get(peer.owner_id) || peer.owner_id) : '-'}
                        </td>
                      )}
                      <td className="px-6 py-4 whitespace-nowrap text-sm">
                        {/* Status column: a single dot summarising the peer's overall state.
                            Priority (highest first): quarantined > pending_auth > connected/agent/auth > offline.
                              red    — quarantined (auth blocked)
                              orange — pending captive-portal auth (token outstanding)
                              green  — connected, OR agent up, OR authenticated
                              grey   — disconnected / no agent / no auth */}
                        {(() => {
                          // Status-dot logic for a peer.  Priority order matches the
                          // user-visible meaning of the colour:
                          //
                          //   red    — quarantined (peer is BLOCKED, auth failures
                          //            exceeded threshold; admin reset required)
                          //   orange — action needed by the user (a captive-portal
                          //            sign-in is in progress, OR the WG tunnel is
                          //            up but the peer hasn't authenticated yet so
                          //            no actual network access is granted)
                          //   green  — fully usable (connected AND authenticated,
                          //            OR a jump peer for which captive-portal
                          //            auth doesn't apply)
                          //   grey   — offline / no active session
                          //
                          // The key change vs a naive "connected = green" is that a
                          // non-jump peer with a fresh WG handshake but EMPTY
                          // captive_portal_state is shown orange, not green.
                          // Without captive-portal auth the iptables WIRETY_JUMP
                          // chain drops all forwarded traffic from this peer — the
                          // tunnel is up but the firewall blocks every actual
                          // request.  Showing green there gives the user a false
                          // sense that the device works.
                          //
                          // Visual cues:
                          //   - The label appears on hover only (opacity transition,
                          //     not display: none, so the row layout doesn't shift
                          //     when the user moves the cursor across it).
                          //   - The dot pulses ONLY for pending_auth — a sign-in
                          //     that's actively in flight.  Static orange ("not
                          //     authenticated") doesn't pulse: the user knows what
                          //     to do, no need to draw the eye.
                          const portalState = peer.session_status?.captive_portal_state;
                          const isOnline = isConnected || hasActiveAgent;
                          let color = 'bg-gray-400';
                          let label = 'Offline';
                          let pulse = false;

                          if (portalState === 'quarantined') {
                            color = 'bg-red-500';
                            label = 'Quarantined';
                          } else if (portalState === 'pending_auth') {
                            color = 'bg-orange-500';
                            label = 'Pending auth';
                            pulse = true;
                          } else if (!isOnline) {
                            color = 'bg-gray-400';
                            label = peer.use_agent ? 'Agent offline' : 'Offline';
                          } else if (peer.is_jump) {
                            // Jump peers don't authenticate to themselves — being
                            // online is sufficient.
                            color = 'bg-green-500';
                            label = 'Connected';
                          } else if (portalState === 'authenticated') {
                            color = 'bg-green-500';
                            label = 'Connected';
                          } else {
                            // Online + non-jump + no auth state = WG tunnel is up but
                            // the peer hasn't completed captive-portal auth.  Network
                            // access is blocked at the firewall.
                            color = 'bg-orange-500';
                            label = 'Not authenticated';
                          }

                          return (
                            <div className="group flex items-center gap-2 cursor-default">
                              <span className={`w-2.5 h-2.5 rounded-full ${color} ${pulse ? 'animate-pulse' : ''}`} />
                              <span className="text-xs text-gray-600 dark:text-gray-300 opacity-0 group-hover:opacity-100 transition-opacity duration-150 whitespace-nowrap">
                                {label}
                              </span>
                            </div>
                          );
                        })()}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm">
                        {/* Type column at end: Jump or Regular */}
                        {peer.is_jump ? (
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200">Jump</span>
                        ) : (
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">Regular</span>
                        )}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm">
                        <div className="flex items-center gap-2">
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              if (!peer.network_id) return;
                              setEditingPeer(peer);
                              setSelectedNetworkId(peer.network_id);
                              if (peer.is_jump) {
                                setIsJumpModalOpen(true);
                              } else {
                                setIsRegularModalOpen(true);
                              }
                            }}
                            className="text-primary-600 hover:text-primary-800 dark:text-primary-400 dark:hover:text-primary-300 transition-colors"
                            title="Edit peer"
                          >
                            <FontAwesomeIcon icon={faPencil} />
                          </button>
                          <button
                            onClick={async (e) => {
                              e.stopPropagation();
                              if (!peer.network_id) return;
                              if (confirm(`Are you sure you want to delete peer "${peer.name}"?`)) {
                                try {
                                  await api.deletePeer(peer.network_id, peer.id);
                                  refetchPeers();
                                } catch (error) {
                                  console.error('Failed to delete peer:', error);
                                  alert('Failed to delete peer');
                                }
                              }
                            }}
                            className="text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300 transition-colors"
                            title="Delete peer"
                          >
                            <FontAwesomeIcon icon={faTrash} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="mt-8 flex items-center justify-between">
            <div className="text-sm text-gray-500">
              Page {page} of {totalPages}
            </div>
            <div className="flex gap-2">
              <button
                onClick={() => setPage(Math.max(1, page - 1))}
                disabled={page === 1}
                className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Previous
              </button>
              <button
                onClick={() => setPage(Math.min(totalPages, page + 1))}
                disabled={page >= totalPages}
                className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Next
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Peer Modals */}
      <JumpPeerModal
        isOpen={isJumpModalOpen}
        onClose={handleModalClose}
        onSuccess={handleModalSuccess}
        networkId={selectedNetworkId}
        networks={networks}
        peer={editingPeer}
        users={users}
      />
      <RegularPeerModal
        isOpen={isRegularModalOpen}
        onClose={handleModalClose}
        onSuccess={handleModalSuccess}
        networkId={selectedNetworkId}
        networks={networks}
        peer={editingPeer}
        users={users}
      />

      {/* Detail Modal */}
      <PeerDetailModal
        isOpen={isDetailModalOpen}
        onClose={() => setIsDetailModalOpen(false)}
        peer={selectedPeer}
        onUpdate={refetchPeers}
        users={users}
      />
    </div>
  );
}
