import { useState, useEffect } from 'react';
import Modal from './Modal';
import api from '../api/client';
import type { Network } from '../types';
import { isValidCIDR, getCIDRError, suggestIPv6ULACIDRs } from '../utils/validation';

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
    cidr_v6: '',
    dns: [] as string[],
    domain_suffix: 'internal',
    default_group_ids: [] as string[],
  });
  const [dnsInput, setDnsInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [cidrError, setCidrError] = useState<string | null>(null);
  const [cidrV6Error, setCidrV6Error] = useState<string | null>(null);
  const [maxPeers, setMaxPeers] = useState<number>(100);
  const [suggestions, setSuggestions] = useState<string[]>([]);
  const [loadingSuggestions, setLoadingSuggestions] = useState(false);
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [ipv6Suggestions, setIpv6Suggestions] = useState<string[]>([]);
  const [showIpv6Suggestions, setShowIpv6Suggestions] = useState(false);

  const isEditMode = !!network;

  useEffect(() => {
    if (network) {
      setFormData({
        name: network.name,
        cidr: network.cidr,
        cidr_v6: network.cidr_v6 || '',
        dns: network.dns,
        domain_suffix: network.domain_suffix || 'internal',
        default_group_ids: network.default_group_ids || [],
      });
    } else {
      setFormData({
        name: '',
        cidr: '',
        cidr_v6: '',
        dns: [],
        domain_suffix: 'internal',
        default_group_ids: [],
      });
    }
    setError(null);
    setCidrError(null);
    setCidrV6Error(null);
    setSuggestions([]);
    setShowSuggestions(false);
    setIpv6Suggestions([]);
    setShowIpv6Suggestions(false);
  }, [network, isOpen]);

  const generateIPv6Suggestions = () => {
    setIpv6Suggestions(suggestIPv6ULACIDRs(5));
    setShowIpv6Suggestions(true);
  };

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

    // At least one of cidr or cidr_v6 is required
    if (!formData.cidr && !formData.cidr_v6) {
      setCidrError('At least one of IPv4 CIDR or IPv6 CIDR is required');
      setLoading(false);
      return;
    }

    // Validate IPv4 CIDR if provided
    if (formData.cidr) {
      if (!isValidCIDR(formData.cidr)) {
        const err = getCIDRError(formData.cidr);
        setCidrError(err || 'Invalid CIDR format');
        setLoading(false);
        return;
      }
    }
    setCidrError(null);

    // Validate IPv6 CIDR if provided
    if (formData.cidr_v6) {
      if (!isValidCIDR(formData.cidr_v6)) {
        const err = getCIDRError(formData.cidr_v6);
        setCidrV6Error(err || 'Invalid IPv6 CIDR format');
        setLoading(false);
        return;
      }
    }
    setCidrV6Error(null);

    try {
      if (isEditMode && network) {
        await api.updateNetwork(network.id, {
          name: formData.name,
          cidr: formData.cidr || undefined,
          cidr_v6: formData.cidr_v6 || undefined,
          dns: formData.dns,
          domain_suffix: formData.domain_suffix,
          default_group_ids: formData.default_group_ids,
        });
      } else {
        await api.createNetwork({
          name: formData.name,
          cidr: formData.cidr || undefined,
          cidr_v6: formData.cidr_v6 || undefined,
          dns: formData.dns.length > 0 ? formData.dns : undefined,
          domain_suffix: formData.domain_suffix,
          default_group_ids: formData.default_group_ids.length > 0 ? formData.default_group_ids : undefined,
        });
      }
      onSuccess();
      onClose();
    } catch (err) {
      const error = err as { response?: { data?: { error?: string } } };
      setError(error.response?.data?.error || 'Failed to save network');
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
            placeholder="e.g., prod or prod.eu"
          />
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">Lowercase letters, numbers, hyphens and dots only (e.g. <code className="font-mono">prod</code>, <code className="font-mono">prod.eu</code>)</p>
        </div>

        {/* CIDR */}
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            IPv4 CIDR <span className="text-gray-400 font-normal">(optional if IPv6 provided)</span>
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
          <p className="mt-1 text-sm text-gray-500">IPv4 network address range in CIDR notation</p>
        </div>

        {/* IPv6 CIDR */}
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            IPv6 CIDR <span className="text-gray-400 font-normal">(optional)</span>
          </label>

          {/* IPv6 ULA Suggestion button (only for create) */}
          {!isEditMode && (
            <div className="mb-2 flex justify-end">
              <button
                type="button"
                onClick={generateIPv6Suggestions}
                className="px-3 py-1.5 text-xs bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
                title="Generate random Unique Local Address (RFC 4193) prefixes — the IPv6 equivalent of RFC 1918 private addresses"
              >
                Suggest ULA prefixes
              </button>
            </div>
          )}

          {/* IPv6 CIDR Suggestions */}
          {showIpv6Suggestions && ipv6Suggestions.length > 0 && (
            <div className="mb-2 p-3 bg-purple-50 dark:bg-purple-900/20 border border-purple-200 dark:border-purple-800 rounded-lg">
              <p className="text-xs font-medium text-purple-800 dark:text-purple-300 mb-2">
                Suggested IPv6 ULA prefixes (RFC 4193, click to use):
              </p>
              <div className="grid grid-cols-1 gap-2">
                {ipv6Suggestions.map((cidr) => (
                  <button
                    key={cidr}
                    type="button"
                    onClick={() => {
                      setFormData((prev) => ({ ...prev, cidr_v6: cidr }));
                      setCidrV6Error(null);
                      setShowIpv6Suggestions(false);
                    }}
                    className="text-left px-2 py-1 text-sm font-mono bg-white dark:bg-gray-700 border border-purple-300 dark:border-purple-700 rounded hover:bg-purple-100 dark:hover:bg-purple-800 text-gray-900 dark:text-white transition-colors"
                  >
                    {cidr}
                  </button>
                ))}
              </div>
              <p className="mt-2 text-xs text-purple-700 dark:text-purple-300">
                Each prefix uses a freshly randomised 40-bit Global ID per RFC 4193 §3.2.2 — non-routable on the public internet, safe for private use.
              </p>
            </div>
          )}

          <input
            type="text"
            value={formData.cidr_v6}
            onChange={(e) => {
              const value = e.target.value;
              setFormData({ ...formData, cidr_v6: value });
              if (value) {
                setCidrV6Error(getCIDRError(value));
              } else {
                setCidrV6Error(null);
              }
            }}
            className={`w-full px-3 py-2 border rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 focus:ring-2 focus:ring-primary-500 focus:border-transparent ${
              cidrV6Error ? 'border-red-300 dark:border-red-600' : 'border-gray-300 dark:border-gray-600'
            }`}
            placeholder="e.g., fd00::/64"
          />
          {cidrV6Error && (
            <p className="mt-1 text-sm text-red-600">{cidrV6Error}</p>
          )}
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            IPv6 network address range for dual-stack support. Use a Unique Local Address (<code className="font-mono">fd00::/8</code>) — the IPv6 equivalent of RFC 1918 — never a globally-routable prefix.
          </p>
        </div>

        {/* DNS Servers */}
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
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
                  <span className="text-left px-2 py-1 text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-white transition-colors">{dns}</span>
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

        {/* Domain Suffix */}
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Domain Suffix
          </label>
          <input
            type="text"
            value={formData.domain_suffix}
            onChange={(e) => setFormData({ ...formData, domain_suffix: e.target.value })}
            className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500"
            placeholder="internal"
          />
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">DNS suffix appended to route names (e.g. <code className="font-mono">internal</code>, <code className="font-mono">corp.example.com</code>)</p>
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
