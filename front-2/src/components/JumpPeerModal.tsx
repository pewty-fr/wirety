import { useState, useEffect } from 'react';
import Modal from './Modal';
import api from '../api/client';
import type { Peer } from '../types';

interface JumpPeerModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  networkId: string;
  peer?: Peer | null;
}

export default function JumpPeerModal({ isOpen, onClose, onSuccess, networkId, peer }: JumpPeerModalProps) {
  const [formData, setFormData] = useState({
    name: '',
    endpoint: '',
    listen_port: 51820,
    jump_nat_interface: '',
    use_agent: false,
    additional_allowed_ips: [] as string[],
  });
  const [ipInput, setIpInput] = useState('');
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
        additional_allowed_ips: peer.additional_allowed_ips || [],
      });
    } else {
      setFormData({
        name: '',
        endpoint: '',
        listen_port: 51820,
        jump_nat_interface: '',
        use_agent: false,
        additional_allowed_ips: [],
      });
    }
    setError(null);
  }, [peer, isOpen]);

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
          additional_allowed_ips: formData.additional_allowed_ips.length > 0 ? formData.additional_allowed_ips : undefined,
        });
      } else {
        await api.createPeer(networkId, {
          name: formData.name,
          endpoint: formData.endpoint || undefined,
          listen_port: formData.listen_port,
          is_jump: true,
          jump_nat_interface: formData.jump_nat_interface || undefined,
          use_agent: formData.use_agent,
          additional_allowed_ips: formData.additional_allowed_ips.length > 0 ? formData.additional_allowed_ips : undefined,
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

  const addIp = () => {
    if (ipInput.trim()) {
      setFormData({ ...formData, additional_allowed_ips: [...formData.additional_allowed_ips, ipInput.trim()] });
      setIpInput('');
    }
  };

  const removeIp = (index: number) => {
    setFormData({ ...formData, additional_allowed_ips: formData.additional_allowed_ips.filter((_, i) => i !== index) });
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
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            placeholder="e.g., Jump Server 1"
          />
        </div>

        {/* Endpoint */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Endpoint
          </label>
          <input
            type="text"
            value={formData.endpoint}
            onChange={(e) => setFormData({ ...formData, endpoint: e.target.value })}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            placeholder="e.g., vpn.example.com:51820"
          />
          <p className="mt-1 text-sm text-gray-500">External endpoint (IP:port or domain:port)</p>
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
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent"
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
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent"
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

        {/* Additional Allowed IPs */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Additional Allowed IPs
          </label>
          <div className="flex gap-2 mb-2">
            <input
              type="text"
              value={ipInput}
              onChange={(e) => setIpInput(e.target.value)}
              onKeyPress={(e) => e.key === 'Enter' && (e.preventDefault(), addIp())}
              className="flex-1 px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent"
              placeholder="e.g., 192.168.1.0/24"
            />
            <button
              type="button"
              onClick={addIp}
              className="px-4 py-2 bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200"
            >
              Add
            </button>
          </div>
          {formData.additional_allowed_ips.length > 0 && (
            <div className="space-y-1">
              {formData.additional_allowed_ips.map((ip, index) => (
                <div key={index} className="flex items-center justify-between bg-gray-50 px-3 py-2 rounded">
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
            className="px-4 py-2 text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200"
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
