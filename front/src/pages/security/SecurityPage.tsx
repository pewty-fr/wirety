import { useState, useEffect, useMemo } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faShieldHalved } from '@fortawesome/free-solid-svg-icons';
import PageHeader from '../../components/PageHeader';
import IncidentDetailModal from '../../components/IncidentDetailModal';
import SearchableSelect from '../../components/SearchableSelect';
import { useNetworks } from '../../hooks/useQueries';
import api from '../../api/client';
import type { SecurityIncident } from '../../types';

type FilterStatus = 'all' | 'active' | 'resolved';

export default function SecurityPage() {
  const [incidents, setIncidents] = useState<SecurityIncident[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [selectedIncident, setSelectedIncident] = useState<SecurityIncident | null>(null);
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false);
  const [filterStatus, setFilterStatus] = useState<FilterStatus>('all');
  const [filterNetwork, setFilterNetwork] = useState<string>('');
  const [filterPeer, setFilterPeer] = useState<string>('');

  const pageSize = 20;

  // Fetch networks for filtering
  const { data: networks = [] } = useNetworks();

  // Reset page when filters change
  useEffect(() => {
    setPage(1);
  }, [filterNetwork, filterPeer, filterStatus]);

  useEffect(() => {
    const loadIncidents = async () => {
    setLoading(true);
    try {
      const resolved = filterStatus === 'all' ? undefined : filterStatus === 'resolved';
      const response = await api.getSecurityIncidents(page, pageSize, resolved);
      // Handle both paginated response and plain array (like users endpoint)
      const incidentsArray = Array.isArray(response) ? response : (response.data || []);
      setIncidents(incidentsArray);
    } catch (error) {
      console.error('Failed to load security incidents:', error);
      setIncidents([]);
    } finally {
      setLoading(false);
    }
  };

    loadIncidents();
  }, [page, filterStatus, filterNetwork, filterPeer]);

  // Get unique peers from incidents for filter dropdown (filtered by network if selected)
  const uniquePeers = useMemo(() => {
    const peers = new Map<string, { id: string; name: string; network_name: string; network_id: string }>();
    incidents.forEach(incident => {
      if (incident.peer_id && incident.peer_name) {
        // Only include peers from selected network if network filter is active
        if (!filterNetwork || incident.network_id === filterNetwork) {
          peers.set(incident.peer_id, {
            id: incident.peer_id,
            name: incident.peer_name,
            network_name: incident.network_name || 'Unknown',
            network_id: incident.network_id
          });
        }
      }
    });
    return Array.from(peers.values()).sort((a, b) => a.name.localeCompare(b.name));
  }, [incidents, filterNetwork]);

  // Create options for SearchableSelect components
  const networkOptions = useMemo(() => 
    networks.map(network => ({
      value: network.id,
      label: network.name
    })), [networks]
  );

  const peerOptions = useMemo(() => 
    uniquePeers.map(peer => ({
      value: peer.id,
      label: peer.name,
      sublabel: peer.network_name
    })), [uniquePeers]
  );

  // Filter incidents based on selected filters
  const filteredIncidents = useMemo(() => {
    return incidents.filter(incident => {
      if (filterNetwork && incident.network_id !== filterNetwork) {
        return false;
      }
      if (filterPeer && incident.peer_id !== filterPeer) {
        return false;
      }
      return true;
    });
  }, [incidents, filterNetwork, filterPeer]);

  const handleFilterChange = (status: FilterStatus) => {
    setFilterStatus(status);
    setPage(1); // Reset to first page when changing filter
  };

  const handleIncidentClick = (incident: SecurityIncident) => {
    setSelectedIncident(incident);
    setIsDetailModalOpen(true);
  };

  const filteredTotal = filteredIncidents.length;
  const totalPages = Math.ceil(filteredTotal / pageSize);
  const unresolvedCount = filteredIncidents.filter(i => !i.resolved).length;

  // Paginate the filtered incidents
  const paginatedIncidents = filteredIncidents.slice((page - 1) * pageSize, page * pageSize);

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
        subtitle={`${filteredTotal} incident${filteredTotal !== 1 ? 's' : ''} shown, ${unresolvedCount} unresolved`}
      />

      <div className="p-8">
        {/* Filters */}
        <div className="bg-white/80 dark:bg-gray-800/80 backdrop-blur-sm rounded-2xl border border-gray-200 dark:border-gray-700 p-6 mb-6 shadow-sm">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {/* Network Filter */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-200 mb-2">Network</label>
              <SearchableSelect
                options={networkOptions}
                value={filterNetwork}
                onChange={(value) => {
                  setFilterNetwork(value);
                  setFilterPeer(''); // Clear peer filter when network changes
                  setPage(1);
                }}
                placeholder="All Networks"
              />
            </div>

            {/* Peer Filter */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-200 mb-2">Peer</label>
              <SearchableSelect
                options={peerOptions}
                value={filterPeer}
                onChange={(value) => {
                  setFilterPeer(value);
                  setPage(1);
                }}
                placeholder="All Peers"
              />
            </div>

            {/* Status Filter */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-200 mb-2">Status</label>
              <select
                value={filterStatus}
                onChange={(e) => handleFilterChange(e.target.value as FilterStatus)}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
              >
                <option value="all">All Incidents</option>
                <option value="active">Active</option>
                <option value="resolved">Resolved</option>
              </select>
            </div>
          </div>

          {/* Clear Filters */}
          {(filterNetwork || filterPeer || filterStatus !== 'all') && (
            <div className="mt-4 flex justify-end">
              <button
                onClick={() => {
                  setFilterNetwork('');
                  setFilterPeer('');
                  setFilterStatus('all');
                  setPage(1);
                }}
                className="text-sm text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
              >
                Clear all filters
              </button>
            </div>
          )}
        </div>

        {loading ? (
          <div className="flex flex-col items-center justify-center py-16">
            <div className="inline-block animate-spin rounded-full h-12 w-12 border-4 border-solid border-current border-r-transparent align-[-0.125em] text-primary-600 dark:text-primary-400 motion-reduce:animate-[spin_1.5s_linear_infinite] mb-4"></div>
            <p className="text-gray-600 dark:text-gray-300 font-medium">Loading security incidents...</p>
          </div>
        ) : filteredIncidents.length === 0 ? (
          <div className="bg-gradient-to-br from-white via-gray-50 to-white dark:from-gray-800 dark:via-gray-800/50 dark:to-gray-800 rounded-2xl border border-gray-200 dark:border-gray-700 p-16 text-center shadow-sm">
            <div className="inline-flex items-center justify-center w-20 h-20 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue mb-6">
              <FontAwesomeIcon icon={faShieldHalved} className="text-3xl text-white" />
            </div>
            <h3 className="text-xl font-bold text-gray-900 dark:text-gray-100 mb-2">No security incidents</h3>
            <p className="text-gray-600 dark:text-gray-300">
              {filterNetwork || filterPeer ? 'No incidents match the selected filters' : 'All systems are secure'}
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            {paginatedIncidents.map((incident) => (
              <div 
                key={incident.id} 
                onClick={() => handleIncidentClick(incident)}
                className="bg-white/80 dark:bg-gray-800/80 backdrop-blur-sm rounded-2xl border border-gray-200 dark:border-gray-700 p-6 hover:border-primary-300 dark:hover:border-primary-500 hover:scale-[1.01] hover:shadow-xl transition-all cursor-pointer"
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

                    <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-2">
                      {incident.peer_name} ({incident.network_name})
                    </h3>

                    <p className="text-sm text-gray-600 dark:text-gray-300 mb-4">{incident.details}</p>

                    <div className="grid grid-cols-2 gap-4 text-sm">
                      <div>
                        <span className="text-gray-600 dark:text-gray-300">Detected:</span>
                        <span className="ml-2 text-gray-900 dark:text-gray-100">
                          {new Date(incident.detected_at).toLocaleString()}
                        </span>
                      </div>
                      {incident.resolved_at && incident.resolved && (
                        <div>
                          <span className="text-gray-600 dark:text-gray-300">Resolved:</span>
                          <span className="ml-2 text-gray-900 dark:text-gray-100">
                            {new Date(incident.resolved_at).toLocaleString()}
                          </span>
                        </div>
                      )}
                      <div>
                        <span className="text-gray-600 dark:text-gray-300">Endpoints:</span>
                        <span className="ml-2 text-gray-900 dark:text-gray-100 font-mono">
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
        {!loading && filteredIncidents.length > 0 && totalPages > 1 && (
          <div className="mt-8 flex items-center justify-between">
            <div className="text-sm text-gray-500 dark:text-gray-400">
              Page {page} of {totalPages} ({filteredTotal} incident{filteredTotal !== 1 ? 's' : ''})
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
