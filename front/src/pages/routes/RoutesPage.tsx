import { useState, useEffect, useCallback, useMemo } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faRoute, faPencil, faTrash, faGlobe } from '@fortawesome/free-solid-svg-icons';
import PageHeader from '../../components/PageHeader';
import SearchableSelect from '../../components/SearchableSelect';
import api from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';
import type { Route, DNSMapping, Network, Peer } from '../../types';

export default function RoutesPage() {
  const { user } = useAuth();
  const [networks, setNetworks] = useState<Network[]>([]);
  const [selectedNetworkId, setSelectedNetworkId] = useState<string>('');
  const [routes, setRoutes] = useState<Route[]>([]);
  const [jumpPeers, setJumpPeers] = useState<Peer[]>([]);
  const [loading, setLoading] = useState(true);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingRoute, setEditingRoute] = useState<Route | null>(null);
  const [selectedRoute, setSelectedRoute] = useState<Route | null>(null);
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false);

  const isAdmin = user?.role === 'administrator';

  // Create a map of jump peer IDs to names
  const jumpPeerMap = useMemo(() => {
    const map = new Map<string, string>();
    jumpPeers.forEach(peer => map.set(peer.id, peer.name));
    return map;
  }, [jumpPeers]);

  useEffect(() => {
    const loadNetworks = async () => {
      try {
        const response = await api.getNetworks(1, 100);
        setNetworks(response.data || []);
        if (response.data && response.data.length > 0) {
          setSelectedNetworkId(response.data[0].id);
        }
      } catch (error) {
        console.error('Failed to load networks:', error);
      }
    };
    void loadNetworks();
  }, []);

  const loadRoutes = useCallback(async () => {
    if (!selectedNetworkId) {
      setRoutes([]);
      setJumpPeers([]);
      setLoading(false);
      return;
    }

    setLoading(true);
    try {
      const [routesData, peersData] = await Promise.all([
        api.getRoutes(selectedNetworkId),
        api.getAllNetworkPeers(selectedNetworkId)
      ]);
      setRoutes(routesData || []);
      setJumpPeers((peersData || []).filter((p: Peer) => p.is_jump));
    } catch (error) {
      console.error('Failed to load routes:', error);
      setRoutes([]);
    } finally {
      setLoading(false);
    }
  }, [selectedNetworkId]);

  useEffect(() => {
    void loadRoutes();
  }, [loadRoutes]);

  const handleRouteClick = (route: Route) => {
    setSelectedRoute(route);
    setIsDetailModalOpen(true);
  };

  const handleCreate = () => {
    setEditingRoute(null);
    setIsModalOpen(true);
  };

  const handleEdit = (route: Route, e: React.MouseEvent) => {
    e.stopPropagation();
    setEditingRoute(route);
    setIsModalOpen(true);
  };

  const handleDelete = async (route: Route, e: React.MouseEvent) => {
    e.stopPropagation();
    if (confirm(`Are you sure you want to delete route "${route.name}"?`)) {
      try {
        await api.deleteRoute(selectedNetworkId, route.id);
        loadRoutes();
      } catch (error) {
        console.error('Failed to delete route:', error);
        alert('Failed to delete route');
      }
    }
  };

  if (!isAdmin) {
    return (
      <div className="p-8">
        <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4">
          <p className="text-yellow-800 dark:text-yellow-200">
            You need administrator privileges to manage routes.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div>
      <PageHeader 
        title="Routes" 
        subtitle={`${routes.length} route${routes.length !== 1 ? 's' : ''} in selected network`}
        action={
          selectedNetworkId ? (
            <button
              onClick={handleCreate}
              className="px-4 py-2.5 bg-gradient-to-r from-primary-600 to-accent-blue text-white rounded-xl hover:scale-105 active:scale-95 shadow-lg hover:shadow-xl flex items-center gap-2 cursor-pointer transition-all font-semibold"
            >
              <svg className="w-5 h-5 group-hover:rotate-90 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              Route
            </button>
          ) : undefined
        }
      />

      <div className="p-8">
        {/* Network Filter */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 mb-6">
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Network</label>
              <SearchableSelect
                options={useMemo(() => networks.map(network => ({
                  value: network.id,
                  label: `${network.name} (${network.cidr})`
                })), [networks])}
                value={selectedNetworkId}
                onChange={setSelectedNetworkId}
                placeholder="Select a network..."
              />
            </div>
          </div>
        </div>

        {!selectedNetworkId ? (
          <div className="bg-gradient-to-br from-white via-gray-50 to-white dark:from-gray-800 dark:via-gray-800/50 dark:to-gray-800 rounded-2xl border border-gray-200 dark:border-gray-700 p-16 text-center shadow-sm">
            <div className="inline-flex items-center justify-center w-20 h-20 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue mb-6">
              <FontAwesomeIcon icon={faRoute} className="text-3xl text-white" />
            </div>
            <h3 className="text-xl font-bold text-gray-900 dark:text-white mb-2">Select a network</h3>
            <p className="text-gray-600 dark:text-gray-300">
              Choose a network from the dropdown above to view and manage routes
            </p>
          </div>
        ) : loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-gray-500">Loading routes...</div>
          </div>
        ) : routes.length === 0 ? (
          <div className="bg-gradient-to-br from-white via-gray-50 to-white dark:from-gray-800 dark:via-gray-800/50 dark:to-gray-800 rounded-2xl border border-gray-200 dark:border-gray-700 p-16 text-center shadow-sm">
            <div className="inline-flex items-center justify-center w-20 h-20 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue mb-6">
              <FontAwesomeIcon icon={faRoute} className="text-3xl text-white" />
            </div>
            <h3 className="text-xl font-bold text-gray-900 dark:text-white mb-2">No routes found</h3>
            <p className="text-gray-600 dark:text-gray-300">
              Get started by creating your first route
            </p>
          </div>
        ) : (
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead className="bg-gray-50 dark:bg-gray-700">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Name</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Destination CIDR</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Jump Peer</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Domain Suffix</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Created</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Actions</th>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                {routes.map((route) => (
                  <tr
                    key={route.id}
                    onClick={() => handleRouteClick(route)}
                    className="hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer"
                  >
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex items-center">
                        <div className="inline-flex items-center justify-center w-10 h-10 rounded-xl bg-gradient-to-br from-primary-500 to-accent-blue mr-3">
                          <FontAwesomeIcon icon={faRoute} className="text-lg text-white" />
                        </div>
                        <div className="text-sm font-medium text-gray-900 dark:text-white">{route.name}</div>
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm font-mono text-gray-900 dark:text-white">
                      {route.destination_cidr}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                      {jumpPeerMap.get(route.jump_peer_id) || route.jump_peer_id || '-'}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                      {route.domain_suffix || 'internal'}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                      {new Date(route.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm">
                      <div className="flex items-center gap-2">
                        <button
                          onClick={(e) => handleEdit(route, e)}
                          className="text-primary-600 hover:text-primary-800 dark:text-primary-400 dark:hover:text-primary-300 transition-colors"
                          title="Edit route"
                        >
                          <FontAwesomeIcon icon={faPencil} />
                        </button>
                        <button
                          onClick={(e) => handleDelete(route, e)}
                          className="text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300 transition-colors"
                          title="Delete route"
                        >
                          <FontAwesomeIcon icon={faTrash} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <RouteModal
        isOpen={isModalOpen}
        onClose={() => {
          setIsModalOpen(false);
          setEditingRoute(null);
        }}
        onSuccess={loadRoutes}
        networkId={selectedNetworkId}
        route={editingRoute}
      />

      <RouteDetailModal
        isOpen={isDetailModalOpen}
        onClose={() => {
          setIsDetailModalOpen(false);
          setSelectedRoute(null);
        }}
        route={selectedRoute}
        networkId={selectedNetworkId}
        onUpdate={loadRoutes}
      />
    </div>
  );
}

// Route Modal Component
function RouteModal({
  isOpen,
  onClose,
  onSuccess,
  networkId,
  route,
}: {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  networkId: string;
  route: Route | null;
}) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [destinationCidr, setDestinationCidr] = useState('');
  const [jumpPeerId, setJumpPeerId] = useState('');
  const [domainSuffix, setDomainSuffix] = useState('internal');
  const [jumpPeers, setJumpPeers] = useState<Peer[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (isOpen && networkId) {
      loadJumpPeers();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, networkId]);

  useEffect(() => {
    if (route) {
      setName(route.name);
      setDescription(route.description || '');
      setDestinationCidr(route.destination_cidr);
      setJumpPeerId(route.jump_peer_id);
      setDomainSuffix(route.domain_suffix || 'internal');
    } else {
      setName('');
      setDescription('');
      setDestinationCidr('');
      setJumpPeerId('');
      setDomainSuffix('internal');
    }
  }, [route]);

  const loadJumpPeers = async () => {
    try {
      const peers = await api.getAllNetworkPeers(networkId);
      setJumpPeers(peers.filter(p => p.is_jump));
    } catch (error) {
      console.error('Failed to load jump peers:', error);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);

    try {
      if (route) {
        await api.updateRoute(networkId, route.id, {
          name,
          description,
          destination_cidr: destinationCidr,
          jump_peer_id: jumpPeerId,
          domain_suffix: domainSuffix,
        });
      } else {
        await api.createRoute(networkId, {
          name,
          description,
          destination_cidr: destinationCidr,
          jump_peer_id: jumpPeerId,
          domain_suffix: domainSuffix,
        });
      }
      onSuccess();
      onClose();
    } catch (error) {
      console.error('Failed to save route:', error);
      alert('Failed to save route');
    } finally {
      setLoading(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      {/* Backdrop with blur */}
      <div 
        className="fixed inset-0 backdrop-blur-sm bg-gradient-to-br from-primary-500/10 to-accent-blue/10 dark:from-black/50 dark:to-primary-900/50 transition-all"
        onClick={onClose}
      />
      
      {/* Modal */}
      <div className="flex min-h-full items-center justify-center p-4">
        <div 
          className="relative bg-gradient-to-br from-white to-gray-50 dark:from-dark dark:to-gray-800 rounded-lg shadow-2xl w-full max-w-md transform transition-all border-2 border-primary-300 dark:border-primary-700"
          onClick={(e) => e.stopPropagation()}
        >
          <div className="p-6">
        <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-4">
          {route ? 'Edit Route' : 'Create Route'}
        </h2>
        <form onSubmit={handleSubmit}>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Name *
              </label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Description
              </label>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={2}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Destination CIDR *
              </label>
              <input
                type="text"
                value={destinationCidr}
                onChange={(e) => setDestinationCidr(e.target.value)}
                placeholder="10.0.0.0/24"
                required
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white font-mono"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Jump Peer *
              </label>
              <select
                value={jumpPeerId}
                onChange={(e) => setJumpPeerId(e.target.value)}
                required
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              >
                <option value="">Select a jump peer...</option>
                {jumpPeers.map((peer) => (
                  <option key={peer.id} value={peer.id}>
                    {peer.name}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Domain Suffix
              </label>
              <input
                type="text"
                value={domainSuffix}
                onChange={(e) => setDomainSuffix(e.target.value)}
                placeholder="internal"
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              />
            </div>
          </div>
          <div className="mt-6 flex justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              className="px-4 py-2 text-sm font-medium text-white bg-gradient-to-r from-primary-600 to-accent-blue rounded-lg hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? 'Saving...' : route ? 'Update' : 'Create'}
            </button>
          </div>
        </form>
        </div>
      </div>
      </div>
    </div>
  );
}

// Route Detail Modal Component
function RouteDetailModal({
  isOpen,
  onClose,
  route,
  networkId,
  onUpdate,
}: {
  isOpen: boolean;
  onClose: () => void;
  route: Route | null;
  networkId: string;
  onUpdate: () => void;
}) {
  const [dnsMappings, setDnsMappings] = useState<DNSMapping[]>([]);
  const [isAddDNSModalOpen, setIsAddDNSModalOpen] = useState(false);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (isOpen && route && networkId) {
      loadDNSMappings();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, route, networkId]);

  const loadDNSMappings = async () => {
    if (!route || !networkId) return;

    setLoading(true);
    try {
      const data = await api.getDNSMappings(networkId, route.id);
      setDnsMappings(data || []);
    } catch (error) {
      console.error('Failed to load DNS mappings:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleAddDNS = async (name: string, ipAddress: string) => {
    if (!route || !networkId) return;
    try {
      await api.createDNSMapping(networkId, route.id, { name, ip_address: ipAddress });
      await loadDNSMappings();
      onUpdate();
      setIsAddDNSModalOpen(false);
    } catch (error) {
      console.error('Failed to add DNS mapping:', error);
      alert('Failed to add DNS mapping');
    }
  };

  const handleDeleteDNS = async (dnsId: string) => {
    if (!route || !networkId) return;
    try {
      await api.deleteDNSMapping(networkId, route.id, dnsId);
      await loadDNSMappings();
      onUpdate();
    } catch (error) {
      console.error('Failed to delete DNS mapping:', error);
      alert('Failed to delete DNS mapping');
    }
  };

  if (!isOpen || !route) return null;

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      {/* Backdrop with blur */}
      <div 
        className="fixed inset-0 backdrop-blur-sm bg-gradient-to-br from-primary-500/10 to-accent-blue/10 dark:from-black/50 dark:to-primary-900/50 transition-all"
        onClick={onClose}
      />
      
      {/* Modal */}
      <div className="flex min-h-full items-center justify-center p-4">
        <div 
          className="relative bg-gradient-to-br from-white to-gray-50 dark:from-dark dark:to-gray-800 rounded-lg shadow-2xl w-full max-w-4xl transform transition-all border-2 border-primary-300 dark:border-primary-700 max-h-[90vh] overflow-y-auto"
          onClick={(e) => e.stopPropagation()}
        >
        <div className="p-6 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-start gap-4">
            <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue">
              <FontAwesomeIcon icon={faRoute} className="text-2xl text-white" />
            </div>
            <div>
              <h2 className="text-2xl font-bold text-gray-900 dark:text-white">{route.name}</h2>
              <p className="text-sm text-gray-600 dark:text-gray-300 mt-1">ID: {route.id}</p>
              {route.description && (
                <p className="text-gray-600 dark:text-gray-400 mt-1">{route.description}</p>
              )}
              <div className="mt-3 space-y-1">
                <p className="text-sm text-gray-600 dark:text-gray-400">
                  <span className="font-medium">Destination:</span> <span className="font-mono">{route.destination_cidr}</span>
                </p>
                <p className="text-sm text-gray-600 dark:text-gray-400">
                  <span className="font-medium">Domain Suffix:</span> {route.domain_suffix || 'internal'}
                </p>
              </div>
            </div>
          </div>
        </div>

        {loading ? (
          <div className="p-6 text-center text-gray-500">Loading DNS mappings...</div>
        ) : (
          <div className="p-6">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                <FontAwesomeIcon icon={faGlobe} />
                DNS Mappings ({dnsMappings.length})
              </h3>
              <button
                onClick={() => setIsAddDNSModalOpen(true)}
                className="px-3 py-1.5 text-sm bg-gradient-to-r from-primary-600 to-accent-blue text-white rounded-lg hover:scale-105"
              >
                Add DNS Mapping
              </button>
            </div>

            {dnsMappings.length === 0 ? (
              <div className="text-center py-8 text-gray-500">
                No DNS mappings defined. Add mappings to resolve custom domains.
              </div>
            ) : (
              <div className="space-y-2">
                {dnsMappings.map((dns) => (
                  <div key={dns.id} className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                    <div>
                      <div className="text-sm font-medium text-gray-900 dark:text-white">
                        {dns.name}.{route.name}.{route.domain_suffix || 'internal'}
                      </div>
                      <div className="text-xs text-gray-500 dark:text-gray-400 font-mono mt-1">
                        → {dns.ip_address}
                      </div>
                    </div>
                    <button
                      onClick={() => handleDeleteDNS(dns.id)}
                      className="text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300"
                      title="Delete DNS mapping"
                    >
                      <FontAwesomeIcon icon={faTrash} />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        <div className="p-6 border-t border-gray-200 dark:border-gray-700 flex justify-end">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600"
          >
            Close
          </button>
        </div>
        </div>
      </div>
      
      <AddDNSModal
        isOpen={isAddDNSModalOpen}
        onClose={() => setIsAddDNSModalOpen(false)}
        onAdd={handleAddDNS}
        routeCidr={route.destination_cidr}
      />
    </div>
  );
}

// Add DNS Modal Component
function AddDNSModal({
  isOpen,
  onClose,
  onAdd,
  routeCidr,
}: {
  isOpen: boolean;
  onClose: () => void;
  onAdd: (name: string, ipAddress: string) => void;
  routeCidr: string;
}) {
  const [name, setName] = useState('');
  const [ipAddress, setIpAddress] = useState('');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onAdd(name, ipAddress);
    setName('');
    setIpAddress('');
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-[60] overflow-y-auto">
      {/* Backdrop with blur */}
      <div 
        className="fixed inset-0 backdrop-blur-sm bg-gradient-to-br from-primary-500/10 to-accent-blue/10 dark:from-black/50 dark:to-primary-900/50 transition-all"
        onClick={onClose}
      />
      
      {/* Modal */}
      <div className="flex min-h-full items-center justify-center p-4">
        <div 
          className="relative bg-gradient-to-br from-white to-gray-50 dark:from-dark dark:to-gray-800 rounded-lg shadow-2xl w-full max-w-md transform transition-all border-2 border-primary-300 dark:border-primary-700"
          onClick={(e) => e.stopPropagation()}
        >
          <div className="p-6">
        <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-4">Add DNS Mapping</h2>
        <form onSubmit={handleSubmit}>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Name *
              </label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="server1"
                required
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                IP Address *
              </label>
              <input
                type="text"
                value={ipAddress}
                onChange={(e) => setIpAddress(e.target.value)}
                placeholder="10.0.0.10"
                required
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white font-mono"
              />
              <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                Must be within route CIDR: {routeCidr}
              </p>
            </div>
          </div>
          <div className="mt-6 flex justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600"
            >
              Cancel
            </button>
            <button
              type="submit"
              className="px-4 py-2 text-sm font-medium text-white bg-gradient-to-r from-primary-600 to-accent-blue rounded-lg hover:scale-105"
            >
              Add Mapping
            </button>
          </div>
        </form>
        </div>
      </div>
      </div>
    </div>
  );
}
