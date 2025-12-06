import { useState, useEffect, useCallback, useMemo } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faRoute, faPencil, faTrash, faGlobe, faUsers } from '@fortawesome/free-solid-svg-icons';
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
        // Don't auto-select first network - let user choose
      } catch (error) {
        console.error('Failed to load networks:', error);
      }
    };
    void loadNetworks();
  }, []);

  const loadRoutes = useCallback(async () => {
    setLoading(true);
    try {
      let routesData;
      let peersData = [];
      
      if (selectedNetworkId) {
        // Load routes and peers for specific network
        [routesData, peersData] = await Promise.all([
          api.getRoutes(selectedNetworkId),
          api.getAllNetworkPeers(selectedNetworkId)
        ]);
      } else {
        // Load all routes from all networks
        routesData = await api.getAllRoutes();
        // For cross-network view, we don't need jump peers data
      }
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
    // Open the edit modal directly (which has all the tabs and functionality)
    setEditingRoute(route);
    setIsModalOpen(true);
  };

  const handleCreate = () => {
    setEditingRoute(null);
    setIsModalOpen(true);
  };

  const handleEdit = (route: Route, e?: React.MouseEvent) => {
    if (e) e.stopPropagation();
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
        subtitle={`${routes.length} route${routes.length !== 1 ? 's' : ''} ${selectedNetworkId ? 'in selected network' : 'across all networks'}`}
        action={
          <button
            onClick={handleCreate}
            className="px-4 py-2.5 bg-gradient-to-r from-primary-600 to-accent-blue text-white rounded-xl hover:scale-105 active:scale-95 shadow-lg hover:shadow-xl flex items-center gap-2 cursor-pointer transition-all font-semibold"
          >
            <svg className="w-5 h-5 group-hover:rotate-90 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
            Route
          </button>
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

        {loading ? (
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
                  {!selectedNetworkId && (
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Network</th>
                  )}
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
                    {!selectedNetworkId && (
                      <td className="px-6 py-4 text-sm text-gray-900 dark:text-white">
                        {route.network_name || '-'}
                      </td>
                    )}
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
        networks={networks}
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
  networks,
}: {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  networkId: string;
  route: Route | null;
  networks: Network[];
}) {
  const [activeTab, setActiveTab] = useState<'details' | 'dns' | 'groups'>('details');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [destinationCidr, setDestinationCidr] = useState('');
  const [jumpPeerId, setJumpPeerId] = useState('');
  const [domainSuffix, setDomainSuffix] = useState('internal');
  const [selectedNetworkId, setSelectedNetworkId] = useState<string>(networkId);
  const [jumpPeers, setJumpPeers] = useState<Peer[]>([]);
  const [loading, setLoading] = useState(false);
  
  // DNS mappings management
  const [dnsMappings, setDnsMappings] = useState<DNSMapping[]>([]);
  const [stagedDnsMappings, setStagedDnsMappings] = useState<Array<{ name: string; ip_address: string }>>([]);
  const [isAddDNSModalOpen, setIsAddDNSModalOpen] = useState(false);
  
  // Groups management
  const [attachedGroups, setAttachedGroups] = useState<any[]>([]);
  const [availableGroups, setAvailableGroups] = useState<any[]>([]);
  const [stagedGroupIds, setStagedGroupIds] = useState<string[]>([]);

  // Individual edit modes
  const [isEditingName, setIsEditingName] = useState(false);
  const [isEditingDescription, setIsEditingDescription] = useState(false);
  const [isEditingDestinationCidr, setIsEditingDestinationCidr] = useState(false);
  const [isEditingJumpPeer, setIsEditingJumpPeer] = useState(false);
  const [isEditingDomainSuffix, setIsEditingDomainSuffix] = useState(false);

  useEffect(() => {
    if (isOpen && selectedNetworkId) {
      loadJumpPeers();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, selectedNetworkId]);

  useEffect(() => {
    if (route) {
      setName(route.name);
      setDescription(route.description || '');
      setDestinationCidr(route.destination_cidr);
      setJumpPeerId(route.jump_peer_id);
      setDomainSuffix(route.domain_suffix || 'internal');
      setSelectedNetworkId(networkId);
      setStagedDnsMappings([]);
      setStagedGroupIds([]);
      // Reset edit modes
      setIsEditingName(false);
      setIsEditingDescription(false);
      setIsEditingDestinationCidr(false);
      setIsEditingJumpPeer(false);
      setIsEditingDomainSuffix(false);
      loadDNSMappings();
      loadAttachments();
    } else {
      setName('');
      setDescription('');
      setDestinationCidr('');
      setJumpPeerId('');
      setDomainSuffix('internal');
      setSelectedNetworkId(networkId || '');
      setDnsMappings([]);
      setStagedDnsMappings([]);
      setAttachedGroups([]);
      setStagedGroupIds([]);
      // Reset edit modes
      setIsEditingName(false);
      setIsEditingDescription(false);
      setIsEditingDestinationCidr(false);
      setIsEditingJumpPeer(false);
      setIsEditingDomainSuffix(false);
      if (selectedNetworkId) {
        loadAvailableItems();
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [route, isOpen]);

  // Load available items when network changes in creation mode
  useEffect(() => {
    if (!route && selectedNetworkId && isOpen) {
      loadAvailableItems();
      loadJumpPeers();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedNetworkId, route, isOpen]);

  const loadJumpPeers = async () => {
    if (!selectedNetworkId) return;
    try {
      const peers = await api.getAllNetworkPeers(selectedNetworkId);
      setJumpPeers(peers.filter(p => p.is_jump));
    } catch (error) {
      console.error('Failed to load jump peers:', error);
    }
  };

  const loadDNSMappings = async () => {
    if (!route || !selectedNetworkId) return;

    try {
      const data = await api.getDNSMappings(selectedNetworkId, route.id);
      setDnsMappings(data || []);
    } catch (error) {
      console.error('Failed to load DNS mappings:', error);
    }
  };

  const loadAvailableItems = async () => {
    if (!selectedNetworkId) return;

    try {
      const allGroups = await api.getGroups(selectedNetworkId);
      setAvailableGroups(allGroups);
    } catch (error) {
      console.error('Failed to load available items:', error);
    }
  };

  const loadAttachments = async () => {
    if (!route || !selectedNetworkId) return;

    try {
      // Load all groups in the network
      const allGroups = await api.getGroups(selectedNetworkId);
      
      // Filter groups that have this route attached
      const groupsWithRoute = allGroups.filter(g => g.route_ids?.includes(route.id));
      setAttachedGroups(groupsWithRoute);
      
      // Available groups are those without this route
      const groupsWithoutRoute = allGroups.filter(g => !g.route_ids?.includes(route.id));
      setAvailableGroups(groupsWithoutRoute);
    } catch (error) {
      console.error('Failed to load groups:', error);
    }
  };

  const handleCreateRoute = async () => {
    if (!selectedNetworkId) {
      alert('Please select a network');
      return;
    }
    
    setLoading(true);
    try {
      const newRoute = await api.createRoute(selectedNetworkId, {
        name,
        description,
        destination_cidr: destinationCidr,
        jump_peer_id: jumpPeerId,
        domain_suffix: domainSuffix,
      });
      
      // Add staged DNS mappings and attach to groups
      const attachPromises = [];
      
      for (const dns of stagedDnsMappings) {
        attachPromises.push(api.createDNSMapping(selectedNetworkId, newRoute.id, dns));
      }
      
      for (const groupId of stagedGroupIds) {
        attachPromises.push(api.attachRouteToGroup(selectedNetworkId, groupId, newRoute.id));
      }
      
      if (attachPromises.length > 0) {
        await Promise.all(attachPromises);
      }
      
      onSuccess();
      onClose();
    } catch (error) {
      console.error('Failed to create route:', error);
      alert('Failed to create route');
    } finally {
      setLoading(false);
    }
  };

  const handleUpdateName = async () => {
    if (!route || !selectedNetworkId) return;
    
    try {
      await api.updateRoute(selectedNetworkId, route.id, { name });
      const updatedRoute = await api.getRoute(selectedNetworkId, route.id);
      Object.assign(route, updatedRoute);
      setIsEditingName(false);
      onSuccess();
    } catch (error) {
      console.error('Failed to update name:', error);
      alert('Failed to update route name');
    }
  };

  const handleUpdateDescription = async () => {
    if (!route || !selectedNetworkId) return;
    
    try {
      await api.updateRoute(selectedNetworkId, route.id, { description });
      const updatedRoute = await api.getRoute(selectedNetworkId, route.id);
      Object.assign(route, updatedRoute);
      setIsEditingDescription(false);
      onSuccess();
    } catch (error) {
      console.error('Failed to update description:', error);
      alert('Failed to update route description');
    }
  };

  const handleUpdateDestinationCidr = async () => {
    if (!route || !selectedNetworkId) return;
    
    try {
      await api.updateRoute(selectedNetworkId, route.id, { destination_cidr: destinationCidr });
      const updatedRoute = await api.getRoute(selectedNetworkId, route.id);
      Object.assign(route, updatedRoute);
      setIsEditingDestinationCidr(false);
      onSuccess();
    } catch (error) {
      console.error('Failed to update destination CIDR:', error);
      alert('Failed to update route destination CIDR');
    }
  };

  const handleUpdateJumpPeer = async () => {
    if (!route || !selectedNetworkId) return;
    
    try {
      await api.updateRoute(selectedNetworkId, route.id, { jump_peer_id: jumpPeerId });
      const updatedRoute = await api.getRoute(selectedNetworkId, route.id);
      Object.assign(route, updatedRoute);
      setIsEditingJumpPeer(false);
      onSuccess();
    } catch (error) {
      console.error('Failed to update jump peer:', error);
      alert('Failed to update route jump peer');
    }
  };

  const handleUpdateDomainSuffix = async () => {
    if (!route || !selectedNetworkId) return;
    
    try {
      await api.updateRoute(selectedNetworkId, route.id, { domain_suffix: domainSuffix });
      const updatedRoute = await api.getRoute(selectedNetworkId, route.id);
      Object.assign(route, updatedRoute);
      setIsEditingDomainSuffix(false);
      onSuccess();
    } catch (error) {
      console.error('Failed to update domain suffix:', error);
      alert('Failed to update route domain suffix');
    }
  };

  const handleCancelEdit = (field: 'name' | 'description' | 'destinationCidr' | 'jumpPeer' | 'domainSuffix') => {
    switch (field) {
      case 'name':
        setName(route?.name || '');
        setIsEditingName(false);
        break;
      case 'description':
        setDescription(route?.description || '');
        setIsEditingDescription(false);
        break;
      case 'destinationCidr':
        setDestinationCidr(route?.destination_cidr || '');
        setIsEditingDestinationCidr(false);
        break;
      case 'jumpPeer':
        setJumpPeerId(route?.jump_peer_id || '');
        setIsEditingJumpPeer(false);
        break;
      case 'domainSuffix':
        setDomainSuffix(route?.domain_suffix || 'internal');
        setIsEditingDomainSuffix(false);
        break;
    }
  };

  const handleAddDNS = async (dnsName: string, ipAddress: string) => {
    if (route && selectedNetworkId) {
      // Edit mode: add immediately
      try {
        await api.createDNSMapping(selectedNetworkId, route.id, { name: dnsName, ip_address: ipAddress });
        await loadDNSMappings();
        onSuccess();
        setIsAddDNSModalOpen(false);
      } catch (error) {
        console.error('Failed to add DNS mapping:', error);
        alert('Failed to add DNS mapping');
      }
    } else {
      // Creation mode: stage for later
      setStagedDnsMappings([...stagedDnsMappings, { name: dnsName, ip_address: ipAddress }]);
      setIsAddDNSModalOpen(false);
    }
  };

  const handleDeleteDNS = async (dnsId: string) => {
    if (route && selectedNetworkId) {
      // Edit mode: delete immediately
      try {
        await api.deleteDNSMapping(selectedNetworkId, route.id, dnsId);
        await loadDNSMappings();
        onSuccess();
      } catch (error) {
        console.error('Failed to delete DNS mapping:', error);
        alert('Failed to delete DNS mapping');
      }
    }
  };

  const handleRemoveStagedDNS = (index: number) => {
    // Creation mode: remove from staged list
    setStagedDnsMappings(stagedDnsMappings.filter((_, i) => i !== index));
  };

  const handleAttachToGroup = async (groupId: string) => {
    const group = availableGroups.find(g => g.id === groupId);
    if (!group) return;
    
    if (route && selectedNetworkId) {
      // Edit mode: attach immediately
      try {
        await api.attachRouteToGroup(selectedNetworkId, groupId, route.id);
        // Refetch the updated route data and reload attachments
        const updatedRoute = await api.getRoute(selectedNetworkId, route.id);
        Object.assign(route, updatedRoute); // Update the route object with fresh data
        await loadAttachments();
        onSuccess();
      } catch (error) {
        console.error('Failed to attach route to group:', error);
        alert('Failed to attach route to group');
      }
    } else {
      // Creation mode: stage for later
      setStagedGroupIds([...stagedGroupIds, groupId]);
      setAttachedGroups([...attachedGroups, group]);
      setAvailableGroups(availableGroups.filter(g => g.id !== groupId));
    }
  };

  const handleDetachFromGroup = async (groupId: string) => {
    if (route && selectedNetworkId) {
      // Edit mode: detach immediately
      try {
        await api.detachRouteFromGroup(selectedNetworkId, groupId, route.id);
        // Refetch the updated route data and reload attachments
        const updatedRoute = await api.getRoute(selectedNetworkId, route.id);
        Object.assign(route, updatedRoute); // Update the route object with fresh data
        await loadAttachments();
        onSuccess();
      } catch (error) {
        console.error('Failed to detach route from group:', error);
        alert('Failed to detach route from group');
      }
    } else {
      // Creation mode: unstage
      const group = attachedGroups.find(g => g.id === groupId);
      if (group) {
        setStagedGroupIds(stagedGroupIds.filter(id => id !== groupId));
        setAttachedGroups(attachedGroups.filter(g => g.id !== groupId));
        setAvailableGroups([...availableGroups, group]);
      }
    }
  };

  if (!isOpen) return null;

  const allDnsMappings = route ? dnsMappings : stagedDnsMappings.map((dns, idx) => ({ 
    id: `temp-${idx}`, 
    ...dns,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString()
  }));

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
          <div className="p-6">
        {/* Header Info */}
        {route && (
          <div className="flex items-start justify-between mb-6">
            <div className="flex items-start gap-4">
              <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue">
                <FontAwesomeIcon icon={faRoute} className="text-2xl text-white" />
              </div>
              <div>
                <h3 className="text-2xl font-bold bg-gradient-to-r from-gray-900 to-gray-600 dark:from-gray-100 dark:to-gray-300 bg-clip-text text-transparent">{route.name}</h3>
                <p className="text-sm text-gray-600 dark:text-gray-300 mt-1">ID: {route.id}</p>
              </div>
            </div>
          </div>
        )}
        
        {!route && (
          <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-4">
            Create Route
          </h2>
        )}

        {/* Tabs */}
        <div className="flex border-b border-gray-200 dark:border-gray-700 mb-4">
          <button
            type="button"
            onClick={() => setActiveTab('details')}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              activeTab === 'details'
                ? 'border-primary-600 text-primary-600 dark:text-primary-400'
                : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
            }`}
          >
            Details
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('dns')}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              activeTab === 'dns'
                ? 'border-primary-600 text-primary-600 dark:text-primary-400'
                : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
            }`}
          >
            <FontAwesomeIcon icon={faGlobe} className="mr-1" />
            DNS Mappings ({allDnsMappings.length})
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('groups')}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              activeTab === 'groups'
                ? 'border-primary-600 text-primary-600 dark:text-primary-400'
                : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
            }`}
          >
            <FontAwesomeIcon icon={faUsers} className="mr-1" />
            Groups ({attachedGroups.length})
          </button>
        </div>

        {/* Details Tab */}
        {activeTab === 'details' && (
          <div className="p-6">
            <div className="space-y-6">
              {!route && (
                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Network *
                  </label>
                  <SearchableSelect
                    options={networks.map(network => ({
                      value: network.id,
                      label: `${network.name} (${network.cidr})`
                    }))}
                    value={selectedNetworkId}
                    onChange={setSelectedNetworkId}
                    placeholder="Select a network..."
                  />
                </div>
              )}

              {/* Name Field */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  Name *
                </label>
                {route && !isEditingName ? (
                  <div className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                    <span className="text-gray-900 dark:text-white font-medium">{name}</span>
                    <button
                      onClick={() => setIsEditingName(true)}
                      className="text-primary-600 hover:text-primary-800 dark:text-primary-400 dark:hover:text-primary-300 text-sm"
                    >
                      <FontAwesomeIcon icon={faPencil} className="mr-1" />
                      Edit
                    </button>
                  </div>
                ) : (
                  <div className="flex items-center gap-2">
                    <input
                      type="text"
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      required
                      className="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                      placeholder="Enter route name..."
                    />
                    {route && (
                      <>
                        <button
                          onClick={handleUpdateName}
                          className="px-3 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 text-sm"
                        >
                          Save
                        </button>
                        <button
                          onClick={() => handleCancelEdit('name')}
                          className="px-3 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 text-sm"
                        >
                          Cancel
                        </button>
                      </>
                    )}
                  </div>
                )}
              </div>

              {/* Description Field */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  Description
                </label>
                {route && !isEditingDescription ? (
                  <div className="flex items-start justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                    <span className="text-gray-900 dark:text-white flex-1">
                      {description || <em className="text-gray-500">No description</em>}
                    </span>
                    <button
                      onClick={() => setIsEditingDescription(true)}
                      className="text-primary-600 hover:text-primary-800 dark:text-primary-400 dark:hover:text-primary-300 text-sm ml-3"
                    >
                      <FontAwesomeIcon icon={faPencil} className="mr-1" />
                      Edit
                    </button>
                  </div>
                ) : (
                  <div className="space-y-2">
                    <textarea
                      value={description}
                      onChange={(e) => setDescription(e.target.value)}
                      rows={2}
                      className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                      placeholder="Enter route description..."
                    />
                    {route && (
                      <div className="flex items-center gap-2">
                        <button
                          onClick={handleUpdateDescription}
                          className="px-3 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 text-sm"
                        >
                          Save
                        </button>
                        <button
                          onClick={() => handleCancelEdit('description')}
                          className="px-3 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 text-sm"
                        >
                          Cancel
                        </button>
                      </div>
                    )}
                  </div>
                )}
              </div>

              {/* Destination CIDR Field */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  Destination CIDR *
                </label>
                {route && !isEditingDestinationCidr ? (
                  <div className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                    <span className="text-gray-900 dark:text-white font-mono">{destinationCidr}</span>
                    <button
                      onClick={() => setIsEditingDestinationCidr(true)}
                      className="text-primary-600 hover:text-primary-800 dark:text-primary-400 dark:hover:text-primary-300 text-sm"
                    >
                      <FontAwesomeIcon icon={faPencil} className="mr-1" />
                      Edit
                    </button>
                  </div>
                ) : (
                  <div className="flex items-center gap-2">
                    <input
                      type="text"
                      value={destinationCidr}
                      onChange={(e) => setDestinationCidr(e.target.value)}
                      placeholder="10.0.0.0/24"
                      required
                      className="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white font-mono"
                    />
                    {route && (
                      <>
                        <button
                          onClick={handleUpdateDestinationCidr}
                          className="px-3 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 text-sm"
                        >
                          Save
                        </button>
                        <button
                          onClick={() => handleCancelEdit('destinationCidr')}
                          className="px-3 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 text-sm"
                        >
                          Cancel
                        </button>
                      </>
                    )}
                  </div>
                )}
              </div>

              {/* Jump Peer Field */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  Jump Peer *
                </label>
                {route && !isEditingJumpPeer ? (
                  <div className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                    <span className="text-gray-900 dark:text-white">
                      {jumpPeers.find(p => p.id === jumpPeerId)?.name || jumpPeerId}
                    </span>
                    <button
                      onClick={() => setIsEditingJumpPeer(true)}
                      className="text-primary-600 hover:text-primary-800 dark:text-primary-400 dark:hover:text-primary-300 text-sm"
                    >
                      <FontAwesomeIcon icon={faPencil} className="mr-1" />
                      Edit
                    </button>
                  </div>
                ) : (
                  <div className="space-y-2">
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
                    {route && (
                      <div className="flex items-center gap-2">
                        <button
                          onClick={handleUpdateJumpPeer}
                          className="px-3 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 text-sm"
                        >
                          Save
                        </button>
                        <button
                          onClick={() => handleCancelEdit('jumpPeer')}
                          className="px-3 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 text-sm"
                        >
                          Cancel
                        </button>
                      </div>
                    )}
                  </div>
                )}
              </div>

              {/* Domain Suffix Field */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  Domain Suffix
                </label>
                {route && !isEditingDomainSuffix ? (
                  <div className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                    <span className="text-gray-900 dark:text-white">{domainSuffix || 'internal'}</span>
                    <button
                      onClick={() => setIsEditingDomainSuffix(true)}
                      className="text-primary-600 hover:text-primary-800 dark:text-primary-400 dark:hover:text-primary-300 text-sm"
                    >
                      <FontAwesomeIcon icon={faPencil} className="mr-1" />
                      Edit
                    </button>
                  </div>
                ) : (
                  <div className="space-y-2">
                    <input
                      type="text"
                      value={domainSuffix}
                      onChange={(e) => setDomainSuffix(e.target.value)}
                      placeholder="internal"
                      className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                    />
                    {route && (
                      <div className="flex items-center gap-2">
                        <button
                          onClick={handleUpdateDomainSuffix}
                          className="px-3 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 text-sm"
                        >
                          Save
                        </button>
                        <button
                          onClick={() => handleCancelEdit('domainSuffix')}
                          className="px-3 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 text-sm"
                        >
                          Cancel
                        </button>
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>

            {/* Create Button for New Routes */}
            {!route && (
              <div className="mt-6 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={onClose}
                  className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600"
                >
                  Cancel
                </button>
                <button
                  onClick={handleCreateRoute}
                  disabled={loading || !name.trim() || !destinationCidr.trim() || !jumpPeerId || !selectedNetworkId}
                  className="px-4 py-2 text-sm font-medium text-white bg-gradient-to-r from-primary-600 to-accent-blue rounded-lg hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {loading ? 'Creating...' : 'Create Route'}
                </button>
              </div>
            )}

            {/* Close Button for Existing Routes */}
            {route && (
              <div className="mt-6 flex justify-end">
                <button
                  onClick={onClose}
                  className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600"
                >
                  Close
                </button>
              </div>
            )}
          </div>
        )}

        {/* DNS Tab */}
        {activeTab === 'dns' && (
          <div className="p-6">
            <div className="space-y-3">
              <div className="flex justify-between items-center mb-3">
                <p className="text-sm text-gray-600 dark:text-gray-400">
                  Manage DNS mappings for this route
                </p>
                <button
                  onClick={() => setIsAddDNSModalOpen(true)}
                  className="px-3 py-1.5 text-sm bg-gradient-to-r from-primary-600 to-accent-blue text-white rounded-lg hover:scale-105"
                >
                  <FontAwesomeIcon icon={faGlobe} className="mr-2" />
                  Add DNS Mapping
                </button>
              </div>
              {allDnsMappings.length === 0 ? (
                <div className="text-center py-8 text-gray-500">
                  No DNS mappings defined. Add mappings to resolve custom domains.
                </div>
              ) : (
                <div className="space-y-2">
                  {allDnsMappings.map((dns, index) => (
                    <div key={dns.id} className="flex items-center justify-between p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
                      <div className="flex-1">
                        <div className="text-sm font-medium text-gray-900 dark:text-white">
                          {dns.name}.{name || route?.name || 'route'}.{domainSuffix || route?.domain_suffix || 'internal'}
                        </div>
                        <div className="text-xs text-gray-500 dark:text-gray-400 font-mono mt-1">
                          → {dns.ip_address}
                        </div>
                      </div>
                      <button
                        onClick={() => {
                          if (confirm('Remove this DNS mapping?')) {
                            if (route) {
                              handleDeleteDNS(dns.id);
                            } else {
                              handleRemoveStagedDNS(index);
                            }
                          }
                        }}
                        className="p-2 text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded ml-4"
                      >
                        <FontAwesomeIcon icon={faTrash} />
                      </button>
                    </div>
                  ))}
                </div>
              )}
              <div className="mt-4 flex justify-end">
                <button
                  onClick={onClose}
                  className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600"
                >
                  Close
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Groups Tab */}
        {activeTab === 'groups' && (
          <div className="p-6">
            <div className="space-y-3">
              <p className="text-sm text-gray-600 dark:text-gray-400 mb-3">
                Groups that have this route attached
              </p>
              {attachedGroups.map((group) => (
                <div key={group.id} className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                  <div>
                    <span className="text-sm font-medium text-gray-900 dark:text-white">{group.name}</span>
                    {group.description && (
                      <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">{group.description}</p>
                    )}
                  </div>
                  <button
                    onClick={() => {
                      if (confirm(`Detach this route from group "${group.name}"?`)) {
                        handleDetachFromGroup(group.id);
                      }
                    }}
                    className="p-2 text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded"
                  >
                    <FontAwesomeIcon icon={faTrash} />
                  </button>
                </div>
              ))}
              {availableGroups.length > 0 ? (
                <div className="border-t border-gray-200 dark:border-gray-600 pt-3">
                  <select
                    onChange={(e) => {
                      if (e.target.value) {
                        handleAttachToGroup(e.target.value);
                        e.target.value = '';
                      }
                    }}
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                  >
                    <option value="">Attach to group...</option>
                    {availableGroups.map((group) => (
                      <option key={group.id} value={group.id}>
                        {group.name}
                      </option>
                    ))}
                  </select>
                </div>
              ) : (
                <div className="text-sm text-gray-500 dark:text-gray-400 text-center py-4">
                  This route is attached to all groups in the network
                </div>
              )}
              <div className="mt-4 flex justify-end">
                <button
                  onClick={onClose}
                  className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600"
                >
                  Close
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Add DNS Modal */}
        <AddDNSModal
          isOpen={isAddDNSModalOpen}
          onClose={() => setIsAddDNSModalOpen(false)}
          onAdd={handleAddDNS}
          routeCidr={destinationCidr || route?.destination_cidr || ''}
        />
      </div>
      </div>
      </div>
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
