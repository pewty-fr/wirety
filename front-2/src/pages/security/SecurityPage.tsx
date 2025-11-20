import { useState, useEffect } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faShieldHalved } from '@fortawesome/free-solid-svg-icons';
import PageHeader from '../../components/PageHeader';
import IncidentDetailModal from '../../components/IncidentDetailModal';
import api from '../../api/client';
import type { SecurityIncident } from '../../types';

type FilterStatus = 'all' | 'active' | 'resolved';

export default function SecurityPage() {
  const [incidents, setIncidents] = useState<SecurityIncident[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [selectedIncident, setSelectedIncident] = useState<SecurityIncident | null>(null);
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false);
  const [filterStatus, setFilterStatus] = useState<FilterStatus>('all');

  const pageSize = 20;

  useEffect(() => {
    loadIncidents();
  }, [page, filterStatus]);

  const loadIncidents = async () => {
    setLoading(true);
    try {
      const resolved = filterStatus === 'all' ? undefined : filterStatus === 'resolved';
      const response = await api.getSecurityIncidents(page, pageSize, resolved);
      // Handle both paginated response and plain array (like users endpoint)
      const incidentsArray = Array.isArray(response) ? response : (response.data || []);
      const totalCount = Array.isArray(response) ? response.length : (response.total || 0);
      setIncidents(incidentsArray);
      setTotal(totalCount);
    } catch (error) {
      console.error('Failed to load security incidents:', error);
      setIncidents([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  const handleFilterChange = (status: FilterStatus) => {
    setFilterStatus(status);
    setPage(1); // Reset to first page when changing filter
  };

  const handleIncidentClick = (incident: SecurityIncident) => {
    setSelectedIncident(incident);
    setIsDetailModalOpen(true);
  };

  const totalPages = Math.ceil(total / pageSize);
  const unresolvedCount = incidents.filter(i => !i.resolved).length;

  const getIncidentTypeLabel = (type: string) => {
    switch (type) {
      case 'shared_config':
        return 'Shared Config';
      case 'session_conflict':
        return 'Session Conflict';
      case 'suspicious_activity':
        return 'Suspicious Activity';
      default:
        return type;
    }
  };

  const getIncidentTypeColor = (type: string) => {
    switch (type) {
      case 'shared_config':
        return 'bg-orange-100 text-orange-800';
      case 'session_conflict':
        return 'bg-red-100 text-red-800';
      case 'suspicious_activity':
        return 'bg-purple-100 text-purple-800';
      default:
        return 'bg-gray-100 text-gray-800';
    }
  };

  return (
    <div>
      <PageHeader 
        title="Security Incidents" 
        subtitle={`${total} incident${total !== 1 ? 's' : ''} tracked, ${unresolvedCount} unresolved`}
      />

      <div className="p-8">
        {/* Filter Segmented Button */}
        <div className="mb-6 flex items-center gap-2">
          <span className="text-sm font-medium text-gray-700 dark:text-gray-300 mr-2">Filter:</span>
          <div className="inline-flex rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800">
            <button
              onClick={() => handleFilterChange('all')}
              className={`px-4 py-2 text-sm font-medium transition-colors ${
                filterStatus === 'all'
                  ? 'bg-primary-500 text-white'
                  : 'text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'
              } rounded-l-lg`}
            >
              All
            </button>
            <button
              onClick={() => handleFilterChange('active')}
              className={`px-4 py-2 text-sm font-medium transition-colors border-x border-gray-200 dark:border-gray-700 ${
                filterStatus === 'active'
                  ? 'bg-primary-500 text-white'
                  : 'text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'
              }`}
            >
              Active
            </button>
            <button
              onClick={() => handleFilterChange('resolved')}
              className={`px-4 py-2 text-sm font-medium transition-colors ${
                filterStatus === 'resolved'
                  ? 'bg-primary-500 text-white'
                  : 'text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'
              } rounded-r-lg`}
            >
              Resolved
            </button>
          </div>
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-gray-500">Loading security incidents...</div>
          </div>
        ) : incidents.length === 0 ? (
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-12 text-center">
            <div className="text-gray-400 text-5xl mb-4">
              <FontAwesomeIcon icon={faShieldHalved} />
            </div>
            <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">No security incidents</h3>
            <p className="text-gray-500 dark:text-gray-400">All systems are secure</p>
          </div>
        ) : (
          <div className="space-y-4">
            {incidents.map((incident) => (
              <div 
                key={incident.id} 
                onClick={() => handleIncidentClick(incident)}
                className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6 hover:border-primary-300 dark:hover:border-primary-500 hover:shadow-md transition-all cursor-pointer"
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center gap-3 mb-3">
                      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getIncidentTypeColor(incident.incident_type)}`}>
                        {getIncidentTypeLabel(incident.incident_type)}
                      </span>
                      {incident.resolved ? (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">
                          ✓ Resolved
                        </span>
                      ) : (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200">
                          • Active
                        </span>
                      )}
                    </div>

                    <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
                      {incident.peer_name} ({incident.network_name})
                    </h3>

                    <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">{incident.details}</p>

                    <div className="grid grid-cols-2 gap-4 text-sm">
                      <div>
                        <span className="text-gray-500 dark:text-gray-400">Detected:</span>
                        <span className="ml-2 text-gray-900 dark:text-white">
                          {new Date(incident.detected_at).toLocaleString()}
                        </span>
                      </div>
                      {incident.resolved_at && (
                        <div>
                          <span className="text-gray-500 dark:text-gray-400">Resolved:</span>
                          <span className="ml-2 text-gray-900 dark:text-white">
                            {new Date(incident.resolved_at).toLocaleString()}
                          </span>
                        </div>
                      )}
                      <div>
                        <span className="text-gray-500 dark:text-gray-400">Endpoints:</span>
                        <span className="ml-2 text-gray-900 dark:text-white font-mono">
                          {incident.endpoints.join(', ')}
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Pagination */}
        {!loading && incidents.length > 0 && totalPages > 1 && (
          <div className="mt-8 flex items-center justify-between">
            <div className="text-sm text-gray-500 dark:text-gray-400">
              Page {page} of {totalPages} ({total} incident{total !== 1 ? 's' : ''})
            </div>
            <div className="flex gap-2">
              <button
                onClick={() => setPage(Math.max(1, page - 1))}
                disabled={page === 1}
                className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                Previous
              </button>
              <button
                onClick={() => setPage(Math.min(totalPages, page + 1))}
                disabled={page >= totalPages}
                className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                Next
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Detail Modal */}
      <IncidentDetailModal
        isOpen={isDetailModalOpen}
        onClose={() => setIsDetailModalOpen(false)}
        incident={selectedIncident}
        onUpdate={loadIncidents}
      />
    </div>
  );
}
