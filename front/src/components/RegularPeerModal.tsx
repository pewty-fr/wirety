import { useState, useEffect } from 'react';
import Modal from './Modal';
import SearchableSelect from './SearchableSelect';
import { useAuth } from '../contexts/AuthContext';
import api from '../api/client';
import type { Peer, Network, User } from '../types';
import { isValidCIDR, getCIDRError } from '../utils/validation';

interface RegularPeerModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  networkId: string;
  networks?: Network[];
  peer?: Peer | null;
  users?: User[];
}

export default function RegularPeerModal({ isOpen, onClose, onSuccess, networkId, networks = [], peer, users = [] }: RegularPeerModalProps) {
  const [formData, setFormData] = useState({
    name: '',
    is_isolated: false,
    full_encapsulation: false,
    use_agent: false,
    additional_allowed_ips: [] as string[],
    owner_id: '',
  });
  const [selectedNetworkId, setSelectedNetworkId] = useState(networkId);
  const [ipInput, setIpInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ipError, setIpError] = useState<string | null>(null);
  const { user: currentUser } = useAuth();

  const isEditMode = !!peer;
  const isAdmin = currentUser?.role === 'administrator';

  // Create user options for SearchableSelect
  const userOptions = users.map(user => ({
    value: user.id,
    label: user.name + ' ' + user.email,
  }));

  useEffect(() => {
    if (peer) {
      setFormData({
        name: peer.name,
        is_isolated: peer.is_isolated,
        full_encapsulation: peer.full_encapsulation,
        use_agent: peer.use_agent,
        additional_allowed_ips: peer.additional_allowed_ips || [],
        owner_id: peer.owner_id || '',
      });
    } else {
      setFormData({
        name: '',
        is_isolated: false,
        full_encapsulation: false,
        use_agent: false,
        additional_allowed_ips: [],
        owner_id: currentUser?.id || '',
      });
      setSelectedNetworkId(networkId || (networks.length > 0 ? networks[0].id : ''));
    }
    setError(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [peer, isOpen, networkId, currentUser]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      if (isEditMode && peer) {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const updateData: any = {
          name: formData.name,
          is_isolated: formData.is_isolated,
          full_encapsulation: formData.full_encapsulation,
          additional_allowed_ips: formData.additional_allowed_ips.length > 0 ? formData.additional_allowed_ips : undefined,
        };
        // Only include owner_id if admin and it changed
        if (isAdmin && formData.owner_id !== peer.owner_id) {
          updateData.owner_id = formData.owner_id;
        }
        await api.updatePeer(networkId, peer.id, updateData);
      } else {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const createData: any = {
          name: formData.name,
          is_jump: false,
          is_isolated: formData.is_isolated,
          full_encapsulation: formData.full_encapsulation,
          use_agent: formData.use_agent,
          additional_allowed_ips: formData.additional_allowed_ips.length > 0 ? formData.additional_allowed_ips : undefined,
        };
        await api.createPeer(selectedNetworkId, createData);
      }
      onSuccess();
      onClose();
    } catch (err) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const error = err as any;
      setError(error.response?.data?.error || 'Failed to save peer');
    } finally {
      setLoading(false);
    }
  };

  const addIp = () => {
    if (ipInput.trim()) {
      if (!isValidCIDR(ipInput.trim())) {
        setIpError(getCIDRError(ipInput.trim()) || 'Invalid CIDR format');
        return;
      }
      setFormData({ ...formData, additional_allowed_ips: [...formData.additional_allowed_ips, ipInput.trim()] });
      setIpInput('');
      setIpError(null);
    }
  };

  const removeIp = (index: number) => {
    setFormData({ ...formData, additional_allowed_ips: formData.additional_allowed_ips.filter((_, i) => i !== index) });
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={isEditMode ? 'Edit Peer' : 'Create Peer'}
      size="md"
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && (
          <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded">
            {error}
          </div>
        )}

        {/* Network (only for create) */}
        {!isEditMode && (
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Network <span className="text-red-500">*</span>
            </label>
            <select
              required
              value={selectedNetworkId}
              onChange={(e) => setSelectedNetworkId(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            >
              {networks.map((network) => (
                <option key={network.id} value={network.id}>
                  {network.name} ({network.cidr})
                </option>
              ))}
            </select>
            <p className="mt-1 text-sm text-gray-500">Select the network for this peer</p>
          </div>
        )}

        {/* Name */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Name <span className="text-red-500">*</span>
          </label>
          <input
            type="text"
            required
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            placeholder="e.g., Laptop - John"
          />
        </div>

        {/* Use Agent (only for create) */}
        {!isEditMode && (
          <div>
            <label className="flex items-center space-x-2">
              <input
                type="checkbox"
                checked={formData.use_agent}
                onChange={(e) => setFormData({ ...formData, use_agent: e.target.checked })}
                className="w-4 h-4 text-primary-600 border-gray-300 rounded focus:ring-primary-500"
              />
              <span className="text-sm font-medium text-gray-700">Use Agent (dynamic configuration)</span>
            </label>
            <p className="mt-1 text-sm text-gray-500 ml-6">Enable for automatic configuration via agent</p>
          </div>
        )}

        {/* Isolated */}
        <div>
          <label className="flex items-center space-x-2">
            <input
              type="checkbox"
              checked={formData.is_isolated}
              onChange={(e) => setFormData({ ...formData, is_isolated: e.target.checked })}
              className="w-4 h-4 text-primary-600 border-gray-300 rounded focus:ring-primary-500"
            />
            <span className="text-sm font-medium text-gray-700">Isolated</span>
          </label>
          <p className="mt-1 text-sm text-gray-500 ml-6">Prevent this peer from communicating with other regular peers</p>
        </div>

        {/* Full Encapsulation */}
        <div>
          <label className="flex items-center space-x-2">
            <input
              type="checkbox"
              checked={formData.full_encapsulation}
              onChange={(e) => setFormData({ ...formData, full_encapsulation: e.target.checked })}
              className="w-4 h-4 text-primary-600 border-gray-300 rounded focus:ring-primary-500"
            />
            <span className="text-sm font-medium text-gray-700">Full Encapsulation</span>
          </label>
          <p className="mt-1 text-sm text-gray-500 ml-6">Route all traffic (0.0.0.0/0) through jump peer</p>
        </div>

        {/* Owner (admin only, edit mode only) */}
        {isAdmin && isEditMode && (
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Owner
            </label>
            <SearchableSelect
              options={userOptions}
              value={formData.owner_id}
              onChange={(value) => setFormData({ ...formData, owner_id: value })}
              placeholder="Select owner"
            />
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">Change the owner of this peer</p>
          </div>
        )}

        {/* Additional Allowed IPs */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Additional Allowed IPs
          </label>
          <div className="flex gap-2 mb-2">
            <div className="flex-1">
              <input
                type="text"
                value={ipInput}
                onChange={(e) => {
                  setIpInput(e.target.value);
                  if (e.target.value && !isValidCIDR(e.target.value)) {
                    setIpError(getCIDRError(e.target.value));
                  } else {
                    setIpError(null);
                  }
                }}
                onKeyPress={(e) => e.key === 'Enter' && (e.preventDefault(), addIp())}
                className={`w-full px-3 py-2 border rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 focus:ring-2 focus:ring-primary-500 focus:border-transparent ${
                  ipError ? 'border-red-300 dark:border-red-600' : 'border-gray-300 dark:border-gray-600'
                }`}
                placeholder="e.g., 192.168.1.0/24"
              />
              {ipError && (
                <p className="mt-1 text-sm text-red-600">{ipError}</p>
              )}
            </div>
            <button
              type="button"
              onClick={addIp}
              className="px-4 py-2 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600"
            >
              Add
            </button>
          </div>
          {formData.additional_allowed_ips.length > 0 && (
            <div className="space-y-1">
              {formData.additional_allowed_ips.map((ip, index) => (
                <div key={index} className="flex items-center justify-between bg-gray-50 dark:bg-gray-700 px-3 py-2 rounded">
                  <span className="text-sm text-gray-700">{ip}</span>
                  <button
                    type="button"
                    onClick={() => removeIp(index)}
                    className="text-red-600 hover:text-red-800"
                  >
                    Remove
                  </button>
                </div>
              ))}
            </div>
          )}
          <p className="mt-1 text-sm text-gray-500">Additional routes this peer can access</p>
        </div>

        {/* Actions */}
        <div className="flex justify-end gap-3 pt-4 border-t border-gray-200">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={loading}
            className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {loading ? 'Saving...' : isEditMode ? 'Update' : 'Create'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
