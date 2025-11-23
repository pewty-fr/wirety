import { useState } from 'react';
import Modal from './Modal';
import api from '../api/client';
import type { SecurityIncident } from '../types';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faShieldHalved } from '@fortawesome/free-solid-svg-icons';

interface IncidentDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  incident: SecurityIncident | null;
  onUpdate: () => void;
}

export default function IncidentDetailModal({ isOpen, onClose, incident, onUpdate }: IncidentDetailModalProps) {
  const [resolving, setResolving] = useState(false);

  if (!incident) return null;

  const incidentTypeLabels = {
    shared_config: 'Shared Configuration',
    session_conflict: 'Session Conflict',
    suspicious_activity: 'Suspicious Activity',
  };

  const incidentTypeColors = {
    shared_config: 'bg-yellow-100 text-yellow-800',
    session_conflict: 'bg-red-100 text-red-800',
    suspicious_activity: 'bg-orange-100 text-orange-800',
  };

  const handleResolve = async () => {
    if (!confirm('Mark this incident as resolved?')) {
      return;
    }
    
    setResolving(true);
    try {
      await api.resolveIncident(incident.id);
      onUpdate();
      onClose();
    } catch (error: any) {
      alert(error.response?.data?.error || 'Failed to resolve incident');
    } finally {
      setResolving(false);
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Security Incident Details"
      size="lg"
    >
      <div className="space-y-6">
        {/* Header Info */}
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-4">
            <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue">
              <FontAwesomeIcon icon={faShieldHalved} className="text-xl text-white" />
            </div>
            <div>
              <h3 className="text-2xl font-bold text-gray-900 dark:text-white">
                {incidentTypeLabels[incident.incident_type]}
              </h3>
              <p className="text-sm text-gray-500 mt-1">ID: {incident.id}</p>
              <div className="flex gap-2 mt-2">
                {incident.resolved ? (
                  <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                    Resolved
                  </span>
                ) : (
                  <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800">
                    Active
                  </span>
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Affected Resources */}
        <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
          <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Affected Resources</h4>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Network</label>
              <p className="text-sm text-gray-900 dark:text-white">{incident.network_name || incident.network_id}</p>
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Peer</label>
              <p className="text-sm text-gray-900 dark:text-white">{incident.peer_name || incident.peer_id}</p>
            </div>
          </div>
        </div>

        {/* Technical Details */}
        <div>
          <label className="block text-sm font-medium text-gray-500 mb-1">Public Key</label>
          <p className="text-sm font-mono text-gray-900 dark:text-gray-100 bg-gray-50 dark:bg-gray-700 p-3 rounded break-all">
            {incident.public_key}
          </p>
        </div>

        {/* Endpoints */}
        {incident.endpoints && incident.endpoints.length > 0 && (
          <div>
            <label className="block text-sm font-medium text-gray-500 mb-2">Detected Endpoints</label>
            <div className="space-y-1">
              {incident.endpoints.map((endpoint, index) => (
                <div key={index} className="bg-gray-50 dark:bg-gray-700 px-3 py-2 rounded text-sm font-mono text-gray-900 dark:text-gray-100">
                  {endpoint}
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Details */}
        <div>
          <label className="block text-sm font-medium text-gray-500 mb-1">Details</label>
          <p className="text-sm text-gray-900 dark:text-gray-100 bg-gray-50 dark:bg-gray-700 p-3 rounded">
            {incident.details}
          </p>
        </div>

        {/* Detection Time */}
        <div>
          <label className="block text-sm font-medium text-gray-500 mb-1">Detected At</label>
          <p className="text-sm text-gray-900">
            {new Date(incident.detected_at).toLocaleString()}
          </p>
        </div>

        {/* Resolution Info */}
        {incident.resolved && (
          <div className="bg-green-50 rounded-lg p-4 border border-green-200">
            <h4 className="text-sm font-medium text-green-900 mb-2">Resolution Details</h4>
            <div className="space-y-2">
              {incident.resolved_at && (
                <div>
                  <label className="block text-xs font-medium text-green-700 mb-1">Resolved At</label>
                  <p className="text-sm text-green-900">
                    {new Date(incident.resolved_at).toLocaleString()}
                  </p>
                </div>
              )}
              {incident.resolved_by && (
                <div>
                  <label className="block text-xs font-medium text-green-700 mb-1">Resolved By</label>
                  <p className="text-sm text-green-900">{incident.resolved_by}</p>
                </div>
              )}
            </div>
          </div>
        )}

        {/* Actions */}
        <div className="flex justify-end gap-3 pt-4 border-t border-gray-200">
          <button
            onClick={onClose}
            className="px-4 py-2 text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 cursor-pointer transition-colors"
          >
            Close
          </button>
          {!incident.resolved && (
            <button
              onClick={handleResolve}
              disabled={resolving}
              className="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer transition-colors"
            >
              {resolving ? 'Resolving...' : 'Mark as Resolved'}
            </button>
          )}
        </div>
      </div>
    </Modal>
  );
}
