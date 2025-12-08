import { useState, useEffect, useCallback, useMemo } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faShieldAlt, faPencil, faTrash, faPlus, faUsers, faList } from '@fortawesome/free-solid-svg-icons';
import PageHeader from '../../components/PageHeader';
import SearchableSelect from '../../components/SearchableSelect';
import api from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';
import type { Policy, PolicyRule, Network, Route } from '../../types';

export default function PoliciesPage() {
  const { user } = useAuth();
  const [networks, setNetworks] = useState<Network[]>([]);
  const [selectedNetworkId, setSelectedNetworkId] = useState<string>('');
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [loading, setLoading] = useState(true);
  const [isPolicyModalOpen, setIsPolicyModalOpen] = useState(false);
  const [editingPolicy, setEditingPolicy] = useState<Policy | null>(null);

  const isAdmin = user?.role === 'administrator';

  // Memoized network options for SearchableSelect
  const networkOptions = useMemo(() => networks.map(network => ({
    value: network.id,
    label: `${network.name} (${network.cidr})`
  })), [networks]);

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

  const loadPolicies = useCallback(async () => {
    setLoading(true);
    try {
      let data;
      if (selectedNetworkId) {
        // Load policies for specific network
        data = await api.getPolicies(selectedNetworkId);
      } else {
        // Load all policies from all networks
        data = await api.getAllPolicies();
      }
      setPolicies(data || []);
    } catch (error) {
      console.error('Failed to load policies:', error);
      setPolicies([]);
    } finally {
      setLoading(false);
    }
  }, [selectedNetworkId]);

  useEffect(() => {
    void loadPolicies();
  }, [loadPolicies]);

  const handlePolicyClick = (policy: Policy) => {
    // Open the edit modal directly (which has all the tabs and functionality)
    setEditingPolicy(policy);
    setIsPolicyModalOpen(true);
  };

  const handleCreate = () => {
    setEditingPolicy(null);
    setIsPolicyModalOpen(true);
  };

  const handleEdit = (policy: Policy, e?: React.MouseEvent) => {
    if (e) e.stopPropagation();
    setEditingPolicy(policy);
    setIsPolicyModalOpen(true);
  };

  const handleDeletePolicy = async (policyId: string) => {
    try {
      await api.deletePolicy(selectedNetworkId, policyId);
      loadPolicies();
    } catch (error) {
      console.error('Failed to delete policy:', error);
      alert('Failed to delete policy');
    }
  };

  const handleDelete = async (policy: Policy, e: React.MouseEvent) => {
    e.stopPropagation();
    if (confirm(`Are you sure you want to delete policy "${policy.name}"?`)) {
      await handleDeletePolicy(policy.id);
    }
  };

  if (!isAdmin) {
    return (
      <div className="p-8">
        <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4">
          <p className="text-yellow-800 dark:text-yellow-200">
            You need administrator privileges to manage policies.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div>
      <PageHeader 
        title="Policies" 
        subtitle={`${policies.length} polic${policies.length !== 1 ? 'ies' : 'y'} ${selectedNetworkId ? 'in selected network' : 'across all networks'}`}
        action={
          <div className="flex gap-2">
            <button
              onClick={handleCreate}
              className="px-4 py-2.5 bg-gradient-to-r from-primary-600 to-accent-blue text-white rounded-xl hover:scale-105 active:scale-95 shadow-lg hover:shadow-xl flex items-center gap-2 cursor-pointer transition-all font-semibold"
            >
              <svg className="w-5 h-5 group-hover:rotate-90 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              Policy
            </button>
          </div>
        }
      />

      <div className="p-8">
        {/* Network Filter */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 mb-6">
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Network</label>
              <SearchableSelect
                options={networkOptions}
                value={selectedNetworkId}
                onChange={setSelectedNetworkId}
                placeholder="Select a network..."
              />
            </div>
          </div>
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-gray-500">Loading policies...</div>
          </div>
        ) : policies.length === 0 ? (
          <div className="bg-gradient-to-br from-white via-gray-50 to-white dark:from-gray-800 dark:via-gray-800/50 dark:to-gray-800 rounded-2xl border border-gray-200 dark:border-gray-700 p-16 text-center shadow-sm">
            <div className="inline-flex items-center justify-center w-20 h-20 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue mb-6">
              <FontAwesomeIcon icon={faShieldAlt} className="text-3xl text-white" />
            </div>
            <h3 className="text-xl font-bold text-gray-900 dark:text-white mb-2">No policies found</h3>
            <p className="text-gray-600 dark:text-gray-300">
              Get started by creating your first policy or using a template
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
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Rules</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Created</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Actions</th>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                {policies.map((policy) => (
                  <tr
                    key={policy.id}
                    onClick={() => handlePolicyClick(policy)}
                    className="hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer"
                  >
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex items-center">
                        <div className="inline-flex items-center justify-center w-10 h-10 rounded-xl bg-gradient-to-br from-primary-500 to-accent-blue mr-3">
                          <FontAwesomeIcon icon={faShieldAlt} className="text-lg text-white" />
                        </div>
                        <div className="text-sm font-medium text-gray-900 dark:text-white">{policy.name}</div>
                      </div>
                    </td>
                    {!selectedNetworkId && (
                      <td className="px-6 py-4 text-sm text-gray-900 dark:text-white">
                        {policy.network_name || '-'}
                      </td>
                    )}
                    <td className="px-6 py-4 text-sm text-gray-900 dark:text-white">
                      {policy.description || '-'}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                      {policy.rules?.length || 0}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                      {new Date(policy.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm">
                      <div className="flex items-center gap-2">
                        <button
                          onClick={(e) => handleEdit(policy, e)}
                          className="text-primary-600 hover:text-primary-800 dark:text-primary-400 dark:hover:text-primary-300 transition-colors"
                          title="Edit policy"
                        >
                          <FontAwesomeIcon icon={faPencil} />
                        </button>
                        <button
                          onClick={(e) => handleDelete(policy, e)}
                          className="text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300 transition-colors"
                          title="Delete policy"
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

      <PolicyModal
        isOpen={isPolicyModalOpen}
        onClose={() => {
          setIsPolicyModalOpen(false);
          setEditingPolicy(null);
        }}
        onSuccess={loadPolicies}
        networkId={selectedNetworkId}
        policy={editingPolicy}
        networks={networks}
      />
    </div>
  );
}

// Policy Modal Component
function PolicyModal({
  isOpen,
  onClose,
  onSuccess,
  networkId,
  policy,
  networks,
}: {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  networkId: string;
  policy: Policy | null;
  networks: Network[];
}) {
  const [activeTab, setActiveTab] = useState<'details' | 'rules' | 'groups'>('details');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [selectedNetworkId, setSelectedNetworkId] = useState<string>(networkId);
  const [loading, setLoading] = useState(false);
  
  // Rules management
  const [rules, setRules] = useState<PolicyRule[]>([]);
  const [isAddRuleModalOpen, setIsAddRuleModalOpen] = useState(false);
  
  // Groups management
  const [attachedGroups, setAttachedGroups] = useState<{ id: string; name: string; network_name?: string; description?: string }[]>([]);
  const [availableGroups, setAvailableGroups] = useState<{ id: string; name: string; network_name?: string; description?: string }[]>([]);
  
  // Staged attachments for creation mode
  const [stagedRules, setStagedRules] = useState<Omit<PolicyRule, 'id'>[]>([]);
  const [stagedGroupIds, setStagedGroupIds] = useState<string[]>([]);

  // Individual edit modes
  const [isEditingName, setIsEditingName] = useState(false);
  const [isEditingDescription, setIsEditingDescription] = useState(false);

  useEffect(() => {
    if (policy) {
      setName(policy.name);
      setDescription(policy.description || '');
      setRules(policy.rules || []);
      setSelectedNetworkId(networkId);
      setStagedRules([]);
      setStagedGroupIds([]);
      // Reset edit modes
      setIsEditingName(false);
      setIsEditingDescription(false);
      loadAttachments();
    } else {
      setName('');
      setDescription('');
      setRules([]);
      setSelectedNetworkId(networkId || '');
      setStagedRules([]);
      setStagedGroupIds([]);
      setAttachedGroups([]);
      // Reset edit modes
      setIsEditingName(false);
      setIsEditingDescription(false);
      if (selectedNetworkId) {
        loadAvailableItems();
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [policy, networkId, isOpen]);

  // Load available items when network changes in creation mode
  useEffect(() => {
    if (!policy && selectedNetworkId && isOpen) {
      loadAvailableItems();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedNetworkId, policy, isOpen]);

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
    if (!policy || !selectedNetworkId) return;

    try {
      // Load all groups in the network
      const allGroups = await api.getGroups(selectedNetworkId);
      
      // Filter groups that have this policy attached
      const groupsWithPolicy = allGroups.filter(g => g.policy_ids?.includes(policy.id));
      setAttachedGroups(groupsWithPolicy);
      
      // Available groups are those without this policy
      const groupsWithoutPolicy = allGroups.filter(g => !g.policy_ids?.includes(policy.id));
      setAvailableGroups(groupsWithoutPolicy);
    } catch (error) {
      console.error('Failed to load groups:', error);
    }
  };

  const handleCreatePolicy = async () => {
    if (!selectedNetworkId) {
      alert('Please select a network');
      return;
    }
    
    setLoading(true);
    try {
      const newPolicy = await api.createPolicy(selectedNetworkId, { name, description });
      
      // Attach staged items
      const attachPromises = [];
      
      // Add staged rules
      for (const rule of stagedRules) {
        attachPromises.push(api.addRuleToPolicy(selectedNetworkId, newPolicy.id, rule));
      }
      
      // Attach to staged groups
      for (const groupId of stagedGroupIds) {
        attachPromises.push(api.attachPolicyToGroup(selectedNetworkId, groupId, newPolicy.id));
      }
      
      // Wait for all attachments to complete
      if (attachPromises.length > 0) {
        await Promise.all(attachPromises);
      }
      
      onSuccess();
      onClose();
    } catch (error) {
      console.error('Failed to create policy:', error);
      alert('Failed to create policy');
    } finally {
      setLoading(false);
    }
  };

  const handleUpdateName = async () => {
    if (!policy) return;
    
    const networkIdToUse = policy.network_id || selectedNetworkId;
    if (!networkIdToUse) return;
    
    try {
      await api.updatePolicy(networkIdToUse, policy.id, { name });
      const updatedPolicy = await api.getPolicy(networkIdToUse, policy.id);
      Object.assign(policy, updatedPolicy);
      setIsEditingName(false);
      onSuccess();
    } catch (error) {
      console.error('Failed to update name:', error);
      alert('Failed to update policy name');
    }
  };

  const handleUpdateDescription = async () => {
    if (!policy) return;
    
    const networkIdToUse = policy.network_id || selectedNetworkId;
    if (!networkIdToUse) return;
    
    try {
      await api.updatePolicy(networkIdToUse, policy.id, { description });
      const updatedPolicy = await api.getPolicy(networkIdToUse, policy.id);
      Object.assign(policy, updatedPolicy);
      setIsEditingDescription(false);
      onSuccess();
    } catch (error) {
      console.error('Failed to update description:', error);
      alert('Failed to update policy description');
    }
  };

  const handleCancelEdit = (field: 'name' | 'description') => {
    switch (field) {
      case 'name':
        setName(policy?.name || '');
        setIsEditingName(false);
        break;
      case 'description':
        setDescription(policy?.description || '');
        setIsEditingDescription(false);
        break;
    }
  };

  const handleAddRule = async (rule: Omit<PolicyRule, 'id'>) => {
    if (rule.target_type === 'route') rule.target_type = 'cidr';
    
    if (policy) {
      const networkIdToUse = policy.network_id || selectedNetworkId;
      if (!networkIdToUse) return;
      
      // Edit mode: add immediately
      try {
        await api.addRuleToPolicy(networkIdToUse, policy.id, rule);
        const updatedPolicy = await api.getPolicy(networkIdToUse, policy.id);
        setRules(updatedPolicy.rules || []);
        onSuccess();
        setIsAddRuleModalOpen(false);
      } catch (error) {
        console.error('Failed to add rule:', error);
        alert('Failed to add rule to policy');
      }
    } else {
      // Creation mode: stage for later
      setStagedRules([...stagedRules, rule]);
      // Add to rules with a temporary ID for display
      setRules([...rules, { ...rule, id: `temp-${Date.now()}` } as PolicyRule]);
      setIsAddRuleModalOpen(false);
    }
  };

  const handleRemoveRule = async (ruleId: string) => {
    if (policy) {
      const networkIdToUse = policy.network_id || selectedNetworkId;
      if (!networkIdToUse) return;
      
      // Edit mode: remove immediately
      try {
        await api.removeRuleFromPolicy(networkIdToUse, policy.id, ruleId);
        const updatedPolicy = await api.getPolicy(networkIdToUse, policy.id);
        setRules(updatedPolicy.rules || []);
        onSuccess();
      } catch (error) {
        console.error('Failed to remove rule:', error);
        alert('Failed to remove rule from policy');
      }
    } else {
      // Creation mode: unstage
      const ruleIndex = rules.findIndex(r => r.id === ruleId);
      if (ruleIndex >= 0) {
        setRules(rules.filter(r => r.id !== ruleId));
        setStagedRules(stagedRules.filter((_, i) => i !== ruleIndex));
      }
    }
  };

  const handleAttachToGroup = async (groupId: string) => {
    const group = availableGroups.find(g => g.id === groupId);
    if (!group) return;
    
    if (policy && selectedNetworkId) {
      // Edit mode: attach immediately
      try {
        await api.attachPolicyToGroup(selectedNetworkId, groupId, policy.id);
        // Refetch the updated policy data and reload attachments
        const updatedPolicy = await api.getPolicy(selectedNetworkId, policy.id);
        Object.assign(policy, updatedPolicy); // Update the policy object with fresh data
        await loadAttachments();
        onSuccess();
      } catch (error) {
        console.error('Failed to attach policy to group:', error);
        alert('Failed to attach policy to group');
      }
    } else {
      // Creation mode: stage for later
      setStagedGroupIds([...stagedGroupIds, groupId]);
      setAttachedGroups([...attachedGroups, group]);
      setAvailableGroups(availableGroups.filter(g => g.id !== groupId));
    }
  };

  const handleDetachFromGroup = async (groupId: string) => {
    if (policy && selectedNetworkId) {
      // Edit mode: detach immediately
      try {
        await api.detachPolicyFromGroup(selectedNetworkId, groupId, policy.id);
        // Refetch the updated policy data and reload attachments
        const updatedPolicy = await api.getPolicy(selectedNetworkId, policy.id);
        Object.assign(policy, updatedPolicy); // Update the policy object with fresh data
        await loadAttachments();
        onSuccess();
      } catch (error) {
        console.error('Failed to detach policy from group:', error);
        alert('Failed to detach policy from group');
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
        {policy && (
          <div className="flex items-start justify-between mb-6">
            <div className="flex items-start gap-4">
              <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue">
                <FontAwesomeIcon icon={faShieldAlt} className="text-2xl text-white" />
              </div>
              <div>
                <h3 className="text-2xl font-bold bg-gradient-to-r from-gray-900 to-gray-600 dark:from-gray-100 dark:to-gray-300 bg-clip-text text-transparent">{policy.name}</h3>
                <p className="text-sm text-gray-600 dark:text-gray-300 mt-1">ID: {policy.id}</p>
              </div>
            </div>
          </div>
        )}
        
        {!policy && (
          <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-4">
            Create Policy
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
              onClick={() => setActiveTab('rules')}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                activeTab === 'rules'
                  ? 'border-primary-600 text-primary-600 dark:text-primary-400'
                  : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
              }`}
            >
              <FontAwesomeIcon icon={faList} className="mr-1" />
              Rules ({rules.length})
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
        </div>

        {/* Details Tab */}
        {activeTab === 'details' && (
          <div className="p-6">
            <div className="space-y-6">
              {!policy && (
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
                {policy && !isEditingName ? (
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
                      placeholder="Enter policy name..."
                    />
                    {policy && (
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
                {policy && !isEditingDescription ? (
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
                      placeholder="Enter policy description..."
                    />
                    {policy && (
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
            </div>

            {/* Create Button for New Policies */}
            {!policy && (
              <div className="mt-6 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={onClose}
                  className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600"
                >
                  Cancel
                </button>
                <button
                  onClick={handleCreatePolicy}
                  disabled={loading || !name.trim() || !selectedNetworkId}
                  className="px-4 py-2 text-sm font-medium text-white bg-gradient-to-r from-primary-600 to-accent-blue rounded-lg hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {loading ? 'Creating...' : 'Create Policy'}
                </button>
              </div>
            )}

            {/* Close Button for Existing Policies */}
            {policy && (
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

        {/* Rules Tab */}
        {activeTab === 'rules' && (
          <div className="p-6">
          <div className="space-y-3">
            <div className="flex justify-between items-center mb-3">
              <p className="text-sm text-gray-600 dark:text-gray-400">
                Manage firewall rules for this policy
              </p>
              <button
                onClick={() => setIsAddRuleModalOpen(true)}
                className="px-3 py-1.5 text-sm bg-gradient-to-r from-primary-600 to-accent-blue text-white rounded-lg hover:scale-105"
              >
                <FontAwesomeIcon icon={faPlus} className="mr-2" />
                Add Rule
              </button>
            </div>
            {rules.length === 0 ? (
              <div className="text-center py-8 text-gray-500">
                No rules defined. Add rules to control traffic.
              </div>
            ) : (
              <div className="space-y-2">
                {rules.map((rule) => (
                  <div key={rule.id} className="flex items-center justify-between p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-1">
                        <span className={`px-2 py-1 text-xs font-semibold rounded ${
                          rule.action === 'allow' 
                            ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                            : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                        }`}>
                          {rule.action.toUpperCase()}
                        </span>
                        <span className="px-2 py-1 text-xs font-semibold rounded bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">
                          {rule.direction.toUpperCase()}
                        </span>
                        <span className="text-sm font-mono text-gray-900 dark:text-white">
                          {rule.target_type}: {rule.target}
                        </span>
                      </div>
                      {rule.description && (
                        <p className="text-sm text-gray-600 dark:text-gray-400">{rule.description}</p>
                      )}
                    </div>
                    <button
                      onClick={() => {
                        if (confirm('Remove this rule?')) {
                          handleRemoveRule(rule.id);
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
              Groups that have this policy attached
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
                    if (confirm(`Detach this policy from group "${group.name}"?`)) {
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
                This policy is attached to all groups in the network
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

        {/* Add Rule Modal */}
        <AddRuleModal
          isOpen={isAddRuleModalOpen}
          onClose={() => setIsAddRuleModalOpen(false)}
          onAdd={handleAddRule}
          networkId={selectedNetworkId}
        />
      </div>
      </div>
    </div>
  );
}

// PolicyDetailModal removed - clicking on a policy now opens the PolicyModal directly with all tabs

// Add Rule Modal Component
function AddRuleModal({
  isOpen,
  onClose,
  onAdd,
  networkId,
}: {
  isOpen: boolean;
  onClose: () => void;
  onAdd: (rule: Omit<PolicyRule, 'id'>) => void;
  networkId: string;
}) {
  const [direction, setDirection] = useState<'input' | 'output'>('output');
  const [action, setAction] = useState<'allow' | 'deny'>('allow');
  const [targetType, setTargetType] = useState<'cidr' | 'peer' | 'group' | 'route' | 'network'>('cidr');
  const [target, setTarget] = useState('');
  const [description, setDescription] = useState('');
  const [routes, setRoutes] = useState<Route[]>([]);
  const [loadingRoutes, setLoadingRoutes] = useState(false);
  const [networkCIDR, setNetworkCIDR] = useState<string>('');

  // Load network CIDR and routes when modal opens
  useEffect(() => {
    if (!isOpen || !networkId) return;
    
    const loadData = async () => {
      try {
        // Load network CIDR
        const network = await api.getNetwork(networkId);
        setNetworkCIDR(network.cidr);
        
        // Load routes if needed
        if (targetType === 'route') {
          setLoadingRoutes(true);
          try {
            const routesData = await api.getRoutes(networkId);
            setRoutes(routesData);
          } catch (error) {
            console.error('Failed to load routes:', error);
          } finally {
            setLoadingRoutes(false);
          }
        }
      } catch (error) {
        console.error('Failed to load network:', error);
      }
    };
    
    void loadData();
  }, [targetType, networkId, isOpen]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    // Convert "network" and "route" to "cidr" for backend (they're just frontend helpers)
    const actualTargetType = (targetType === 'network' || targetType === 'route') ? 'cidr' : targetType;
    onAdd({ direction, action, target_type: actualTargetType, target, description });
    setDirection('input');
    setAction('allow');
    setTargetType('cidr');
    setTarget('');
    setDescription('');
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
        <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-4">Add Rule</h2>
        <form onSubmit={handleSubmit}>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Direction *
              </label>
              <select
                value={direction}
                onChange={(e) => setDirection(e.target.value as 'input' | 'output')}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              >
                <option value="output">Output (from peer to target)</option>
                <option value="input">Input (from target to peer)</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Action *
              </label>
              <select
                value={action}
                onChange={(e) => setAction(e.target.value as 'allow' | 'deny')}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              >
                <option value="allow">Allow</option>
                <option value="deny">Deny</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Target Type *
              </label>
              <select
                value={targetType}
                onChange={(e) => {
                  const newType = e.target.value as 'cidr' | 'peer' | 'group' | 'route' | 'network';
                  setTargetType(newType);
                  // Auto-fill network CIDR when "network" is selected
                  if (newType === 'network') {
                    setTarget(networkCIDR);
                  } else {
                    setTarget(''); // Reset target when type changes
                  }
                }}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              >
                <option value="network">Network</option>
                <option value="cidr">CIDR</option>
                <option value="peer">Peer</option>
                <option value="group">Group</option>
                <option value="route">Route</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Target *
              </label>
              {targetType === 'route' ? (
                <select
                  value={target}
                  onChange={(e) => setTarget(e.target.value)}
                  required
                  disabled={loadingRoutes}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                >
                  <option value="">Select a route...</option>
                  {routes.map((route) => (
                    <option key={route.id} value={route.destination_cidr}>
                      {route.name} ({route.destination_cidr})
                    </option>
                  ))}
                </select>
              ) : targetType === 'network' ? (
                <input
                  type="text"
                  value={target}
                  onChange={(e) => setTarget(e.target.value)}
                  placeholder="Network CIDR"
                  required
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                />
              ) : (
                <input
                  type="text"
                  value={target}
                  onChange={(e) => setTarget(e.target.value)}
                  placeholder={targetType === 'cidr' ? '10.0.0.0/24' : 'ID'}
                  required
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                />
              )}
              {targetType === 'network' && (
                <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  This will be sent as CIDR type to the backend
                </p>
              )}
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
              Add Rule
            </button>
          </div>
        </form>
        </div>
      </div>
      </div>
    </div>
  );
}

