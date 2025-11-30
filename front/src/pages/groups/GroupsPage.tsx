import { useState, useEffect, useCallback, useMemo } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faUsers, faPencil, faTrash, faUserPlus, faUserMinus, faShieldAlt, faRoute, faGripVertical, faArrowUp, faArrowDown } from '@fortawesome/free-solid-svg-icons';
import PageHeader from '../../components/PageHeader';
import SearchableSelect from '../../components/SearchableSelect';
import api from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';
import type { Group, Network, Peer, Policy, Route } from '../../types';

export default function GroupsPage() {
  const { user } = useAuth();
  const [networks, setNetworks] = useState<Network[]>([]);
  const [selectedNetworkId, setSelectedNetworkId] = useState<string>('');
  const [groups, setGroups] = useState<Group[]>([]);
  const [loading, setLoading] = useState(true);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingGroup, setEditingGroup] = useState<Group | null>(null);
  const [selectedGroup, setSelectedGroup] = useState<Group | null>(null);
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false);

  const isAdmin = user?.role === 'administrator';

  // Load networks on mount
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

  const loadGroups = useCallback(async () => {
    if (!selectedNetworkId) {
      setGroups([]);
      setLoading(false);
      return;
    }

    setLoading(true);
    try {
      const data = await api.getGroups(selectedNetworkId);
      setGroups(data || []);
    } catch (error) {
      console.error('Failed to load groups:', error);
      setGroups([]);
    } finally {
      setLoading(false);
    }
  }, [selectedNetworkId]);

  useEffect(() => {
    void loadGroups();
  }, [loadGroups]);

  const handleGroupClick = (group: Group) => {
    setSelectedGroup(group);
    setIsDetailModalOpen(true);
  };

  const handleCreate = () => {
    setEditingGroup(null);
    setIsModalOpen(true);
  };

  const handleEdit = (group: Group, e: React.MouseEvent) => {
    e.stopPropagation();
    setEditingGroup(group);
    setIsModalOpen(true);
  };

  const handleDelete = async (group: Group, e: React.MouseEvent) => {
    e.stopPropagation();
    if (confirm(`Are you sure you want to delete group "${group.name}"? This will remove all peer associations.`)) {
      try {
        await api.deleteGroup(selectedNetworkId, group.id);
        loadGroups();
      } catch (error) {
        console.error('Failed to delete group:', error);
        alert('Failed to delete group');
      }
    }
  };

  if (!isAdmin) {
    return (
      <div className="p-8">
        <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4">
          <p className="text-yellow-800 dark:text-yellow-200">
            You need administrator privileges to manage groups.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div>
      <PageHeader 
        title="Groups" 
        subtitle={`${groups.length} group${groups.length !== 1 ? 's' : ''} in selected network`}
        action={
          selectedNetworkId ? (
            <button
              onClick={handleCreate}
              className="px-4 py-2.5 bg-gradient-to-r from-primary-600 to-accent-blue text-white rounded-xl hover:scale-105 active:scale-95 shadow-lg hover:shadow-xl flex items-center gap-2 cursor-pointer transition-all font-semibold"
            >
              <svg className="w-5 h-5 group-hover:rotate-90 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              Group
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

        {/* Groups List */}
        {!selectedNetworkId ? (
          <div className="bg-gradient-to-br from-white via-gray-50 to-white dark:from-gray-800 dark:via-gray-800/50 dark:to-gray-800 rounded-2xl border border-gray-200 dark:border-gray-700 p-16 text-center shadow-sm">
            <div className="inline-flex items-center justify-center w-20 h-20 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue mb-6">
              <FontAwesomeIcon icon={faUsers} className="text-3xl text-white" />
            </div>
            <h3 className="text-xl font-bold text-gray-900 dark:text-white mb-2">Select a network</h3>
            <p className="text-gray-600 dark:text-gray-300">
              Choose a network from the dropdown above to view and manage groups
            </p>
          </div>
        ) : loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-gray-500">Loading groups...</div>
          </div>
        ) : groups.length === 0 ? (
          <div className="bg-gradient-to-br from-white via-gray-50 to-white dark:from-gray-800 dark:via-gray-800/50 dark:to-gray-800 rounded-2xl border border-gray-200 dark:border-gray-700 p-16 text-center shadow-sm">
            <div className="inline-flex items-center justify-center w-20 h-20 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue mb-6">
              <FontAwesomeIcon icon={faUsers} className="text-3xl text-white" />
            </div>
            <h3 className="text-xl font-bold text-gray-900 dark:text-white mb-2">No groups found</h3>
            <p className="text-gray-600 dark:text-gray-300">
              Get started by creating your first group
            </p>
          </div>
        ) : (
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead className="bg-gray-50 dark:bg-gray-700">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Name</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Description</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Priority</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Peers</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Policies</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Routes</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Created</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Actions</th>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                {groups.map((group) => (
                  <tr
                    key={group.id}
                    onClick={() => handleGroupClick(group)}
                    className="hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer"
                  >
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex items-center">
                        <div className="inline-flex items-center justify-center w-10 h-10 rounded-xl bg-gradient-to-br from-primary-500 to-accent-blue mr-3">
                          <FontAwesomeIcon icon={faUsers} className="text-lg text-white" />
                        </div>
                        <div className="text-sm font-medium text-gray-900 dark:text-white">{group.name}</div>
                      </div>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-900 dark:text-white">
                      {group.description || '-'}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm">
                      <span className={`px-2 py-1 rounded-full text-xs font-semibold ${
                        group.priority === 0 
                          ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                          : group.priority < 100
                          ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
                          : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'
                      }`}>
                        {group.priority}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                      {group.peer_ids?.length || 0}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                      {group.policy_ids?.length || 0}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                      {group.route_ids?.length || 0}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                      {new Date(group.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm">
                      <div className="flex items-center gap-2">
                        <button
                          onClick={(e) => handleEdit(group, e)}
                          className="text-primary-600 hover:text-primary-800 dark:text-primary-400 dark:hover:text-primary-300 transition-colors"
                          title="Edit group"
                        >
                          <FontAwesomeIcon icon={faPencil} />
                        </button>
                        <button
                          onClick={(e) => handleDelete(group, e)}
                          className="text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300 transition-colors"
                          title="Delete group"
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

      {/* Create/Edit Modal */}
      <GroupModal
        isOpen={isModalOpen}
        onClose={() => {
          setIsModalOpen(false);
          setEditingGroup(null);
        }}
        onSuccess={loadGroups}
        networkId={selectedNetworkId}
        group={editingGroup}
      />

      {/* Detail Modal */}
      <GroupDetailModal
        isOpen={isDetailModalOpen}
        onClose={() => {
          setIsDetailModalOpen(false);
          setSelectedGroup(null);
        }}
        group={selectedGroup}
        networkId={selectedNetworkId}
        onUpdate={loadGroups}
      />
    </div>
  );
}

// Group Modal Component
function GroupModal({
  isOpen,
  onClose,
  onSuccess,
  networkId,
  group,
}: {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  networkId: string;
  group: Group | null;
}) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [priority, setPriority] = useState<number>(100);
  const [loading, setLoading] = useState(false);

  const isQuarantineGroup = group?.name.toLowerCase() === 'quarantine';

  useEffect(() => {
    if (group) {
      setName(group.name);
      setDescription(group.description || '');
      setPriority(group.priority);
    } else {
      setName('');
      setDescription('');
      setPriority(100);
    }
  }, [group]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);

    try {
      if (group) {
        await api.updateGroup(networkId, group.id, { name, description, priority: isQuarantineGroup ? undefined : priority });
      } else {
        await api.createGroup(networkId, { name, description, priority });
      }
      onSuccess();
      onClose();
    } catch (error) {
      console.error('Failed to save group:', error);
      alert('Failed to save group');
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
          {group ? 'Edit Group' : 'Create Group'}
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
                rows={3}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              />
            </div>
            {!isQuarantineGroup && (
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Priority (1-999)
                </label>
                <input
                  type="number"
                  min="1"
                  max="999"
                  value={priority}
                  onChange={(e) => setPriority(parseInt(e.target.value) || 100)}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                />
                <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  Lower number = higher priority. Default is 100. Quarantine groups use 0.
                </p>
              </div>
            )}
            {isQuarantineGroup && (
              <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-3">
                <p className="text-sm text-yellow-800 dark:text-yellow-200">
                  Quarantine groups have priority 0 (highest) and cannot be changed.
                </p>
              </div>
            )}
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
              {loading ? 'Saving...' : group ? 'Update' : 'Create'}
            </button>
          </div>
        </form>
        </div>
      </div>
      </div>
    </div>
  );
}

// Group Detail Modal Component
function GroupDetailModal({
  isOpen,
  onClose,
  group,
  networkId,
  onUpdate,
}: {
  isOpen: boolean;
  onClose: () => void;
  group: Group | null;
  networkId: string;
  onUpdate: () => void;
}) {
  const [peers, setPeers] = useState<Peer[]>([]);
  const [availablePeers, setAvailablePeers] = useState<Peer[]>([]);
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [availablePolicies, setAvailablePolicies] = useState<Policy[]>([]);
  const [routes, setRoutes] = useState<Route[]>([]);
  const [availableRoutes, setAvailableRoutes] = useState<Route[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (isOpen && group && networkId) {
      loadGroupDetails();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, group, networkId]);

  const loadGroupDetails = async () => {
    if (!group || !networkId) return;

    setLoading(true);
    try {
      // Refetch the group to get updated peer_ids, policy_ids, route_ids
      const updatedGroup = await api.getGroup(networkId, group.id);
      
      // Load all network peers
      const allPeers = await api.getAllNetworkPeers(networkId);
      const groupPeerIds = new Set(updatedGroup.peer_ids || []);
      setPeers(allPeers.filter(p => groupPeerIds.has(p.id)));
      
      // Load group routes first to filter out jump peers used by routes
      const [groupRoutes, allRoutes] = await Promise.all([
        api.getGroupRoutes(networkId, updatedGroup.id),
        api.getRoutes(networkId),
      ]);
      setRoutes(groupRoutes);
      
      // Get jump peer IDs used by routes in this group
      const jumpPeerIdsInRoutes = new Set(groupRoutes.map(r => r.jump_peer_id));
      
      // Filter available peers: exclude group members and jump peers used by group routes
      setAvailablePeers(allPeers.filter(p => 
        !groupPeerIds.has(p.id) && 
        !(p.is_jump && jumpPeerIdsInRoutes.has(p.id))
      ));

      // Load group policies and all network policies
      const [groupPolicies, allPolicies] = await Promise.all([
        api.getGroupPolicies(networkId, updatedGroup.id),
        api.getPolicies(networkId),
      ]);
      setPolicies(groupPolicies);
      const groupPolicyIds = new Set(groupPolicies.map(p => p.id));
      setAvailablePolicies(allPolicies.filter(p => !groupPolicyIds.has(p.id)));

      // Available routes (already loaded above)
      // Filter out routes that are already attached or whose jump peer is in the group
      const groupRouteIds = new Set(groupRoutes.map(r => r.id));
      setAvailableRoutes(allRoutes.filter(r => 
        !groupRouteIds.has(r.id) && 
        !groupPeerIds.has(r.jump_peer_id)
      ));
    } catch (error) {
      console.error('Failed to load group details:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleAddPeer = async (peerId: string) => {
    if (!group || !networkId) return;
    
    // Check if the peer is a jump peer
    const peer = availablePeers.find(p => p.id === peerId);
    if (peer && peer.is_jump) {
      // Check if any routes in this group use this jump peer
      const conflictingRoutes = routes.filter(route => route.jump_peer_id === peerId);
      if (conflictingRoutes.length > 0) {
        const routeNames = conflictingRoutes.map(r => r.name).join(', ');
        alert(`Cannot add jump peer "${peer.name}" to this group because it is the gateway for the following routes attached to this group: ${routeNames}`);
        return;
      }
    }
    
    try {
      await api.addPeerToGroup(networkId, group.id, peerId);
      await loadGroupDetails();
      onUpdate();
    } catch (error: any) {
      console.error('Failed to add peer:', error);
      // Check if it's a circular routing error from the backend
      if (error.response?.data?.error) {
        alert(error.response.data.error);
      } else {
        alert('Failed to add peer to group');
      }
    }
  };

  const handleRemovePeer = async (peerId: string) => {
    if (!group || !networkId) return;
    try {
      await api.removePeerFromGroup(networkId, group.id, peerId);
      await loadGroupDetails();
      onUpdate();
    } catch (error) {
      console.error('Failed to remove peer:', error);
      alert('Failed to remove peer from group');
    }
  };

  const handleAttachPolicy = async (policyId: string) => {
    if (!group || !networkId) return;
    try {
      await api.attachPolicyToGroup(networkId, group.id, policyId);
      await loadGroupDetails();
      onUpdate();
    } catch (error) {
      console.error('Failed to attach policy:', error);
      alert('Failed to attach policy to group');
    }
  };

  const handleDetachPolicy = async (policyId: string) => {
    if (!group || !networkId) return;
    try {
      await api.detachPolicyFromGroup(networkId, group.id, policyId);
      await loadGroupDetails();
      onUpdate();
    } catch (error) {
      console.error('Failed to detach policy:', error);
      alert('Failed to detach policy from group');
    }
  };

  const handleReorderPolicies = async (fromIndex: number, toIndex: number) => {
    if (!group || !networkId) return;
    
    // Create a new array with the reordered policies
    const reorderedPolicies = [...policies];
    const [movedPolicy] = reorderedPolicies.splice(fromIndex, 1);
    reorderedPolicies.splice(toIndex, 0, movedPolicy);
    
    // Optimistically update the UI
    setPolicies(reorderedPolicies);
    
    try {
      // Send the new order to the backend
      const policyIds = reorderedPolicies.map(p => p.id);
      await api.reorderGroupPolicies(networkId, group.id, policyIds);
      onUpdate();
    } catch (error) {
      console.error('Failed to reorder policies:', error);
      alert('Failed to reorder policies');
      // Reload to get the correct order from the server
      await loadGroupDetails();
    }
  };

  const handleAttachRoute = async (routeId: string) => {
    if (!group || !networkId) return;
    
    // Check if the route's jump peer is a member of this group
    const route = availableRoutes.find(r => r.id === routeId);
    if (route) {
      const jumpPeerInGroup = peers.find(p => p.id === route.jump_peer_id);
      if (jumpPeerInGroup) {
        alert(`Cannot attach route "${route.name}" to this group because its jump peer "${jumpPeerInGroup.name}" is a member of this group. This would create a circular routing configuration.`);
        return;
      }
    }
    
    try {
      await api.attachRouteToGroup(networkId, group.id, routeId);
      await loadGroupDetails();
      onUpdate();
    } catch (error: any) {
      console.error('Failed to attach route:', error);
      // Check if it's a circular routing error from the backend
      if (error.response?.data?.error) {
        alert(error.response.data.error);
      } else {
        alert('Failed to attach route to group');
      }
    }
  };

  const handleDetachRoute = async (routeId: string) => {
    if (!group || !networkId) return;
    try {
      await api.detachRouteFromGroup(networkId, group.id, routeId);
      await loadGroupDetails();
      onUpdate();
    } catch (error) {
      console.error('Failed to detach route:', error);
      alert('Failed to detach route from group');
    }
  };

  if (!isOpen || !group) return null;

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
              <FontAwesomeIcon icon={faUsers} className="text-2xl text-white" />
            </div>
            <div>
              <h2 className="text-2xl font-bold text-gray-900 dark:text-white">{group.name}</h2>
              <p className="text-sm text-gray-600 dark:text-gray-300 mt-1">ID: {group.id}</p>
              {group.description && (
                <p className="text-gray-600 dark:text-gray-400 mt-1">{group.description}</p>
              )}
            </div>
          </div>
        </div>

        {loading ? (
          <div className="p-6 text-center text-gray-500">Loading...</div>
        ) : (
          <div className="p-6 space-y-6">
            {/* Peers Section */}
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-3 flex items-center gap-2">
                <FontAwesomeIcon icon={faUserPlus} />
                Peers ({peers.length})
              </h3>
              <div className="space-y-2">
                {peers.map((peer) => (
                  <div key={peer.id} className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                    <span className="text-sm text-gray-900 dark:text-white">{peer.name}</span>
                    <button
                      onClick={() => handleRemovePeer(peer.id)}
                      className="text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300"
                      title="Remove peer"
                    >
                      <FontAwesomeIcon icon={faUserMinus} />
                    </button>
                  </div>
                ))}
                {availablePeers.length > 0 && (
                  <select
                    onChange={(e) => {
                      if (e.target.value) {
                        handleAddPeer(e.target.value);
                        e.target.value = '';
                      }
                    }}
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                  >
                    <option value="">Add peer...</option>
                    {availablePeers.map((peer) => (
                      <option key={peer.id} value={peer.id}>{peer.name}</option>
                    ))}
                  </select>
                )}
              </div>
            </div>

            {/* Policies Section */}
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-3 flex items-center gap-2">
                <FontAwesomeIcon icon={faShieldAlt} />
                Policies ({policies.length})
                {policies.length > 1 && (
                  <span className="text-xs text-gray-500 dark:text-gray-400 font-normal ml-2">
                    (Order matters - first to last)
                  </span>
                )}
              </h3>
              <div className="space-y-2">
                {policies.map((policy, index) => (
                  <div
                    key={policy.id}
                    draggable
                    onDragStart={(e) => {
                      e.dataTransfer.effectAllowed = 'move';
                      e.dataTransfer.setData('policyIndex', index.toString());
                    }}
                    onDragOver={(e) => {
                      e.preventDefault();
                      e.dataTransfer.dropEffect = 'move';
                    }}
                    onDrop={(e) => {
                      e.preventDefault();
                      const fromIndex = parseInt(e.dataTransfer.getData('policyIndex'));
                      if (fromIndex !== index) {
                        handleReorderPolicies(fromIndex, index);
                      }
                    }}
                    className="flex items-center gap-2 p-3 bg-gray-50 dark:bg-gray-700 rounded-lg cursor-move hover:bg-gray-100 dark:hover:bg-gray-600 transition-colors"
                  >
                    <FontAwesomeIcon 
                      icon={faGripVertical} 
                      className="text-gray-400 dark:text-gray-500 cursor-grab active:cursor-grabbing"
                      title="Drag to reorder"
                    />
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <span className="text-xs font-mono text-gray-500 dark:text-gray-400">#{index + 1}</span>
                        <span className="text-sm font-medium text-gray-900 dark:text-white">{policy.name}</span>
                        <span className="text-xs text-gray-500 dark:text-gray-400">({policy.rules?.length || 0} rules)</span>
                      </div>
                    </div>
                    <div className="flex items-center gap-1">
                      {index > 0 && (
                        <button
                          onClick={() => handleReorderPolicies(index, index - 1)}
                          className="p-1.5 text-gray-600 hover:text-primary-600 dark:text-gray-400 dark:hover:text-primary-400 transition-colors"
                          title="Move up"
                        >
                          <FontAwesomeIcon icon={faArrowUp} className="text-sm" />
                        </button>
                      )}
                      {index < policies.length - 1 && (
                        <button
                          onClick={() => handleReorderPolicies(index, index + 1)}
                          className="p-1.5 text-gray-600 hover:text-primary-600 dark:text-gray-400 dark:hover:text-primary-400 transition-colors"
                          title="Move down"
                        >
                          <FontAwesomeIcon icon={faArrowDown} className="text-sm" />
                        </button>
                      )}
                      <button
                        onClick={() => handleDetachPolicy(policy.id)}
                        className="p-1.5 text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300 transition-colors ml-1"
                        title="Detach policy"
                      >
                        <FontAwesomeIcon icon={faTrash} className="text-sm" />
                      </button>
                    </div>
                  </div>
                ))}
                {availablePolicies.length > 0 && (
                  <select
                    onChange={(e) => {
                      if (e.target.value) {
                        handleAttachPolicy(e.target.value);
                        e.target.value = '';
                      }
                    }}
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                  >
                    <option value="">Attach policy...</option>
                    {availablePolicies.map((policy) => (
                      <option key={policy.id} value={policy.id}>{policy.name}</option>
                    ))}
                  </select>
                )}
              </div>
            </div>

            {/* Routes Section */}
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-3 flex items-center gap-2">
                <FontAwesomeIcon icon={faRoute} />
                Routes ({routes.length})
              </h3>
              <div className="space-y-2">
                {routes.map((route) => (
                  <div key={route.id} className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                    <div>
                      <span className="text-sm font-medium text-gray-900 dark:text-white">{route.name}</span>
                      <span className="text-xs text-gray-500 dark:text-gray-400 ml-2 font-mono">{route.destination_cidr}</span>
                    </div>
                    <button
                      onClick={() => handleDetachRoute(route.id)}
                      className="text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300"
                      title="Detach route"
                    >
                      <FontAwesomeIcon icon={faTrash} />
                    </button>
                  </div>
                ))}
                {availableRoutes.length > 0 && (
                  <select
                    onChange={(e) => {
                      if (e.target.value) {
                        handleAttachRoute(e.target.value);
                        e.target.value = '';
                      }
                    }}
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                  >
                    <option value="">Attach route...</option>
                    {availableRoutes.map((route) => (
                      <option key={route.id} value={route.id}>{route.name} ({route.destination_cidr})</option>
                    ))}
                  </select>
                )}
              </div>
            </div>
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
    </div>
  );
}

