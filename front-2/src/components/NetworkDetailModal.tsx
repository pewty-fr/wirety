import { useState } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faNetworkWired } from '@fortawesome/free-solid-svg-icons';
import Modal from './Modal';
import NetworkModal from './NetworkModal';
import api from '../api/client';
import type { Network } from '../types';
import { computeCapacityFromCIDR } from '../utils/networkCapacity';

interface NetworkDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  network: Network | null;
  onUpdate: () => void;
}

export default function NetworkDetailModal({ isOpen, onClose, network, onUpdate }: NetworkDetailModalProps) {
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  if (!network) return null;

  const capacity = computeCapacityFromCIDR(network.cidr);
  const used = network.peer_count ?? 0;
  const available = capacity !== null ? capacity - used : 0;
  const utilizationPercent = capacity ? Math.round((used / capacity) * 100) : 0;

  const handleDelete = async () => {
    if (!confirm(`Are you sure you want to delete network "${network.name}"? This will delete all associated peers. This action cannot be undone.`)) {
      return;
    }
    
    setDeleting(true);
    try {
      await api.deleteNetwork(network.id);
      onUpdate();
      onClose();
    } catch (error: any) {
      alert(error.response?.data?.error || 'Failed to delete network');
    } finally {
      setDeleting(false);
    }
  };

  const handleEdit = () => {
    setIsEditModalOpen(true);
  };

  const handleEditSuccess = () => {
    onUpdate();
  };

  return (
    <>
      <Modal
        isOpen={isOpen}
        onClose={onClose}
        title="Network Details"
        size="lg"
      >
        <div className="space-y-6">
          {/* Header Info */}
          <div className="flex items-start justify-between">
            <div className="flex items-start gap-4">
              <div className="text-5xl text-primary-600 dark:text-primary-400">
                <FontAwesomeIcon icon={faNetworkWired} />
              </div>
              <div>
                <h3 className="text-2xl font-bold text-gray-900 dark:text-white">{network.name}</h3>
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">ID: {network.id}</p>
              </div>
            </div>
          </div>

          {/* Main Info Grid */}
          <div className="grid grid-cols-2 gap-6">
            <div>
              <label className="block text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">CIDR</label>
              <p className="text-lg font-mono text-gray-900 dark:text-white">{network.cidr}</p>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">Domain</label>
              <p className="text-lg text-gray-900 dark:text-white">{network.domain}</p>
            </div>
          </div>

          {/* Capacity Stats */}
          <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
            <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Capacity</h4>
            <div className="grid grid-cols-4 gap-4">
              <div>
                <p className="text-2xl font-bold text-gray-900 dark:text-white">{capacity ?? 0}</p>
                <p className="text-xs text-gray-500 dark:text-gray-400">Total Slots</p>
              </div>
              <div>
                <p className="text-2xl font-bold text-primary-600 dark:text-primary-400">{used}</p>
                <p className="text-xs text-gray-500 dark:text-gray-400">Used</p>
              </div>
              <div>
                <p className="text-2xl font-bold text-green-600 dark:text-green-400">{available}</p>
                <p className="text-xs text-gray-500 dark:text-gray-400">Available</p>
              </div>
              <div>
                <p className="text-2xl font-bold text-purple-600 dark:text-purple-400">{utilizationPercent}%</p>
                <p className="text-xs text-gray-500">Utilization</p>
              </div>
            </div>
          </div>

          {/* Timestamps */}
          <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
            <div className="grid grid-cols-2 gap-6">
              <div>
                <label className="block text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">Created</label>
                <p className="text-sm text-gray-900 dark:text-white">
                  {new Date(network.created_at).toLocaleString()}
                </p>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">Last Updated</label>
                <p className="text-sm text-gray-900 dark:text-white">
                  {new Date(network.updated_at).toLocaleString()}
                </p>
              </div>
            </div>
          </div>

          {/* Actions */}
          <div className="flex justify-between gap-3 pt-4 border-t border-gray-200">
            <button
              onClick={handleDelete}
              disabled={deleting}
              title="Delete Network"
              className="px-4 py-2 text-red-600 bg-red-50 rounded-lg hover:bg-red-100 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer transition-colors flex items-center gap-2"
            >
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
              {deleting ? 'Deleting...' : 'Delete'}
            </button>
            <div className="flex gap-3">
              <button
                onClick={onClose}
                className="px-4 py-2 text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 cursor-pointer transition-colors"
              >
                Close
              </button>
              <button
                onClick={handleEdit}
                title="Edit Network"
                className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 cursor-pointer transition-colors flex items-center gap-2"
              >
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
                Edit
              </button>
            </div>
          </div>
        </div>
      </Modal>

      {/* Edit Modal */}
      <NetworkModal
        isOpen={isEditModalOpen}
        onClose={() => setIsEditModalOpen(false)}
        onSuccess={handleEditSuccess}
        network={network}
      />
    </>
  );
}
