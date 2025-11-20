import { useState, useEffect } from 'react';
import PageHeader from '../../components/PageHeader';
import api from '../../api/client';
import type { SecurityIncident } from '../../types';

export default function SecurityPage() {
  const [incidents, setIncidents] = useState<SecurityIncident[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);

  const pageSize = 20;

  useEffect(() => {
    loadIncidents();
  }, [page]);

  const loadIncidents = async () => {
    setLoading(true);
    try {
      const response = await api.getSecurityIncidents(page, pageSize);
      setIncidents(response.data || []);
      setTotal(response.total || 0);
    } catch (error) {
      console.error('Failed to load security incidents:', error);
      setIncidents([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  const handleResolve = async (incidentId: string) => {
    try {
      await api.resolveIncident(incidentId);
      loadIncidents();
    } catch (error) {
      console.error('Failed to resolve incident:', error);
    }
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
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-gray-500">Loading security incidents...</div>
          </div>
        ) : incidents.length === 0 ? (
          <div className="bg-white rounded-lg border border-gray-200 p-12 text-center">
            <div className="text-gray-400 text-5xl mb-4">🔒</div>
            <h3 className="text-lg font-medium text-gray-900 mb-2">No security incidents</h3>
            <p className="text-gray-500">All systems are secure</p>
          </div>
        ) : (
          <div className="space-y-4">
            {incidents.map((incident) => (
              <div key={incident.id} className="bg-white rounded-lg border border-gray-200 p-6">
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center gap-3 mb-3">
                      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getIncidentTypeColor(incident.incident_type)}`}>
                        {getIncidentTypeLabel(incident.incident_type)}
                      </span>
                      {incident.resolved ? (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                          ✓ Resolved
                        </span>
                      ) : (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800">
                          • Active
                        </span>
                      )}
                    </div>

                    <h3 className="text-lg font-semibold text-gray-900 mb-2">
                      {incident.peer_name} ({incident.network_name})
                    </h3>

                    <p className="text-sm text-gray-600 mb-4">{incident.details}</p>

                    <div className="grid grid-cols-2 gap-4 text-sm">
                      <div>
                        <span className="text-gray-500">Detected:</span>
                        <span className="ml-2 text-gray-900">
                          {new Date(incident.detected_at).toLocaleString()}
                        </span>
                      </div>
                      {incident.resolved_at && (
                        <div>
                          <span className="text-gray-500">Resolved:</span>
                          <span className="ml-2 text-gray-900">
                            {new Date(incident.resolved_at).toLocaleString()}
                          </span>
                        </div>
                      )}
                      <div>
                        <span className="text-gray-500">Endpoints:</span>
                        <span className="ml-2 text-gray-900 font-mono">
                          {incident.endpoints.join(', ')}
                        </span>
                      </div>
                    </div>
                  </div>

                  {!incident.resolved && (
                    <button
                      onClick={() => handleResolve(incident.id)}
                      className="ml-4 px-4 py-2 text-sm font-medium text-white bg-green-600 rounded-lg hover:bg-green-700"
                    >
                      Mark Resolved
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="mt-8 flex items-center justify-between">
            <div className="text-sm text-gray-500">
              Page {page} of {totalPages}
            </div>
            <div className="flex gap-2">
              <button
                onClick={() => setPage(Math.max(1, page - 1))}
                disabled={page === 1}
                className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Previous
              </button>
              <button
                onClick={() => setPage(Math.min(totalPages, page + 1))}
                disabled={page >= totalPages}
                className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Next
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
