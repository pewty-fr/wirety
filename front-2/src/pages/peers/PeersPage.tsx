import { useState, useEffect, useRef } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faServer, faLaptop, faTriangleExclamation } from '@fortawesome/free-solid-svg-icons';
import PageHeader from '../../components/PageHeader';
import JumpPeerModal from '../../components/JumpPeerModal';
import RegularPeerModal from '../../components/RegularPeerModal';
import PeerDetailModal from '../../components/PeerDetailModal';
import api from '../../api/client';
import type { Peer, Network, ACL } from '../../types';

export default function PeersPage() {
  const [peers, setPeers] = useState<Peer[]>([]);
  const [networks, setNetworks] = useState<Network[]>([]);
  const [networkACLs, setNetworkACLs] = useState<Record<string, ACL>>({});
  const [blockedPeers, setBlockedPeers] = useState<Set<string>>(new Set());
  const [incidentPeerIds, setIncidentPeerIds] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [isJumpModalOpen, setIsJumpModalOpen] = useState(false);
  const [isRegularModalOpen, setIsRegularModalOpen] = useState(false);
  const [editingPeer, setEditingPeer] = useState<Peer | null>(null);
  const [selectedNetworkId, setSelectedNetworkId] = useState<string>('');
  const [selectedPeer, setSelectedPeer] = useState<Peer | null>(null);
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false);
  
  // Filters
  const [filterNetwork, setFilterNetwork] = useState<string>('');
  const [filterType, setFilterType] = useState<string>('');
  const [filterStatus, setFilterStatus] = useState<string>('');
  const [filterIP, setFilterIP] = useState<string>('');

  const pageSize = 20;

  // Guard refs to prevent duplicate calls under React StrictMode in development
  const networksLoadedRef = useRef(false);
  const peersLoadedRef = useRef(false);
  const aclsLoadedRef = useRef(false);
  const incidentsLoadedRef = useRef(false);

  useEffect(() => {
    if (networksLoadedRef.current) return;
    networksLoadedRef.current = true;
    loadNetworks();
  }, []);

  useEffect(() => {
    if (!networks.length || aclsLoadedRef.current) return;
    aclsLoadedRef.current = true;
    loadACLs();
  }, [networks]);

  useEffect(() => {
    if (incidentsLoadedRef.current) return;
    incidentsLoadedRef.current = true;
    loadIncidents();
  }, []);

  useEffect(() => {
    if (peersLoadedRef.current && page === 1) {
      // prevent duplicate initial load; allow subsequent page changes
      return;
    }
    peersLoadedRef.current = true;
    loadPeers();
  }, [page]);

  useEffect(() => {
    updateBlockedPeers(peers);
  }, [networkACLs, peers]);

  const loadNetworks = async () => {
    try {
      const response = await api.getNetworks(1, 100);
      setNetworks(response.data || []);
    } catch (error) {
      console.error('Failed to load networks:', error);
    }
  };

  const loadPeers = async () => {
    setLoading(true);
    try {
      const response = await api.getAllPeers(page, pageSize);
      setPeers(response.peers || []);
      setTotal(response.total || 0);
    } catch (error) {
      console.error('Failed to load peers:', error);
      setPeers([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  const loadACLs = async () => {
    try {
      const acls: Record<string, ACL> = {};
      for (const network of networks) {
        try {
          const acl = await api.getACL(network.id);
          acls[network.id] = acl;
        } catch (error) {
          console.warn(`Failed to load ACL for network ${network.id}:`, error);
        }
      }
      setNetworkACLs(acls);
    } catch (error) {
      console.error('Failed to load ACLs:', error);
    }
  };

  const loadIncidents = async () => {
    try {
      const response = await api.getSecurityIncidents(1, 200, false);
      const incidentPeers = new Set<string>();
      response.data.forEach(inc => {
        if (!inc.resolved && inc.peer_id) {
          incidentPeers.add(inc.peer_id);
        }
      });
      setIncidentPeerIds(incidentPeers);
    } catch (error) {
      console.error('Failed to load security incidents:', error);
    }
  };

  const updateBlockedPeers = (peersList: Peer[]) => {
    const blocked = new Set<string>();
    peersList.forEach(peer => {
      if (peer.network_id && networkACLs[peer.network_id]) {
        const acl = networkACLs[peer.network_id];
        if (acl.enabled && acl.blocked_peers && acl.blocked_peers[peer.id]) {
          blocked.add(peer.id);
        }
      }
    });
    setBlockedPeers(blocked);
  };

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
    loadPeers();
  };

  // Apply filters to peers
  const filteredPeers = peers.filter(peer => {
    // Network filter
    if (filterNetwork && peer.network_id !== filterNetwork) return false;

    // Type filter (only jump or regular)
    if (filterType === 'jump' && !peer.is_jump) return false;
    if (filterType === 'regular' && peer.is_jump) return false;

    // Status filter (wireguard up/down)
    const wgUp = !!peer.session_status?.current_session?.reported_endpoint;
    if (filterStatus === 'wg-up' && !wgUp) return false;
    if (filterStatus === 'wg-down' && wgUp) return false;

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
              className="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 flex items-center gap-2 cursor-pointer transition-colors"
            >
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              Jump Peer
            </button>
            <button
              onClick={handleCreateRegular}
              className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 flex items-center gap-2 cursor-pointer transition-colors"
            >
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
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
              <select
                value={filterNetwork}
                onChange={(e) => setFilterNetwork(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              >
                <option value="">All Networks</option>
                {networks.map(network => (
                  <option key={network.id} value={network.id}>{network.name}</option>
                ))}
              </select>
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
                <option value="wg-up">WireGuard Up</option>
                <option value="wg-down">WireGuard Down</option>
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
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-12 text-center">
            <div className="text-gray-400 dark:text-gray-500 text-5xl mb-4">
              <FontAwesomeIcon icon={faServer} />
            </div>
            <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">No peers found</h3>
            <p className="text-gray-500 dark:text-gray-400">
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
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Status</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Name</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Network</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Address</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Agent</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Type</th>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                {filteredPeers.map((peer) => {
                  const hasActiveAgent = peer.session_status?.has_active_agent;
                  const isPeerUp = !!peer.session_status?.current_session?.reported_endpoint;
                  const isBlocked = blockedPeers.has(peer.id);
                  const hasIncident = incidentPeerIds.has(peer.id);
                  
                  return (
                    <tr
                      key={peer.id}
                      onClick={() => handlePeerClick(peer)}
                      className={`hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer ${
                        (hasIncident || isBlocked) ? 'bg-orange-100 dark:bg-yellow-900/20' : ''
                      }`}
                    >
                      <td className="px-4 py-4 whitespace-nowrap">
                        {/* Status indicators: warning if blocked, otherwise WireGuard status */}
                        <div className="flex items-center gap-1">
                          {(hasIncident || isBlocked) && (
                            <FontAwesomeIcon icon={faTriangleExclamation} className="text-primary-600 dark:text-primary-400" title="Security incident" />
                          )}
                          <span className={`w-3 h-3 rounded-full ${isPeerUp ? 'bg-green-500' : 'bg-red-500'}`} title={isPeerUp ? 'WireGuard Up' : 'WireGuard Down'}></span>
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="flex items-center">
                          <span className="text-2xl mr-3">
                            <FontAwesomeIcon icon={peer.use_agent ? faServer : faLaptop} className="text-gray-600 dark:text-gray-400" />
                          </span>
                          <div>
                            <div className={`text-sm font-medium ${hasIncident ? 'text-primary-600 dark:text-primary-400' : 'text-gray-900 dark:text-white'}`}>{peer.name}</div>
                            <div className={`text-xs ${hasIncident ? 'text-primary-500 dark:text-primary-300' : 'text-gray-500 dark:text-gray-400'}`}>{peer.endpoint || 'No endpoint'}</div>
                          </div>
                        </div>
                      </td>
                      <td className={`px-6 py-4 whitespace-nowrap text-sm ${hasIncident ? 'text-primary-600 dark:text-primary-400' : 'text-gray-900 dark:text-white'}`}>
                        {peer.network_name || peer.network_id}
                      </td>
                      <td className={`px-6 py-4 whitespace-nowrap text-sm font-mono ${hasIncident ? 'text-primary-600 dark:text-primary-400' : 'text-gray-900 dark:text-white'}`}>
                        {peer.address}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm">
                        {/* Agent column: dot badge only */}
                        <div className="flex items-center">
                          <span className={`w-3 h-3 rounded-full ${
                            !peer.use_agent ? 'bg-gray-400' : (hasActiveAgent ? 'bg-green-500' : 'bg-red-500')
                          }`}></span>
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm">
                        {/* Type column at end: Jump or Regular */}
                        {peer.is_jump ? (
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200">Jump</span>
                        ) : (
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">Regular</span>
                        )}
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
      />
      <RegularPeerModal
        isOpen={isRegularModalOpen}
        onClose={handleModalClose}
        onSuccess={handleModalSuccess}
        networkId={selectedNetworkId}
        networks={networks}
        peer={editingPeer}
      />

      {/* Detail Modal */}
      <PeerDetailModal
        isOpen={isDetailModalOpen}
        onClose={() => setIsDetailModalOpen(false)}
        peer={selectedPeer}
        onUpdate={loadPeers}
      />
    </div>
  );
}
