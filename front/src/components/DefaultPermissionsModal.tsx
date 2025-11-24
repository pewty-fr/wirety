import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faCheck, faTimes } from '@fortawesome/free-solid-svg-icons';
import Modal from './Modal';
import type { Network } from '../types';
import { useState, useEffect } from 'react';
import { api } from '../api/client';

interface DefaultPermissions {
  default_role: 'administrator' | 'user';
  default_authorized_networks: string[];
}

interface DefaultPermissionsModalProps {
  isOpen: boolean;
  onClose: () => void;
  onUpdate?: () => void;
}

export default function DefaultPermissionsModal({
  isOpen,
  onClose,
  onUpdate,
}: DefaultPermissionsModalProps) {
  const [selectedRole, setSelectedRole] = useState<'administrator' | 'user'>('user');
  const [selectedNetworks, setSelectedNetworks] = useState<string[]>([]);
  const [availableNetworks, setAvailableNetworks] = useState<Network[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);

  useEffect(() => {
    if (isOpen) {
      loadData();
    }
  }, [isOpen]);

  const loadData = async () => {
    setIsLoading(true);
    try {
      const [permissions, networksResponse] = await Promise.all([
        api.getDefaultPermissions(),
        api.getNetworks(1, 100),
      ]);
      setSelectedRole(permissions.default_role);
      setSelectedNetworks(permissions.default_authorized_networks || []);
      setAvailableNetworks(networksResponse.data);
    } catch (error) {
      console.error('Failed to load data:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const handleSave = async () => {
    setIsSaving(true);
    try {
      await api.updateDefaultPermissions({
        default_role: selectedRole,
        default_authorized_networks: selectedNetworks,
      });
      if (onUpdate) {
        onUpdate();
      }
      onClose();
    } catch (error) {
      console.error('Failed to update default permissions:', error);
      alert('Failed to update default permissions');
    } finally {
      setIsSaving(false);
    }
  };

  const handleCancel = () => {
    onClose();
  };

  const toggleNetwork = (networkId: string) => {
    setSelectedNetworks((prev) =>
      prev.includes(networkId)
        ? prev.filter((id) => id !== networkId)
        : [...prev, networkId]
    );
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Default Permissions for New Users" size="lg">
      <div className="space-y-6">
        {isLoading ? (
          <div className="flex flex-col items-center justify-center py-8">
            <div className="inline-block animate-spin rounded-full h-8 w-8 border-4 border-solid border-current border-r-transparent align-[-0.125em] text-primary-600 dark:text-primary-400 motion-reduce:animate-[spin_1.5s_linear_infinite] mb-4"></div>
            <p className="text-gray-600 dark:text-gray-300 text-sm">Loading...</p>
          </div>
        ) : (
          <>
            {/* Default Role */}
            <div>
              <label className="block text-sm font-medium text-gray-600 dark:text-gray-300 mb-2">
                Default Role
              </label>
              <div className="space-y-2">
                <label className="flex items-center gap-3 px-4 py-3 border border-gray-200 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer">
                  <input
                    type="radio"
                    name="role"
                    value="user"
                    checked={selectedRole === 'user'}
                    onChange={(e) => setSelectedRole(e.target.value as 'user')}
                    className="w-4 h-4 text-primary-600 border-gray-300 focus:ring-primary-500"
                  />
                  <div className="flex-1">
                    <div className="text-sm font-medium text-gray-900 dark:text-gray-100">User</div>
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                      Standard user with limited access
                    </div>
                  </div>
                </label>
                <label className="flex items-center gap-3 px-4 py-3 border border-gray-200 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer">
                  <input
                    type="radio"
                    name="role"
                    value="administrator"
                    checked={selectedRole === 'administrator'}
                    onChange={(e) => setSelectedRole(e.target.value as 'administrator')}
                    className="w-4 h-4 text-primary-600 border-gray-300 focus:ring-primary-500"
                  />
                  <div className="flex-1">
                    <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
                      Administrator
                    </div>
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                      Full access to all networks and settings
                    </div>
                  </div>
                </label>
              </div>
            </div>

            {/* Default Authorized Networks */}
            <div>
              <label className="block text-sm font-medium text-gray-600 dark:text-gray-300 mb-2">
                Default Authorized Networks ({selectedNetworks.length})
              </label>
              <p className="text-xs text-gray-500 dark:text-gray-400 mb-3">
                New users will automatically have access to these networks
              </p>
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
            </div>

            {/* Actions */}
            <div className="flex justify-end gap-3 pt-4 border-t border-gray-200 dark:border-gray-700">
              <button
                onClick={handleCancel}
                disabled={isSaving}
                className="flex items-center gap-2 px-4 py-2 text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <FontAwesomeIcon icon={faTimes} />
                Cancel
              </button>
              <button
                onClick={handleSave}
                disabled={isSaving}
                className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <FontAwesomeIcon icon={faCheck} />
                {isSaving ? 'Saving...' : 'Save'}
              </button>
            </div>
          </>
        )}
      </div>
    </Modal>
  );
}
