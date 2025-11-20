import { useState, useEffect } from 'react';
import Modal from './Modal';
import api from '../api/client';
import type { Peer, Network } from '../types';

interface JumpPeerModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  networkId: string;
  networks?: Network[];
  peer?: Peer | null;
}

export default function JumpPeerModal({ isOpen, onClose, onSuccess, networkId, networks = [], peer }: JumpPeerModalProps) {
  const [formData, setFormData] = useState({
    name: '',
    endpoint: '',
    listen_port: 51820,
    jump_nat_interface: '',
    use_agent: true, // Always use agent mode for jump peers
  });
  const [selectedNetworkId, setSelectedNetworkId] = useState(networkId);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const isEditMode = !!peer;

  useEffect(() => {
    if (peer) {
      setFormData({
        name: peer.name,
        endpoint: peer.endpoint || '',
        listen_port: peer.listen_port || 51820,
        jump_nat_interface: peer.jump_nat_interface || '',
        use_agent: peer.use_agent,
      });
    } else {
      setFormData({
        name: '',
        endpoint: '',
        listen_port: 51820,
        jump_nat_interface: '',
        use_agent: true, // Always use agent mode for jump peers
      });
      setSelectedNetworkId(networkId || (networks.length > 0 ? networks[0].id : ''));
    }
    setError(null);
  }, [peer, isOpen, networkId]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      if (isEditMode && peer) {
        await api.updatePeer(networkId, peer.id, {
          name: formData.name,
          endpoint: formData.endpoint || undefined,
          listen_port: formData.listen_port,
        });
      } else {
        await api.createPeer(selectedNetworkId, {
          name: formData.name,
          endpoint: formData.endpoint || undefined,
          listen_port: formData.listen_port,
          is_jump: true,
          jump_nat_interface: formData.jump_nat_interface || undefined,
          use_agent: formData.use_agent,
        });
      }
      onSuccess();
      onClose();
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to save jump peer');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={isEditMode ? 'Edit Jump Peer' : 'Create Jump Peer'}
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
            placeholder="e.g., Jump Server 1"
          />
        </div>

        {/* Endpoint */}
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Endpoint
          </label>
          <input
            type="text"
            value={formData.endpoint}
            onChange={(e) => setFormData({ ...formData, endpoint: e.target.value })}
            className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            placeholder="e.g., vpn.example.com or 203.0.113.1"
          />
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">IP address or domain (no port needed)</p>
        </div>

        {/* Listen Port */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Listen Port
          </label>
          <input
            type="number"
            value={formData.listen_port}
            onChange={(e) => setFormData({ ...formData, listen_port: parseInt(e.target.value) })}
            className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            min={1}
            max={65535}
          />
          <p className="mt-1 text-sm text-gray-500">WireGuard listen port (default: 51820)</p>
        </div>

        {/* NAT Interface (only for create) */}
        {!isEditMode && (
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              NAT Interface
            </label>
            <input
              type="text"
              value={formData.jump_nat_interface}
              onChange={(e) => setFormData({ ...formData, jump_nat_interface: e.target.value })}
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 focus:ring-2 focus:ring-primary-500 focus:border-transparent"
              placeholder="e.g., eth0"
            />
            <p className="mt-1 text-sm text-gray-500">Network interface for NAT/masquerading</p>
          </div>
        )}

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
