import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faUser, faPencil, faCheck, faTimes } from '@fortawesome/free-solid-svg-icons';
import Modal from './Modal';
import type { User, Network } from '../types';
import { useState, useEffect } from 'react';
import { api } from '../api/client';

interface UserDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  user: User | null;
  onUpdate?: () => void;
}

export default function UserDetailModal({ isOpen, onClose, user, onUpdate }: UserDetailModalProps) {
  const [isEditingNetworks, setIsEditingNetworks] = useState(false);
  const [selectedNetworks, setSelectedNetworks] = useState<string[]>([]);
  const [availableNetworks, setAvailableNetworks] = useState<Network[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [currentUser, setCurrentUser] = useState<User | null>(null);

  useEffect(() => {
    if (isOpen) {
      loadCurrentUser();
      loadNetworks(); // Load networks to display names
    }
  }, [isOpen]);

  useEffect(() => {
    if (user) {
      setSelectedNetworks(user.authorized_networks || []);
    }
  }, [user]);

  const loadCurrentUser = async () => {
    try {
      const userData = await api.getCurrentUser();
      setCurrentUser(userData);
    } catch (error) {
      console.error('Failed to load current user:', error);
    }
  };

  const loadNetworks = async () => {
    setIsLoading(true);
    try {
      const response = await api.getNetworks(1, 100);
      setAvailableNetworks(response.data);
    } catch (error) {
      console.error('Failed to load networks:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const handleSaveNetworks = async () => {
    if (!user) return;
    
    setIsSaving(true);
    try {
      await api.updateUser(user.id, {
        authorized_networks: selectedNetworks,
      });
      setIsEditingNetworks(false);
      if (onUpdate) {
        onUpdate();
      }
    } catch (error) {
      console.error('Failed to update networks:', error);
      alert('Failed to update authorized networks');
    } finally {
      setIsSaving(false);
    }
  };

  const handleCancelEdit = () => {
    setSelectedNetworks(user?.authorized_networks || []);
    setIsEditingNetworks(false);
  };

  const toggleNetwork = (networkId: string) => {
    setSelectedNetworks(prev => 
      prev.includes(networkId)
        ? prev.filter(id => id !== networkId)
        : [...prev, networkId]
    );
  };

  const isAdmin = currentUser?.role === 'administrator';

  if (!user) return null;

  const roleColors = {
    administrator: 'bg-purple-100 text-purple-800',
    user: 'bg-primary-100 text-primary-800 dark:bg-primary-900 dark:text-primary-200',
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="User Details"
      size="lg"
    >
      <div className="space-y-6">
        {/* Header Info */}
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-4">
            <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue">
              <FontAwesomeIcon icon={faUser} className="text-2xl text-white" />
            </div>
            <div>
              <h3 className="text-2xl font-bold text-gray-900 dark:text-white">{user.name}</h3>
              <p className="text-sm text-gray-600 dark:text-gray-300 mt-1">ID: {user.id}</p>
              <div className="flex gap-2 mt-2">
                <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${roleColors[user.role]}`}>
                  {user.role === 'administrator' ? 'Administrator' : 'User'}
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* User Info */}
        <div className="grid grid-cols-2 gap-6">
          <div>
            <label className="block text-sm font-medium text-gray-600 dark:text-gray-300 mb-1">Email</label>
            <p className="text-lg text-gray-900 dark:text-gray-100">{user.email}</p>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-600 dark:text-gray-300 mb-1">Role</label>
            <p className="text-lg text-gray-900 dark:text-gray-100 capitalize">{user.role}</p>
          </div>
        </div>

        {/* Authorized Networks - Only show for non-admin users */}
        {user.role !== 'administrator' && (
          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="block text-sm font-medium text-gray-600 dark:text-gray-300">
                Authorized Networks ({selectedNetworks.length})
              </label>
              {isAdmin && !isEditingNetworks && (
                <button
                  onClick={() => setIsEditingNetworks(true)}
                  className="text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300 text-sm flex items-center gap-1"
                >
                  <FontAwesomeIcon icon={faPencil} className="text-xs" />
                  Edit
                </button>
              )}
            </div>

            {isEditingNetworks ? (
              <div className="space-y-3">
                {isLoading ? (
                  <p className="text-sm text-gray-500">Loading networks...</p>
                ) : (
                  <>
                    <div className="max-h-60 overflow-y-auto border border-gray-200 dark:border-gray-600 rounded-lg">
                      {availableNetworks.length > 0 ? (
                        <div className="divide-y divide-gray-200 dark:divide-gray-600">
                          {availableNetworks.map((network) => (
                            <label
                              key={network.id}
                              className="flex items-center gap-3 px-3 py-2 hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer"
                            >
                              <input
                                type="checkbox"
                                checked={selectedNetworks.includes(network.id)}
                                onChange={() => toggleNetwork(network.id)}
                                className="w-4 h-4 text-primary-600 border-gray-300 rounded focus:ring-primary-500"
                              />
                              <div className="flex-1">
                                <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
                                  {network.name}
                                </div>
                                <div className="text-xs text-gray-500 dark:text-gray-400">
                                  {network.cidr}
                                </div>
                              </div>
                            </label>
                          ))}
                        </div>
                      ) : (
                        <p className="text-sm text-gray-500 p-3">No networks available</p>
                      )}
                    </div>
                    <div className="flex gap-2">
                      <button
                        onClick={handleSaveNetworks}
                        disabled={isSaving}
                        className="flex items-center gap-2 px-3 py-1.5 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed text-sm"
                      >
                        <FontAwesomeIcon icon={faCheck} />
                        {isSaving ? 'Saving...' : 'Save'}
                      </button>
                      <button
                        onClick={handleCancelEdit}
                        disabled={isSaving}
                        className="flex items-center gap-2 px-3 py-1.5 bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-200 rounded-lg hover:bg-gray-300 dark:hover:bg-gray-500 disabled:opacity-50 disabled:cursor-not-allowed text-sm"
                      >
                        <FontAwesomeIcon icon={faTimes} />
                        Cancel
                      </button>
                    </div>
                  </>
                )}
              </div>
            ) : (
              <>
                {selectedNetworks.length > 0 ? (
                  <div className="space-y-1">
                    {selectedNetworks.map((networkId, index) => {
                      const network = availableNetworks.find(n => n.id === networkId);
                      return (
                        <div key={index} className="bg-gray-50 dark:bg-gray-700 px-3 py-2 rounded text-sm text-gray-900 dark:text-gray-100">
                          {network ? `${network.name} (${network.cidr})` : networkId}
                        </div>
                      );
                    })}
                  </div>
                ) : (
                  <p className="text-sm text-gray-500 italic">No networks authorized</p>
                )}
              </>
            )}
          </div>
        )}

        {/* Activity Info */}
        <div className="bg-gradient-to-br from-gray-50 to-primary-50 dark:from-gray-800 dark:to-gray-700 rounded-lg p-4">
          <h4 className="text-sm font-medium text-gray-700 dark:text-gray-100 mb-3">Activity</h4>
          <div className="space-y-2">
            <div className="flex justify-between">
              <span className="text-sm text-gray-600 dark:text-gray-300">Last Login</span>
              <span className="text-sm text-gray-900 dark:text-gray-100">
                {user.last_login_at ? new Date(user.last_login_at).toLocaleString() : 'Never'}
              </span>
            </div>
          </div>
        </div>

        {/* Timestamps */}
        <div className="bg-gradient-to-br from-gray-50 to-primary-50 dark:from-gray-800 dark:to-gray-700 rounded-lg p-4">
          <div className="grid grid-cols-2 gap-6">
            <div>
              <label className="block text-sm font-medium text-gray-600 dark:text-gray-300 mb-1">Created</label>
              <p className="text-sm text-gray-900 dark:text-gray-100">
                {new Date(user.created_at).toLocaleString()}
              </p>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-600 dark:text-gray-300 mb-1">Last Updated</label>
              <p className="text-sm text-gray-900 dark:text-gray-100">
                {new Date(user.updated_at).toLocaleString()}
              </p>
            </div>
          </div>
        </div>

        {/* Actions */}
        <div className="flex justify-end gap-3 pt-4 border-t border-gray-200">
          <button
            onClick={onClose}
            className="px-4 py-2 text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 cursor-pointer transition-colors"
          >
            Close
          </button>
        </div>
      </div>
    </Modal>
  );
}
