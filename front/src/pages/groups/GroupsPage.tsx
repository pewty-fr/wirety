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
  const [isGroupModalOpen, setIsGroupModalOpen] = useState(false);
  const [editingGroup, setEditingGroup] = useState<Group | null>(null);

  const isAdmin = user?.role === 'administrator';

  // Load networks on mount
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

  const loadGroups = useCallback(async () => {
    setLoading(true);
    try {
      let data;
      if (selectedNetworkId) {
        // Load groups for specific network
        data = await api.getGroups(selectedNetworkId);
      } else {
        // Load all groups from all networks
        data = await api.getAllGroups();
      }
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
    // Open the edit modal directly (which has all the tabs and functionality)
    setEditingGroup(group);
    setIsGroupModalOpen(true);
  };

  const handleCreate = () => {
    setEditingGroup(null);
    setIsGroupModalOpen(true);
  };



  const handleDeleteGroup = async (groupId: string) => {
    try {
      await api.deleteGroup(selectedNetworkId, groupId);
      loadGroups();
    } catch (error) {
      console.error('Failed to delete group:', error);
      alert('Failed to delete group');
    }
  };

  const handleDelete = async (group: Group, e: React.MouseEvent) => {
    e.stopPropagation();
    if (confirm(`Are you sure you want to delete group "${group.name}"? This will remove all peer associations.`)) {
      await handleDeleteGroup(group.id);
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
        subtitle={`${groups.length} group${groups.length !== 1 ? 's' : ''} ${selectedNetworkId ? 'in selected network' : 'across all networks'}`}
        action={
          <button
            onClick={handleCreate}
            className="px-4 py-2.5 bg-gradient-to-r from-primary-600 to-accent-blue text-white rounded-xl hover:scale-105 active:scale-95 shadow-lg hover:shadow-xl flex items-center gap-2 cursor-pointer transition-all font-semibold"
          >
            <svg className="w-5 h-5 group-hover:rotate-90 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
            Group
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

        {/* Groups List */}
        {loading ? (
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
                  {!selectedNetworkId && (
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Network</th>
                  )}
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
                    {!selectedNetworkId && (
                      <td className="px-6 py-4 text-sm text-gray-900 dark:text-white">
                        {group.network_name || '-'}
                      </td>
                    )}
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
        isOpen={isGroupModalOpen}
        onClose={() => {
          setIsGroupModalOpen(false);
          setEditingGroup(null);
        }}
        onSuccess={loadGroups}
        networkId={selectedNetworkId}
        group={editingGroup}
        networks={networks}
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
  networks,
}: {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  networkId: string;
  group: Group | null;
  networks: Network[];
}) {
  const [activeTab, setActiveTab] = useState<'details' | 'peers' | 'policies' | 'routes'>('details');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [priority, setPriority] = useState<number>(100);
  const [selectedNetworkId, setSelectedNetworkId] = useState<string>(networkId);
  const [loading, setLoading] = useState(false);
  
  // Attachment management state
  const [peers, setPeers] = useState<Peer[]>([]);
  const [availablePeers, setAvailablePeers] = useState<Peer[]>([]);
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [availablePolicies, setAvailablePolicies] = useState<Policy[]>([]);
  const [routes, setRoutes] = useState<Route[]>([]);
  const [availableRoutes, setAvailableRoutes] = useState<Route[]>([]);
  
  // Staged attachments for creation mode (IDs to attach after group is created)
  const [stagedPeerIds, setStagedPeerIds] = useState<string[]>([]);
  const [stagedPolicyIds, setStagedPolicyIds] = useState<string[]>([]);
  const [stagedRouteIds, setStagedRouteIds] = useState<string[]>([]);

  // Individual edit modes
  const [isEditingName, setIsEditingName] = useState(false);
  const [isEditingDescription, setIsEditingDescription] = useState(false);
  const [isEditingPriority, setIsEditingPriority] = useState(false);

  const isQuarantineGroup = group?.name.toLowerCase() === 'quarantine';
  const isDefaultGroup = group?.name.toLowerCase() === 'default';
  const isSpecialGroup = isQuarantineGroup || isDefaultGroup;

  useEffect(() => {
    if (group) {
      setName(group.name);
      setDescription(group.description || '');
      setPriority(group.priority);
      setSelectedNetworkId(networkId);
      setStagedPeerIds([]);
      setStagedPolicyIds([]);
      setStagedRouteIds([]);
      // Reset edit modes
      setIsEditingName(false);
      setIsEditingDescription(false);
      setIsEditingPriority(false);
      // Load attachments when editing
      loadAttachments();
    } else {
      setName('');
      setDescription('');
      setPriority(100);
      setSelectedNetworkId(networkId || '');
      setPeers([]);
      setStagedPeerIds([]);
      setStagedPolicyIds([]);
      setStagedRouteIds([]);
      // Reset edit modes
      setIsEditingName(false);
      setIsEditingDescription(false);
      setIsEditingPriority(false);
      // Load available items for creation mode if network is selected
      if (selectedNetworkId) {
        loadAvailableItems();
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [group, networkId, isOpen]);

  // Load available items when network changes in creation mode
  useEffect(() => {
    if (!group && selectedNetworkId && isOpen) {
      loadAvailableItems();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedNetworkId, group, isOpen]);

  const loadAvailableItems = async () => {
    if (!selectedNetworkId) return;

    try {
      const [allPeers, allPolicies, allRoutes] = await Promise.all([
        api.getAllNetworkPeers(selectedNetworkId),
        api.getPolicies(selectedNetworkId),
        api.getRoutes(selectedNetworkId),
      ]);
      
      setAvailablePeers(allPeers);
      setAvailablePolicies(allPolicies);
      setAvailableRoutes(allRoutes);
    } catch (error) {
      console.error('Failed to load available items:', error);
    }
  };

  const loadAttachments = async () => {
    if (!group || !selectedNetworkId) return;

    try {
      // Load all network peers
      const allPeers = await api.getAllNetworkPeers(selectedNetworkId);
      const groupPeerIds = new Set(group.peer_ids || []);
      setPeers(allPeers.filter(p => groupPeerIds.has(p.id)));
      
      // Load group routes first to filter out jump peers used by routes
      const [groupRoutes, allRoutes] = await Promise.all([
        api.getGroupRoutes(selectedNetworkId, group.id),
        api.getRoutes(selectedNetworkId),
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
        api.getGroupPolicies(selectedNetworkId, group.id),
        api.getPolicies(selectedNetworkId),
      ]);
      setPolicies(groupPolicies);
      const groupPolicyIds = new Set(groupPolicies.map(p => p.id));
      setAvailablePolicies(allPolicies.filter(p => !groupPolicyIds.has(p.id)));

      // Available routes
      const groupRouteIds = new Set(groupRoutes.map(r => r.id));
      setAvailableRoutes(allRoutes.filter(r => 
        !groupRouteIds.has(r.id) && 
        !groupPeerIds.has(r.jump_peer_id)
      ));
    } catch (error) {
      console.error('Failed to load attachments:', error);
    }
  };

  const handleCreateGroup = async () => {
    if (!selectedNetworkId) {
      alert('Please select a network');
      return;
    }
    
    setLoading(true);
    try {
      const newGroup = await api.createGroup(selectedNetworkId, { name, description, priority });
      
      // Attach staged items
      const attachPromises = [];
      
      for (const peerId of stagedPeerIds) {
        attachPromises.push(api.addPeerToGroup(selectedNetworkId, newGroup.id, peerId));
      }
      
      for (const policyId of stagedPolicyIds) {
        attachPromises.push(api.attachPolicyToGroup(selectedNetworkId, newGroup.id, policyId));
      }
      
      for (const routeId of stagedRouteIds) {
        attachPromises.push(api.attachRouteToGroup(selectedNetworkId, newGroup.id, routeId));
      }
      
      // Wait for all attachments to complete
      if (attachPromises.length > 0) {
        await Promise.all(attachPromises);
      }
      
      onSuccess();
      onClose();
    } catch (error) {
      console.error('Failed to create group:', error);
      alert('Failed to create group');
    } finally {
      setLoading(false);
    }
  };

  const handleUpdateName = async () => {
    if (!group || !selectedNetworkId) return;
    
    try {
      await api.updateGroup(selectedNetworkId, group.id, { name });
      const updatedGroup = await api.getGroup(selectedNetworkId, group.id);
      Object.assign(group, updatedGroup);
      setIsEditingName(false);
      onSuccess();
    } catch (error) {
      console.error('Failed to update name:', error);
      alert('Failed to update group name');
    }
  };

  const handleUpdateDescription = async () => {
    if (!group || !selectedNetworkId) return;
    
    try {
      await api.updateGroup(selectedNetworkId, group.id, { description });
      const updatedGroup = await api.getGroup(selectedNetworkId, group.id);
      Object.assign(group, updatedGroup);
      setIsEditingDescription(false);
      onSuccess();
    } catch (error) {
      console.error('Failed to update description:', error);
      alert('Failed to update group description');
    }
  };

  const handleUpdatePriority = async () => {
    if (!group || !selectedNetworkId) return;
    
    try {
      await api.updateGroup(selectedNetworkId, group.id, { priority });
      const updatedGroup = await api.getGroup(selectedNetworkId, group.id);
      Object.assign(group, updatedGroup);
      setIsEditingPriority(false);
      onSuccess();
    } catch (error) {
      console.error('Failed to update priority:', error);
      alert('Failed to update group priority');
    }
  };

  const handleCancelEdit = (field: 'name' | 'description' | 'priority') => {
    switch (field) {
      case 'name':
        setName(group?.name || '');
        setIsEditingName(false);
        break;
      case 'description':
        setDescription(group?.description || '');
        setIsEditingDescription(false);
        break;
      case 'priority':
        setPriority(group?.priority || 100);
        setIsEditingPriority(false);
        break;
    }
  };

  const handleAddPeer = async (peerId: string) => {
    const peer = availablePeers.find(p => p.id === peerId);
    if (!peer) return;
    
    if (peer.is_jump) {
      const conflictingRoutes = (group ? routes : availableRoutes.filter(r => stagedRouteIds.includes(r.id))).filter(route => route.jump_peer_id === peerId);
      if (conflictingRoutes.length > 0) {
        const routeNames = conflictingRoutes.map(r => r.name).join(', ');
        alert(`Cannot add jump peer "${peer.name}" to this group because it is the gateway for the following routes attached to this group: ${routeNames}`);
        return;
      }
    }
    
    if (group && selectedNetworkId) {
      // Edit mode: attach immediately
      try {
        await api.addPeerToGroup(selectedNetworkId, group.id, peerId);
        // Refetch the updated group data and reload attachments
        const updatedGroup = await api.getGroup(selectedNetworkId, group.id);
        Object.assign(group, updatedGroup); // Update the group object with fresh data
        await loadAttachments();
        onSuccess();
      } catch (error: any) {
        console.error('Failed to add peer:', error);
        if (error.response?.data?.error) {
          alert(error.response.data.error);
        } else {
          alert('Failed to add peer to group');
        }
      }
    } else {
      // Creation mode: stage for later
      setStagedPeerIds([...stagedPeerIds, peerId]);
      setPeers([...peers, peer]);
      setAvailablePeers(availablePeers.filter(p => p.id !== peerId));
    }
  };

  const handleRemovePeer = async (peerId: string) => {
    if (group && selectedNetworkId) {
      // Edit mode: remove immediately
      try {
        await api.removePeerFromGroup(selectedNetworkId, group.id, peerId);
        // Refetch the updated group data and reload attachments
        const updatedGroup = await api.getGroup(selectedNetworkId, group.id);
        Object.assign(group, updatedGroup); // Update the group object with fresh data
        await loadAttachments();
        onSuccess();
      } catch (error) {
        console.error('Failed to remove peer:', error);
        alert('Failed to remove peer from group');
      }
    } else {
      // Creation mode: unstage
      const peer = peers.find(p => p.id === peerId);
      if (peer) {
        setStagedPeerIds(stagedPeerIds.filter(id => id !== peerId));
        setPeers(peers.filter(p => p.id !== peerId));
        setAvailablePeers([...availablePeers, peer]);
      }
    }
  };

  const handleAttachPolicy = async (policyId: string) => {
    const policy = availablePolicies.find(p => p.id === policyId);
    if (!policy) return;
    
    if (group && selectedNetworkId) {
      // Edit mode: attach immediately
      try {
        await api.attachPolicyToGroup(selectedNetworkId, group.id, policyId);
        // Refetch the updated group data and reload attachments
        const updatedGroup = await api.getGroup(selectedNetworkId, group.id);
        Object.assign(group, updatedGroup); // Update the group object with fresh data
        await loadAttachments();
        onSuccess();
      } catch (error) {
        console.error('Failed to attach policy:', error);
        alert('Failed to attach policy to group');
      }
    } else {
      // Creation mode: stage for later
      setStagedPolicyIds([...stagedPolicyIds, policyId]);
      setPolicies([...policies, policy]);
      setAvailablePolicies(availablePolicies.filter(p => p.id !== policyId));
    }
  };

  const handleDetachPolicy = async (policyId: string) => {
    if (group && selectedNetworkId) {
      // Edit mode: detach immediately
      try {
        await api.detachPolicyFromGroup(selectedNetworkId, group.id, policyId);
        // Refetch the updated group data and reload attachments
        const updatedGroup = await api.getGroup(selectedNetworkId, group.id);
        Object.assign(group, updatedGroup); // Update the group object with fresh data
        await loadAttachments();
        onSuccess();
      } catch (error) {
        console.error('Failed to detach policy:', error);
        alert('Failed to detach policy from group');
      }
    } else {
      // Creation mode: unstage
      const policy = policies.find(p => p.id === policyId);
      if (policy) {
        setStagedPolicyIds(stagedPolicyIds.filter(id => id !== policyId));
        setPolicies(policies.filter(p => p.id !== policyId));
        setAvailablePolicies([...availablePolicies, policy]);
      }
    }
  };

  const handleReorderPolicies = async (fromIndex: number, toIndex: number) => {
    const reorderedPolicies = [...policies];
    const [movedPolicy] = reorderedPolicies.splice(fromIndex, 1);
    reorderedPolicies.splice(toIndex, 0, movedPolicy);
    
    setPolicies(reorderedPolicies);
    
    if (group && selectedNetworkId) {
      // Edit mode: reorder on server
      try {
        const policyIds = reorderedPolicies.map(p => p.id);
        await api.reorderGroupPolicies(selectedNetworkId, group.id, policyIds);
        // Refetch the updated group data
        const updatedGroup = await api.getGroup(selectedNetworkId, group.id);
        Object.assign(group, updatedGroup); // Update the group object with fresh data
        onSuccess();
      } catch (error) {
        console.error('Failed to reorder policies:', error);
        alert('Failed to reorder policies');
        await loadAttachments();
      }
    } else {
      // Creation mode: just update staged order
      setStagedPolicyIds(reorderedPolicies.map(p => p.id));
    }
  };

  const handleAttachRoute = async (routeId: string) => {
    const route = availableRoutes.find(r => r.id === routeId);
    if (!route) return;
    
    const jumpPeerInGroup = peers.find(p => p.id === route.jump_peer_id);
    if (jumpPeerInGroup) {
      alert(`Cannot attach route "${route.name}" to this group because its jump peer "${jumpPeerInGroup.name}" is a member of this group. This would create a circular routing configuration.`);
      return;
    }
    
    if (group && selectedNetworkId) {
      // Edit mode: attach immediately
      try {
        await api.attachRouteToGroup(selectedNetworkId, group.id, routeId);
        // Refetch the updated group data and reload attachments
        const updatedGroup = await api.getGroup(selectedNetworkId, group.id);
        Object.assign(group, updatedGroup); // Update the group object with fresh data
        await loadAttachments();
        onSuccess();
      } catch (error: any) {
        console.error('Failed to attach route:', error);
        if (error.response?.data?.error) {
          alert(error.response.data.error);
        } else {
          alert('Failed to attach route to group');
        }
      }
    } else {
      // Creation mode: stage for later
      setStagedRouteIds([...stagedRouteIds, routeId]);
      setRoutes([...routes, route]);
      setAvailableRoutes(availableRoutes.filter(r => r.id !== routeId));
    }
  };

  const handleDetachRoute = async (routeId: string) => {
    if (group && selectedNetworkId) {
      // Edit mode: detach immediately
      try {
        await api.detachRouteFromGroup(selectedNetworkId, group.id, routeId);
        // Refetch the updated group data and reload attachments
        const updatedGroup = await api.getGroup(selectedNetworkId, group.id);
        Object.assign(group, updatedGroup); // Update the group object with fresh data
        await loadAttachments();
        onSuccess();
      } catch (error) {
        console.error('Failed to detach route:', error);
        alert('Failed to detach route from group');
      }
    } else {
      // Creation mode: unstage
      const route = routes.find(r => r.id === routeId);
      if (route) {
        setStagedRouteIds(stagedRouteIds.filter(id => id !== routeId));
        setRoutes(routes.filter(r => r.id !== routeId));
        setAvailableRoutes([...availableRoutes, route]);
      }
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
          className="relative bg-gradient-to-br from-white to-gray-50 dark:from-dark dark:to-gray-800 rounded-lg shadow-2xl w-full max-w-4xl transform transition-all border-2 border-primary-300 dark:border-primary-700 max-h-[90vh] overflow-y-auto"
          onClick={(e) => e.stopPropagation()}
        >
          <div className="p-6">
        {/* Header Info */}
        {group && (
          <div className="flex items-start justify-between mb-6">
            <div className="flex items-start gap-4">
              <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue">
                <FontAwesomeIcon icon={faUsers} className="text-2xl text-white" />
              </div>
              <div>
                <h3 className="text-2xl font-bold bg-gradient-to-r from-gray-900 to-gray-600 dark:from-gray-100 dark:to-gray-300 bg-clip-text text-transparent">{group.name}</h3>
                <p className="text-sm text-gray-600 dark:text-gray-300 mt-1">ID: {group.id}</p>
              </div>
            </div>
          </div>
        )}
        
        {!group && (
          <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-4">
            Create Group
          </h2>
        )}

        {/* Tabs - available in both create and edit modes */}
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
              onClick={() => setActiveTab('peers')}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                activeTab === 'peers'
                  ? 'border-primary-600 text-primary-600 dark:text-primary-400'
                  : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
              }`}
            >
              <FontAwesomeIcon icon={faUserPlus} className="mr-1" />
              Peers ({peers.length})
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('policies')}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                activeTab === 'policies'
                  ? 'border-primary-600 text-primary-600 dark:text-primary-400'
                  : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
              }`}
            >
              <FontAwesomeIcon icon={faShieldAlt} className="mr-1" />
              Policies ({policies.length})
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('routes')}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                activeTab === 'routes'
                  ? 'border-primary-600 text-primary-600 dark:text-primary-400'
                  : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
              }`}
            >
              <FontAwesomeIcon icon={faRoute} className="mr-1" />
              Routes ({routes.length})
            </button>
          </div>

        {/* Details Tab */}
        {activeTab === 'details' && (
          <div className="p-6">
            <div className="space-y-6">
              {!group && (
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
                {group && !isEditingName ? (
                  <div className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                    <span className="text-gray-900 dark:text-white font-medium">{name}</span>
                    {!isSpecialGroup && (
                      <button
                        onClick={() => setIsEditingName(true)}
                        className="text-primary-600 hover:text-primary-800 dark:text-primary-400 dark:hover:text-primary-300 text-sm"
                      >
                        <FontAwesomeIcon icon={faPencil} className="mr-1" />
                        Edit
                      </button>
                    )}
                    {isSpecialGroup && (
                      <span className="text-xs text-gray-500 dark:text-gray-400">
                        {isQuarantineGroup ? 'Quarantine' : 'Default'} group name cannot be changed
                      </span>
                    )}
                  </div>
                ) : (
                  <div className="flex items-center gap-2">
                    <input
                      type="text"
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      required
                      className="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                      placeholder="Enter group name..."
                    />
                    {group && (
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
                {group && !isEditingDescription ? (
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
                      rows={3}
                      className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                      placeholder="Enter group description..."
                    />
                    {group && (
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

              {/* Priority Field */}
              {!isQuarantineGroup && (
                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                    Priority (1-999)
                  </label>
                  {group && !isEditingPriority ? (
                    <div className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                      <div>
                        <span className="text-gray-900 dark:text-white font-medium">{priority}</span>
                        <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                          Lower number = higher priority
                        </p>
                      </div>
                      <button
                        onClick={() => setIsEditingPriority(true)}
                        className="text-primary-600 hover:text-primary-800 dark:text-primary-400 dark:hover:text-primary-300 text-sm"
                      >
                        <FontAwesomeIcon icon={faPencil} className="mr-1" />
                        Edit
                      </button>
                    </div>
                  ) : (
                    <div className="space-y-2">
                      <input
                        type="number"
                        min="1"
                        max="999"
                        value={priority}
                        onChange={(e) => setPriority(parseInt(e.target.value) || 100)}
                        className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                      />
                      <p className="text-xs text-gray-500 dark:text-gray-400">
                        Lower number = higher priority. Default is 100.
                      </p>
                      {group && (
                        <div className="flex items-center gap-2">
                          <button
                            onClick={handleUpdatePriority}
                            className="px-3 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 text-sm"
                          >
                            Save
                          </button>
                          <button
                            onClick={() => handleCancelEdit('priority')}
                            className="px-3 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 text-sm"
                          >
                            Cancel
                          </button>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )}

              {/* Quarantine Group Notice */}
              {isQuarantineGroup && (
                <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4">
                  <p className="text-sm text-yellow-800 dark:text-yellow-200">
                    <strong>Quarantine Group:</strong> This group has priority 0 (highest) and its name cannot be changed. 
                    Peers in this group are isolated from the network.
                  </p>
                </div>
              )}

              {/* Default Group Notice */}
              {isDefaultGroup && (
                <div className="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
                  <p className="text-sm text-blue-800 dark:text-blue-200">
                    <strong>Default Group:</strong> This is a system group and its name cannot be changed. 
                    New peers are automatically added to this group.
                  </p>
                </div>
              )}
            </div>

            {/* Create Button for New Groups */}
            {!group && (
              <div className="mt-6 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={onClose}
                  className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600"
                >
                  Cancel
                </button>
                <button
                  onClick={handleCreateGroup}
                  disabled={loading || !name.trim() || !selectedNetworkId}
                  className="px-4 py-2 text-sm font-medium text-white bg-gradient-to-r from-primary-600 to-accent-blue rounded-lg hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {loading ? 'Creating...' : 'Create Group'}
                </button>
              </div>
            )}

            {/* Close Button for Existing Groups */}
            {group && (
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

        {/* Peers Tab */}
        {activeTab === 'peers' && (
          <div className="p-6">
          <div className="space-y-3">
            {peers.map((peer) => (
              <div key={peer.id} className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                <div className="flex items-center gap-3">
                  <div className={`w-3 h-3 rounded-full ${peer.is_jump ? 'bg-blue-500' : 'bg-green-500'}`} />
                  <div>
                    <span className="text-sm font-medium text-gray-900 dark:text-white">{peer.name}</span>
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                      {peer.address} • {peer.is_jump ? 'Jump Peer' : 'Regular Peer'}
                    </div>
                  </div>
                </div>
                <button
                  onClick={() => {
                    if (confirm(`Remove "${peer.name}" from this group?`)) {
                      handleRemovePeer(peer.id);
                    }
                  }}
                  className="p-2 text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors"
                >
                  <FontAwesomeIcon icon={faUserMinus} />
                </button>
              </div>
            ))}
            {availablePeers.length > 0 ? (
              <div className="border-t border-gray-200 dark:border-gray-600 pt-3">
                <select
                  onChange={(e) => {
                    if (e.target.value) {
                      handleAddPeer(e.target.value);
                      e.target.value = '';
                    }
                  }}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                >
                  <option value="">Add peer to group...</option>
                  {availablePeers.map((peer) => (
                    <option key={peer.id} value={peer.id}>
                      {peer.name} ({peer.address}) - {peer.is_jump ? 'Jump' : 'Regular'}
                    </option>
                  ))}
                </select>
              </div>
            ) : (
              <div className="text-sm text-gray-500 dark:text-gray-400 text-center py-4">
                All peers in this network are already in this group
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

        {/* Policies Tab */}
        {activeTab === 'policies' && (
          <div className="p-6">
          <div className="space-y-3">
            {policies.length > 1 && (
              <div className="text-xs text-gray-500 dark:text-gray-400 mb-2">
                Order matters - policies are applied from first to last
              </div>
            )}
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
                <FontAwesomeIcon icon={faGripVertical} className="text-gray-400 dark:text-gray-500" />
                <div className="flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-mono text-gray-500 dark:text-gray-400">#{index + 1}</span>
                    <span className="text-sm font-medium text-gray-900 dark:text-white">{policy.name}</span>
                    <span className="text-xs text-gray-500 dark:text-gray-400">({policy.rules?.length || 0} rules)</span>
                  </div>
                  {policy.description && (
                    <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">{policy.description}</div>
                  )}
                </div>
                <div className="flex items-center gap-1">
                  {index > 0 && (
                    <button
                      onClick={() => handleReorderPolicies(index, index - 1)}
                      className="p-1.5 text-gray-600 hover:text-primary-600 dark:text-gray-400 dark:hover:text-primary-400"
                    >
                      <FontAwesomeIcon icon={faArrowUp} className="text-sm" />
                    </button>
                  )}
                  {index < policies.length - 1 && (
                    <button
                      onClick={() => handleReorderPolicies(index, index + 1)}
                      className="p-1.5 text-gray-600 hover:text-primary-600 dark:text-gray-400 dark:hover:text-primary-400"
                    >
                      <FontAwesomeIcon icon={faArrowDown} className="text-sm" />
                    </button>
                  )}
                  <button
                    onClick={() => {
                      if (confirm(`Detach policy "${policy.name}" from this group?`)) {
                        handleDetachPolicy(policy.id);
                      }
                    }}
                    className="p-1.5 text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded ml-1"
                  >
                    <FontAwesomeIcon icon={faTrash} className="text-sm" />
                  </button>
                </div>
              </div>
            ))}
            {availablePolicies.length > 0 ? (
              <div className="border-t border-gray-200 dark:border-gray-600 pt-3">
                <select
                  onChange={(e) => {
                    if (e.target.value) {
                      handleAttachPolicy(e.target.value);
                      e.target.value = '';
                    }
                  }}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                >
                  <option value="">Attach policy to group...</option>
                  {availablePolicies.map((policy) => (
                    <option key={policy.id} value={policy.id}>
                      {policy.name} ({policy.rules?.length || 0} rules)
                    </option>
                  ))}
                </select>
              </div>
            ) : (
              <div className="text-sm text-gray-500 dark:text-gray-400 text-center py-4">
                All policies in this network are already attached to this group
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

        {/* Routes Tab */}
        {activeTab === 'routes' && (
          <div className="p-6">
          <div className="space-y-3">
            {routes.map((route) => (
              <div key={route.id} className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                <div className="flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-gray-900 dark:text-white">{route.name}</span>
                    <span className="text-xs font-mono bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200 px-2 py-0.5 rounded">
                      {route.destination_cidr}
                    </span>
                  </div>
                  {route.description && (
                    <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">{route.description}</div>
                  )}
                </div>
                <button
                  onClick={() => {
                    if (confirm(`Detach route "${route.name}" from this group?`)) {
                      handleDetachRoute(route.id);
                    }
                  }}
                  className="p-2 text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded"
                >
                  <FontAwesomeIcon icon={faTrash} />
                </button>
              </div>
            ))}
            {availableRoutes.length > 0 ? (
              <div className="border-t border-gray-200 dark:border-gray-600 pt-3">
                <select
                  onChange={(e) => {
                    if (e.target.value) {
                      handleAttachRoute(e.target.value);
                      e.target.value = '';
                    }
                  }}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                >
                  <option value="">Attach route to group...</option>
                  {availableRoutes.map((route) => (
                    <option key={route.id} value={route.id}>
                      {route.name} ({route.destination_cidr})
                    </option>
                  ))}
                </select>
              </div>
            ) : (
              <div className="text-sm text-gray-500 dark:text-gray-400 text-center py-4">
                All routes in this network are already attached to this group
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
        </div>
      </div>
      </div>
    </div>
  );
}
