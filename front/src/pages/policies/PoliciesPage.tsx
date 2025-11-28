import { useState, useEffect, useCallback } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faShieldAlt, faPencil, faTrash, faPlus, faCopy } from '@fortawesome/free-solid-svg-icons';
import PageHeader from '../../components/PageHeader';
import api from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';
import type { Policy, PolicyRule, Network } from '../../types';

export default function PoliciesPage() {
  const { user } = useAuth();
  const [networks, setNetworks] = useState<Network[]>([]);
  const [selectedNetworkId, setSelectedNetworkId] = useState<string>('');
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [loading, setLoading] = useState(true);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingPolicy, setEditingPolicy] = useState<Policy | null>(null);
  const [selectedPolicy, setSelectedPolicy] = useState<Policy | null>(null);
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false);
  const [isTemplateModalOpen, setIsTemplateModalOpen] = useState(false);

  const isAdmin = user?.role === 'administrator';

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

  const loadPolicies = useCallback(async () => {
    if (!selectedNetworkId) {
      setPolicies([]);
      setLoading(false);
      return;
    }

    setLoading(true);
    try {
      const data = await api.getPolicies(selectedNetworkId);
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
    setSelectedPolicy(policy);
    setIsDetailModalOpen(true);
  };

  const handleCreate = () => {
    setEditingPolicy(null);
    setIsModalOpen(true);
  };

  const handleEdit = (policy: Policy, e: React.MouseEvent) => {
    e.stopPropagation();
    setEditingPolicy(policy);
    setIsModalOpen(true);
  };

  const handleDelete = async (policy: Policy, e: React.MouseEvent) => {
    e.stopPropagation();
    if (confirm(`Are you sure you want to delete policy "${policy.name}"?`)) {
      try {
        await api.deletePolicy(selectedNetworkId, policy.id);
        loadPolicies();
      } catch (error) {
        console.error('Failed to delete policy:', error);
        alert('Failed to delete policy');
      }
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
        subtitle={`${policies.length} polic${policies.length !== 1 ? 'ies' : 'y'} in selected network`}
        action={
          selectedNetworkId ? (
            <div className="flex gap-2">
              <button
                onClick={() => setIsTemplateModalOpen(true)}
                className="px-4 py-2.5 bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-300 border border-gray-300 dark:border-gray-600 rounded-xl hover:scale-105 active:scale-95 shadow-lg hover:shadow-xl flex items-center gap-2 cursor-pointer transition-all font-semibold"
              >
                <FontAwesomeIcon icon={faCopy} />
                Templates
              </button>
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
          ) : undefined
        }
      />

      <div className="p-8">
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

        {!selectedNetworkId ? (
          <div className="bg-gradient-to-br from-white via-gray-50 to-white dark:from-gray-800 dark:via-gray-800/50 dark:to-gray-800 rounded-2xl border border-gray-200 dark:border-gray-700 p-16 text-center shadow-sm">
            <div className="inline-flex items-center justify-center w-20 h-20 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue mb-6">
              <FontAwesomeIcon icon={faShieldAlt} className="text-3xl text-white" />
            </div>
            <h3 className="text-xl font-bold text-gray-900 dark:text-white mb-2">Select a network</h3>
            <p className="text-gray-600 dark:text-gray-300">
              Choose a network from the dropdown above to view and manage policies
            </p>
          </div>
        ) : loading ? (
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
        isOpen={isModalOpen}
        onClose={() => {
          setIsModalOpen(false);
          setEditingPolicy(null);
        }}
        onSuccess={loadPolicies}
        networkId={selectedNetworkId}
        policy={editingPolicy}
      />

      <PolicyDetailModal
        isOpen={isDetailModalOpen}
        onClose={() => {
          setIsDetailModalOpen(false);
          setSelectedPolicy(null);
        }}
        policy={selectedPolicy}
        networkId={selectedNetworkId}
        onUpdate={loadPolicies}
      />

      <TemplateModal
        isOpen={isTemplateModalOpen}
        onClose={() => setIsTemplateModalOpen(false)}
        networkId={selectedNetworkId}
        onSuccess={loadPolicies}
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
}: {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  networkId: string;
  policy: Policy | null;
}) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (policy) {
      setName(policy.name);
      setDescription(policy.description || '');
    } else {
      setName('');
      setDescription('');
    }
  }, [policy]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);

    try {
      if (policy) {
        await api.updatePolicy(networkId, policy.id, { name, description });
      } else {
        await api.createPolicy(networkId, { name, description });
      }
      onSuccess();
      onClose();
    } catch (error) {
      console.error('Failed to save policy:', error);
      alert('Failed to save policy');
    } finally {
      setLoading(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-md">
        <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-4">
          {policy ? 'Edit Policy' : 'Create Policy'}
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
              {loading ? 'Saving...' : policy ? 'Update' : 'Create'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// Policy Detail Modal Component
function PolicyDetailModal({
  isOpen,
  onClose,
  policy,
  networkId,
  onUpdate,
}: {
  isOpen: boolean;
  onClose: () => void;
  policy: Policy | null;
  networkId: string;
  onUpdate: () => void;
}) {
  const [rules, setRules] = useState<PolicyRule[]>([]);
  const [isAddRuleModalOpen, setIsAddRuleModalOpen] = useState(false);

  useEffect(() => {
    if (policy) {
      // Sync rules state when policy changes
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setRules(policy.rules || []);
    }
  }, [policy]);

  const handleRemoveRule = async (ruleId: string) => {
    if (!policy || !networkId) return;
    try {
      await api.removeRuleFromPolicy(networkId, policy.id, ruleId);
      const updatedPolicy = await api.getPolicy(networkId, policy.id);
      setRules(updatedPolicy.rules || []);
      onUpdate();
    } catch (error) {
      console.error('Failed to remove rule:', error);
      alert('Failed to remove rule from policy');
    }
  };

  const handleAddRule = async (rule: Omit<PolicyRule, 'id'>) => {
    if (!policy || !networkId) return;
    try {
      await api.addRuleToPolicy(networkId, policy.id, rule);
      const updatedPolicy = await api.getPolicy(networkId, policy.id);
      setRules(updatedPolicy.rules || []);
      onUpdate();
      setIsAddRuleModalOpen(false);
    } catch (error) {
      console.error('Failed to add rule:', error);
      alert('Failed to add rule to policy');
    }
  };

  if (!isOpen || !policy) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div className="bg-white dark:bg-gray-800 rounded-lg w-full max-w-4xl max-h-[90vh] overflow-y-auto">
        <div className="p-6 border-b border-gray-200 dark:border-gray-700">
          <h2 className="text-2xl font-bold text-gray-900 dark:text-white">{policy.name}</h2>
          {policy.description && (
            <p className="text-gray-600 dark:text-gray-400 mt-1">{policy.description}</p>
          )}
        </div>

        <div className="p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
              Rules ({rules.length})
            </h3>
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
                    onClick={() => handleRemoveRule(rule.id)}
                    className="text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300 ml-4"
                    title="Remove rule"
                  >
                    <FontAwesomeIcon icon={faTrash} />
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="p-6 border-t border-gray-200 dark:border-gray-700 flex justify-end">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600"
          >
            Close
          </button>
        </div>
      </div>

      <AddRuleModal
        isOpen={isAddRuleModalOpen}
        onClose={() => setIsAddRuleModalOpen(false)}
        onAdd={handleAddRule}
      />
    </div>
  );
}

// Add Rule Modal Component
function AddRuleModal({
  isOpen,
  onClose,
  onAdd,
}: {
  isOpen: boolean;
  onClose: () => void;
  onAdd: (rule: Omit<PolicyRule, 'id'>) => void;
}) {
  const [direction, setDirection] = useState<'input' | 'output'>('input');
  const [action, setAction] = useState<'allow' | 'deny'>('allow');
  const [targetType, setTargetType] = useState<'cidr' | 'peer' | 'group'>('cidr');
  const [target, setTarget] = useState('');
  const [description, setDescription] = useState('');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onAdd({ direction, action, target_type: targetType, target, description });
    setDirection('input');
    setAction('allow');
    setTargetType('cidr');
    setTarget('');
    setDescription('');
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-[60]">
      <div className="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-md">
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
                <option value="input">Input</option>
                <option value="output">Output</option>
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
                onChange={(e) => setTargetType(e.target.value as 'cidr' | 'peer' | 'group')}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              >
                <option value="cidr">CIDR</option>
                <option value="peer">Peer</option>
                <option value="group">Group</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Target *
              </label>
              <input
                type="text"
                value={target}
                onChange={(e) => setTarget(e.target.value)}
                placeholder={targetType === 'cidr' ? '10.0.0.0/24' : 'ID'}
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
  );
}

// Template Modal Component
function TemplateModal({
  isOpen,
  onClose,
  networkId,
  onSuccess,
}: {
  isOpen: boolean;
  onClose: () => void;
  networkId: string;
  onSuccess: () => void;
}) {
  const [templates, setTemplates] = useState<Policy[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (isOpen && networkId) {
      loadTemplates();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, networkId]);

  const loadTemplates = async () => {
    setLoading(true);
    try {
      const data = await api.getPolicyTemplates(networkId);
      setTemplates(data || []);
    } catch (error) {
      console.error('Failed to load templates:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleUseTemplate = async (template: Policy) => {
    try {
      await api.createPolicy(networkId, {
        name: `${template.name} (Copy)`,
        description: template.description,
        rules: template.rules,
      });
      onSuccess();
      onClose();
    } catch (error) {
      console.error('Failed to create policy from template:', error);
      alert('Failed to create policy from template');
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div className="bg-white dark:bg-gray-800 rounded-lg w-full max-w-3xl max-h-[90vh] overflow-y-auto">
        <div className="p-6 border-b border-gray-200 dark:border-gray-700">
          <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Policy Templates</h2>
          <p className="text-gray-600 dark:text-gray-400 mt-1">
            Choose a template to quickly create a policy with predefined rules
          </p>
        </div>

        {loading ? (
          <div className="p-6 text-center text-gray-500">Loading templates...</div>
        ) : (
          <div className="p-6 space-y-4">
            {templates.map((template) => (
              <div key={template.id} className="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                <div className="flex items-start justify-between mb-3">
                  <div>
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-white">{template.name}</h3>
                    {template.description && (
                      <p className="text-sm text-gray-600 dark:text-gray-400 mt-1">{template.description}</p>
                    )}
                  </div>
                  <button
                    onClick={() => handleUseTemplate(template)}
                    className="px-3 py-1.5 text-sm bg-gradient-to-r from-primary-600 to-accent-blue text-white rounded-lg hover:scale-105"
                  >
                    Use Template
                  </button>
                </div>
                <div className="space-y-2">
                  <p className="text-sm font-medium text-gray-700 dark:text-gray-300">
                    Rules ({template.rules?.length || 0}):
                  </p>
                  {template.rules?.map((rule, idx) => (
                    <div key={idx} className="flex items-center gap-2 text-sm">
                      <span className={`px-2 py-0.5 text-xs font-semibold rounded ${
                        rule.action === 'allow' 
                          ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                          : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                      }`}>
                        {rule.action}
                      </span>
                      <span className="px-2 py-0.5 text-xs font-semibold rounded bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">
                        {rule.direction}
                      </span>
                      <span className="text-gray-900 dark:text-white font-mono">
                        {rule.target_type}: {rule.target}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            ))}
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
