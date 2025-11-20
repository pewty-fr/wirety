import { useState, useEffect } from 'react';
import Modal from './Modal';
import api from '../api/client';
import type { Network } from '../types';
import { isValidCIDR, getCIDRError } from '../utils/validation';

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
  const [cidrError, setCidrError] = useState<string | null>(null);
  const [maxPeers, setMaxPeers] = useState<number>(100);
  const [suggestions, setSuggestions] = useState<string[]>([]);
  const [loadingSuggestions, setLoadingSuggestions] = useState(false);
  const [showSuggestions, setShowSuggestions] = useState(false);

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
    setCidrError(null);
    setSuggestions([]);
    setShowSuggestions(false);
  }, [network, isOpen]);

  const fetchSuggestions = async () => {
    setLoadingSuggestions(true);
    try {
      const response = await api.getSuggestedCIDRs(maxPeers, 10);
      setSuggestions(response.cidrs);
      setShowSuggestions(true);
    } catch (err) {
      console.error('Failed to fetch CIDR suggestions:', err);
    } finally {
      setLoadingSuggestions(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    // Validate CIDR
    if (!isValidCIDR(formData.cidr)) {
      const error = getCIDRError(formData.cidr);
      setCidrError(error || 'Invalid CIDR format');
      setLoading(false);
      return;
    }
    setCidrError(null);

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
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Name <span className="text-red-500">*</span>
          </label>
          <input
            type="text"
            required
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500"
            placeholder="e.g., Production Network"
          />
        </div>

        {/* CIDR */}
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            CIDR <span className="text-red-500">*</span>
          </label>
          
          {/* Max Peers Input (only for create) */}
          {!isEditMode && (
            <div className="mb-2">
              <div className="flex gap-2">
                <div className="flex-1">
                  <label className="block text-xs text-gray-500 dark:text-gray-400 mb-1">Expected max peers</label>
                  <input
                    type="number"
                    value={maxPeers}
                    onChange={(e) => setMaxPeers(parseInt(e.target.value) || 0)}
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                    min={1}
                    placeholder="100"
                  />
                </div>
                <div className="flex items-end">
                  <button
                    type="button"
                    onClick={fetchSuggestions}
                    disabled={loadingSuggestions || !maxPeers}
                    className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                  >
                    {loadingSuggestions ? 'Loading...' : 'Suggest'}
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* CIDR Suggestions */}
          {showSuggestions && suggestions.length > 0 && (
            <div className="mb-2 p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
              <p className="text-xs font-medium text-blue-800 dark:text-blue-300 mb-2">Suggested CIDRs (click to use):</p>
              <div className="grid grid-cols-2 gap-2">
                {suggestions.map((cidr) => (
                  <button
                    key={cidr}
                    type="button"
                    onClick={() => {
                      setFormData({ ...formData, cidr });
                      setCidrError(null);
                      setShowSuggestions(false);
                    }}
                    className="text-left px-2 py-1 text-sm bg-white dark:bg-gray-700 border border-blue-300 dark:border-blue-700 rounded hover:bg-blue-100 dark:hover:bg-blue-800 text-gray-900 dark:text-white transition-colors"
                  >
                    {cidr}
                  </button>
                ))}
              </div>
            </div>
          )}

          <input
            type="text"
            required
            value={formData.cidr}
            onChange={(e) => {
              const value = e.target.value;
              setFormData({ ...formData, cidr: value });
              if (value) {
                setCidrError(getCIDRError(value));
              } else {
                setCidrError(null);
              }
            }}
            className={`w-full px-3 py-2 border rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 focus:ring-2 focus:ring-primary-500 focus:border-transparent ${
              cidrError ? 'border-red-300 dark:border-red-600' : 'border-gray-300 dark:border-gray-600'
            }`}
            placeholder="e.g., 10.0.0.0/16"
          />
          {cidrError && (
            <p className="mt-1 text-sm text-red-600">{cidrError}</p>
          )}
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
            className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 focus:ring-2 focus:ring-primary-500 focus:border-transparent"
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
                className="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                placeholder="e.g., 8.8.8.8"
              />
              <button
                type="button"
                onClick={addDns}
                className="px-4 py-2 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600"
              >
                Add
              </button>
            </div>
            {formData.dns.length > 0 && (
              <div className="space-y-1">
                {formData.dns.map((dns, index) => (
                  <div key={index} className="flex items-center justify-between bg-gray-50 dark:bg-gray-700 px-3 py-2 rounded">
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
