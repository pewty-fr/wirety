import { useState, useEffect } from 'react';
import Modal from './Modal';
import api from '../api/client';
import type { Network } from '../types';

interface NetworkModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  network?: Network | null;
}

export default function NetworkModal({ isOpen, onClose, onSuccess, network }: NetworkModalProps) {
  const [formData, setFormData] = useState({
    name: '',
    cidr: '',
    domain: '',
    dns: [] as string[],
  });
  const [dnsInput, setDnsInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const isEditMode = !!network;

  useEffect(() => {
    if (network) {
      setFormData({
        name: network.name,
        cidr: network.cidr,
        domain: network.domain,
        dns: [],
      });
    } else {
      setFormData({
        name: '',
        cidr: '',
        domain: '',
        dns: [],
      });
    }
    setError(null);
  }, [network, isOpen]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      if (isEditMode && network) {
        await api.updateNetwork(network.id, {
          name: formData.name,
          cidr: formData.cidr,
          domain: formData.domain,
        });
      } else {
        await api.createNetwork({
          name: formData.name,
          cidr: formData.cidr,
          domain: formData.domain,
          dns: formData.dns.length > 0 ? formData.dns : undefined,
        });
      }
      onSuccess();
      onClose();
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to save network');
    } finally {
      setLoading(false);
    }
  };

  const addDns = () => {
    if (dnsInput.trim()) {
      setFormData({ ...formData, dns: [...formData.dns, dnsInput.trim()] });
      setDnsInput('');
    }
  };

  const removeDns = (index: number) => {
    setFormData({ ...formData, dns: formData.dns.filter((_, i) => i !== index) });
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={isEditMode ? 'Edit Network' : 'Create Network'}
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
            placeholder="e.g., Production Network"
          />
        </div>

        {/* CIDR */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            CIDR <span className="text-red-500">*</span>
          </label>
          <input
            type="text"
            required
            value={formData.cidr}
            onChange={(e) => setFormData({ ...formData, cidr: e.target.value })}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            placeholder="e.g., 10.0.0.0/16"
          />
          <p className="mt-1 text-sm text-gray-500">Network address range in CIDR notation</p>
        </div>

        {/* Domain */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Domain <span className="text-red-500">*</span>
          </label>
          <input
            type="text"
            required
            value={formData.domain}
            onChange={(e) => setFormData({ ...formData, domain: e.target.value })}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent"
            placeholder="e.g., prod.wireguard.local"
          />
          <p className="mt-1 text-sm text-gray-500">DNS domain for this network</p>
        </div>

        {/* DNS Servers (only for create) */}
        {!isEditMode && (
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              DNS Servers (optional)
            </label>
            <div className="flex gap-2 mb-2">
              <input
                type="text"
                value={dnsInput}
                onChange={(e) => setDnsInput(e.target.value)}
                onKeyPress={(e) => e.key === 'Enter' && (e.preventDefault(), addDns())}
                className="flex-1 px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                placeholder="e.g., 8.8.8.8"
              />
              <button
                type="button"
                onClick={addDns}
                className="px-4 py-2 bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200"
              >
                Add
              </button>
            </div>
            {formData.dns.length > 0 && (
              <div className="space-y-1">
                {formData.dns.map((dns, index) => (
                  <div key={index} className="flex items-center justify-between bg-gray-50 px-3 py-2 rounded">
                    <span className="text-sm text-gray-700">{dns}</span>
                    <button
                      type="button"
                      onClick={() => removeDns(index)}
                      className="text-red-600 hover:text-red-800"
                    >
                      Remove
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

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
