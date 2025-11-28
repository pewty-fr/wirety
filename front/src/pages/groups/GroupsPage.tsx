import { useState, useEffect, useCallback } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faUsers, faPencil, faTrash, faUserPlus, faUserMinus, faShieldAlt, faRoute } from '@fortawesome/free-solid-svg-icons';
import PageHeader from '../../components/PageHeader';
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
        {/* Network Selector */}
        <div className="mb-6">
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
            Select Network
          </label>
          <select
            value={selectedNetworkId}
            onChange={(e) => setSelectedNetworkId(e.target.value)}
            className="w-full px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
          >
            <option value="">Select a network...</option>
            {networks.map((network) => (
              <option key={network.id} value={network.id}>
                {network.name} ({network.cidr})
              </option>
            ))}
          </select>
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
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (group) {
      setName(group.name);
      setDescription(group.description || '');
    } else {
      setName('');
      setDescription('');
    }
  }, [group]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);

    try {
      if (group) {
        await api.updateGroup(networkId, group.id, { name, description });
      } else {
        await api.createGroup(networkId, { name, description });
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
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-md">
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
      // Load all network peers
      const allPeers = await api.getAllNetworkPeers(networkId);
      const groupPeerIds = new Set(group.peer_ids || []);
      setPeers(allPeers.filter(p => groupPeerIds.has(p.id)));
      setAvailablePeers(allPeers.filter(p => !groupPeerIds.has(p.id)));

      // Load group policies and all network policies
      const [groupPolicies, allPolicies] = await Promise.all([
        api.getGroupPolicies(networkId, group.id),
        api.getPolicies(networkId),
      ]);
      setPolicies(groupPolicies);
      const groupPolicyIds = new Set(groupPolicies.map(p => p.id));
      setAvailablePolicies(allPolicies.filter(p => !groupPolicyIds.has(p.id)));

      // Load group routes and all network routes
      const [groupRoutes, allRoutes] = await Promise.all([
        api.getGroupRoutes(networkId, group.id),
        api.getRoutes(networkId),
      ]);
      setRoutes(groupRoutes);
      const groupRouteIds = new Set(groupRoutes.map(r => r.id));
      setAvailableRoutes(allRoutes.filter(r => !groupRouteIds.has(r.id)));
    } catch (error) {
      console.error('Failed to load group details:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleAddPeer = async (peerId: string) => {
    if (!group || !networkId) return;
    try {
      await api.addPeerToGroup(networkId, group.id, peerId);
      await loadGroupDetails();
      onUpdate();
    } catch (error) {
      console.error('Failed to add peer:', error);
      alert('Failed to add peer to group');
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

  const handleAttachRoute = async (routeId: string) => {
    if (!group || !networkId) return;
    try {
      await api.attachRouteToGroup(networkId, group.id, routeId);
      await loadGroupDetails();
      onUpdate();
    } catch (error) {
      console.error('Failed to attach route:', error);
      alert('Failed to attach route to group');
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
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div className="bg-white dark:bg-gray-800 rounded-lg w-full max-w-4xl max-h-[90vh] overflow-y-auto">
        <div className="p-6 border-b border-gray-200 dark:border-gray-700">
          <h2 className="text-2xl font-bold text-gray-900 dark:text-white">{group.name}</h2>
          {group.description && (
            <p className="text-gray-600 dark:text-gray-400 mt-1">{group.description}</p>
          )}
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
              </h3>
              <div className="space-y-2">
                {policies.map((policy) => (
                  <div key={policy.id} className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                    <div>
                      <span className="text-sm font-medium text-gray-900 dark:text-white">{policy.name}</span>
                      <span className="text-xs text-gray-500 dark:text-gray-400 ml-2">({policy.rules?.length || 0} rules)</span>
                    </div>
                    <button
                      onClick={() => handleDetachPolicy(policy.id)}
                      className="text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300"
                      title="Detach policy"
                    >
                      <FontAwesomeIcon icon={faTrash} />
                    </button>
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
  );
}

