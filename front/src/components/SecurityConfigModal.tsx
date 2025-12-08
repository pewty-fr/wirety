import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faCheck, faTimes, faShieldHalved } from '@fortawesome/free-solid-svg-icons';
import Modal from './Modal';
import SearchableSelect from './SearchableSelect';
import type { SecurityConfigUpdateRequest, Network } from '../types';
import { useState, useEffect } from 'react';
import { api } from '../api/client';

interface SecurityConfigModalProps {
  isOpen: boolean;
  onClose: () => void;
  networkId?: string;
  networks?: Network[];
  onUpdate?: () => void;
}

export default function SecurityConfigModal({
  isOpen,
  onClose,
  networkId,
  networks = [],
  onUpdate,
}: SecurityConfigModalProps) {
  const [selectedNetworkId, setSelectedNetworkId] = useState<string>(networkId || '');
  const [enabled, setEnabled] = useState<boolean>(true);
  const [sessionConflictThreshold, setSessionConflictThreshold] = useState<number>(5);
  const [endpointChangeThreshold, setEndpointChangeThreshold] = useState<number>(30);
  const [maxEndpointChangesPerDay, setMaxEndpointChangesPerDay] = useState<number>(10);
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);

  useEffect(() => {
    if (isOpen) {
      setSelectedNetworkId(networkId || '');
      if (networkId) {
        loadData(networkId);
      } else {
        // Reset to defaults when no network is selected
        setEnabled(true);
        setSessionConflictThreshold(5);
        setEndpointChangeThreshold(30);
        setMaxEndpointChangesPerDay(10);
      }
    }
  }, [isOpen, networkId]);

  useEffect(() => {
    if (selectedNetworkId) {
      loadData(selectedNetworkId);
    }
  }, [selectedNetworkId]);

  const loadData = async (targetNetworkId: string) => {
    if (!targetNetworkId) return;
    
    setIsLoading(true);
    try {
      const config = await api.getSecurityConfig(targetNetworkId);
      setEnabled(config.enabled);
      setSessionConflictThreshold(config.session_conflict_threshold_minutes);
      setEndpointChangeThreshold(config.endpoint_change_threshold_minutes);
      setMaxEndpointChangesPerDay(config.max_endpoint_changes_per_day);
    } catch (error) {
      console.error('Failed to load security config:', error);
      // Use default values if config doesn't exist
      setEnabled(true);
      setSessionConflictThreshold(5);
      setEndpointChangeThreshold(30);
      setMaxEndpointChangesPerDay(10);
    } finally {
      setIsLoading(false);
    }
  };

  const handleSave = async () => {
    if (!selectedNetworkId) {
      alert('Please select a network');
      return;
    }
    
    setIsSaving(true);
    try {
      const updateData: SecurityConfigUpdateRequest = {
        enabled,
        session_conflict_threshold_minutes: sessionConflictThreshold,
        endpoint_change_threshold_minutes: endpointChangeThreshold,
        max_endpoint_changes_per_day: maxEndpointChangesPerDay,
      };
      
      await api.updateSecurityConfig(selectedNetworkId, updateData);
      
      if (onUpdate) {
        onUpdate();
      }
      onClose();
    } catch (error) {
      console.error('Failed to update security config:', error);
      alert('Failed to update security configuration');
    } finally {
      setIsSaving(false);
    }
  };

  const handleCancel = () => {
    onClose();
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Security Configuration" size="lg">
      <div className="space-y-6">
        {isLoading ? (
          <div className="flex flex-col items-center justify-center py-8">
            <div className="inline-block animate-spin rounded-full h-8 w-8 border-4 border-solid border-current border-r-transparent align-[-0.125em] text-primary-600 dark:text-primary-400 motion-reduce:animate-[spin_1.5s_linear_infinite] mb-4"></div>
            <p className="text-gray-600 dark:text-gray-300 text-sm">Loading security configuration...</p>
          </div>
        ) : (
          <>
            {/* Header with icon */}
            <div className="flex items-center gap-3 pb-4 border-b border-gray-200 dark:border-gray-700">
              <div className="inline-flex items-center justify-center w-10 h-10 rounded-lg bg-gradient-to-br from-primary-500 to-accent-blue">
                <FontAwesomeIcon icon={faShieldHalved} className="text-white" />
              </div>
              <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
                  Security Detection Settings
                </h3>
                <p className="text-sm text-gray-500 dark:text-gray-400">
                  Configure security incident detection and thresholds
                </p>
              </div>
            </div>

            {/* Network Selection */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Network *
              </label>
              <SearchableSelect
                options={networks.map(network => ({
                  value: network.id,
                  label: `${network.name} (${network.cidr})`
                }))}
                value={selectedNetworkId}
                onChange={setSelectedNetworkId}
                placeholder="Select a network..."
              />
              {!selectedNetworkId && (
                <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  Please select a network to configure security settings
                </p>
              )}
            </div>

            {/* Message when no network selected */}
            {!selectedNetworkId && (
              <div className="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
                <div className="flex items-start gap-3">
                  <div className="text-blue-600 dark:text-blue-400 mt-0.5">
                    ℹ️
                  </div>
                  <div>
                    <h4 className="text-sm font-medium text-blue-800 dark:text-blue-200">
                      Select a Network
                    </h4>
                    <p className="text-sm text-blue-700 dark:text-blue-300 mt-1">
                      Choose a network from the dropdown above to view and configure its security detection settings.
                    </p>
                  </div>
                </div>
              </div>
            )}

            {/* Security Settings - Only show when network is selected */}
            {selectedNetworkId && (
              <>
                {/* Security Detection Toggle */}
                <div>
                  <label className="flex items-center gap-3 px-4 py-3 border border-gray-200 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={enabled}
                      onChange={(e) => setEnabled(e.target.checked)}
                      className="w-4 h-4 text-primary-600 border-gray-300 rounded focus:ring-primary-500"
                    />
                    <div className="flex-1">
                      <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
                        Enable Security Incident Detection
                      </div>
                      <div className="text-xs text-gray-500 dark:text-gray-400">
                        Automatically detect and respond to suspicious network activity
                      </div>
                    </div>
                  </label>
                </div>

            {/* Security Thresholds - Only show when enabled */}
            {enabled && (
              <div className="space-y-4">
                <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300">
                  Detection Thresholds
                </h4>

                {/* Session Conflict Threshold */}
                <div>
                  <label className="block text-sm font-medium text-gray-600 dark:text-gray-300 mb-2">
                    Session Conflict Threshold
                  </label>
                  <div className="flex items-center gap-3">
                    <input
                      type="number"
                      min="1"
                      max="60"
                      value={sessionConflictThreshold}
                      onChange={(e) => setSessionConflictThreshold(parseInt(e.target.value) || 5)}
                      className="w-20 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                    />
                    <span className="text-sm text-gray-600 dark:text-gray-300">minutes</span>
                  </div>
                  <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                    Time window to consider sessions as conflicting (1-60 minutes)
                  </p>
                </div>

                {/* Endpoint Change Threshold */}
                <div>
                  <label className="block text-sm font-medium text-gray-600 dark:text-gray-300 mb-2">
                    Endpoint Change Threshold
                  </label>
                  <div className="flex items-center gap-3">
                    <input
                      type="number"
                      min="1"
                      max="1440"
                      value={endpointChangeThreshold}
                      onChange={(e) => setEndpointChangeThreshold(parseInt(e.target.value) || 30)}
                      className="w-20 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                    />
                    <span className="text-sm text-gray-600 dark:text-gray-300">minutes</span>
                  </div>
                  <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                    Minimum time between endpoint changes to not be suspicious (1-1440 minutes)
                  </p>
                </div>

                {/* Max Endpoint Changes Per Day */}
                <div>
                  <label className="block text-sm font-medium text-gray-600 dark:text-gray-300 mb-2">
                    Maximum Endpoint Changes Per Day
                  </label>
                  <div className="flex items-center gap-3">
                    <input
                      type="number"
                      min="1"
                      max="1000"
                      value={maxEndpointChangesPerDay}
                      onChange={(e) => setMaxEndpointChangesPerDay(parseInt(e.target.value) || 10)}
                      className="w-20 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                    />
                    <span className="text-sm text-gray-600 dark:text-gray-300">changes</span>
                  </div>
                  <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                    Maximum number of endpoint changes per day before flagging as suspicious (1-1000)
                  </p>
                </div>
              </div>
            )}

                {/* Warning when disabled */}
                {!enabled && (
                  <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4">
                    <div className="flex items-start gap-3">
                      <div className="text-yellow-600 dark:text-yellow-400 mt-0.5">
                        ⚠️
                      </div>
                      <div>
                        <h4 className="text-sm font-medium text-yellow-800 dark:text-yellow-200">
                          Security Detection Disabled
                        </h4>
                        <p className="text-sm text-yellow-700 dark:text-yellow-300 mt-1">
                          When disabled, the system will not detect or respond to suspicious network activity such as session conflicts, shared configurations, or unusual endpoint changes.
                        </p>
                      </div>
                    </div>
                  </div>
                )}
              </>
            )}

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
                disabled={isSaving || !selectedNetworkId}
                className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <FontAwesomeIcon icon={faCheck} />
                {isSaving ? 'Saving...' : 'Save Configuration'}
              </button>
            </div>
          </>
        )}
      </div>
    </Modal>
  );
}
