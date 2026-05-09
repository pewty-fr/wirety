import { useEffect, useState } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faUserPlus, faEye, faEyeSlash } from '@fortawesome/free-solid-svg-icons';
import Modal from './Modal';
import api from '../api/client';
import type { Network } from '../types';

interface CreateUserModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreated?: () => void;
}

export default function CreateUserModal({ isOpen, onClose, onCreated }: CreateUserModalProps) {
  const [email, setEmail] = useState('');
  const [name, setName] = useState('');
  const [role, setRole] = useState<'administrator' | 'user'>('user');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [authorizedNetworks, setAuthorizedNetworks] = useState<string[]>([]);
  const [availableNetworks, setAvailableNetworks] = useState<Network[]>([]);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!isOpen) return;
    setEmail('');
    setName('');
    setRole('user');
    setPassword('');
    setShowPassword(false);
    setAuthorizedNetworks([]);
    setError('');
    void api.getNetworks(1, 200).then(r => setAvailableNetworks(r.data ?? [])).catch(() => setAvailableNetworks([]));
  }, [isOpen]);

  const toggleNetwork = (id: string) => {
    setAuthorizedNetworks(prev => prev.includes(id) ? prev.filter(x => x !== id) : [...prev, id]);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (password.length < 8) {
      setError('Password must be at least 8 characters');
      return;
    }
    if (!email.trim() || !name.trim()) {
      setError('Email and name are required');
      return;
    }

    setSubmitting(true);
    try {
      await api.createUser({
        email: email.trim(),
        name: name.trim(),
        role,
        password,
        authorized_networks: role === 'administrator' ? [] : authorizedNetworks,
      });
      onCreated?.();
      onClose();
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } }; message?: string };
      setError(e?.response?.data?.error || e?.message || 'Failed to create user');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="New User" size="lg">
      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="flex items-center gap-3">
          <div className="inline-flex items-center justify-center w-12 h-12 rounded-xl bg-gradient-to-br from-primary-500 to-accent-blue">
            <FontAwesomeIcon icon={faUserPlus} className="text-xl text-white" />
          </div>
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Create a local user that can sign in with email and password. Only available when AUTH_ENABLED=false.
          </p>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Email</label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-primary-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-primary-500"
            />
          </div>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Password (min. 8 characters)</label>
          <div className="relative">
            <input
              type={showPassword ? 'text' : 'password'}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={8}
              className="w-full px-3 py-2 pr-10 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-primary-500"
            />
            <button type="button" onClick={() => setShowPassword(!showPassword)} className="absolute inset-y-0 right-0 pr-3 flex items-center text-gray-400 hover:text-gray-600">
              <FontAwesomeIcon icon={showPassword ? faEyeSlash : faEye} className="text-sm" />
            </button>
          </div>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Role</label>
          <select
            value={role}
            onChange={(e) => setRole(e.target.value as 'administrator' | 'user')}
            className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-primary-500"
          >
            <option value="user">User</option>
            <option value="administrator">Administrator</option>
          </select>
        </div>

        {role !== 'administrator' && (
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Authorized networks ({authorizedNetworks.length})
            </label>
            <div className="max-h-48 overflow-y-auto border border-gray-200 dark:border-gray-600 rounded-lg">
              {availableNetworks.length === 0 ? (
                <p className="text-sm text-gray-500 p-3">No networks available</p>
              ) : (
                <div className="divide-y divide-gray-200 dark:divide-gray-600">
                  {availableNetworks.map((n) => (
                    <label key={n.id} className="flex items-center gap-3 px-3 py-2 hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={authorizedNetworks.includes(n.id)}
                        onChange={() => toggleNetwork(n.id)}
                        className="w-4 h-4 text-primary-600 border-gray-300 rounded focus:ring-primary-500"
                      />
                      <div className="flex-1">
                        <div className="text-sm font-medium text-gray-900 dark:text-gray-100">{n.name}</div>
                        <div className="text-xs text-gray-500 dark:text-gray-400">{n.cidr}</div>
                      </div>
                    </label>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}

        {error && <p className="text-sm text-red-500 dark:text-red-400">{error}</p>}

        <div className="flex justify-end gap-2 pt-4 border-t border-gray-200 dark:border-gray-700">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={submitting}
            className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {submitting ? 'Creating...' : 'Create user'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
