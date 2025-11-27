import { useState, useEffect, useCallback } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faNetworkWired, faPencil, faTrash } from '@fortawesome/free-solid-svg-icons';
import PageHeader from '../../components/PageHeader';
import NetworkModal from '../../components/NetworkModal';
import NetworkDetailModal from '../../components/NetworkDetailModal';
import api from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';
import type { Network } from '../../types';
import { getNetworkDomain } from '../../types';
import { computeCapacityFromCIDR } from '../../utils/networkCapacity';
import { useDebounce } from '../../hooks/useDebounce';

export default function NetworksPage() {
  const { user } = useAuth();
  const [networks, setNetworks] = useState<Network[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [filter, setFilter] = useState('');
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingNetwork, setEditingNetwork] = useState<Network | null>(null);
  const [selectedNetwork, setSelectedNetwork] = useState<Network | null>(null);
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false);

  const isAdmin = user?.role === 'administrator';
  const pageSize = 20;
  const debouncedFilter = useDebounce(filter, 500);

  const loadNetworks = useCallback(async () => {
    setLoading(true);
    try {
      const response = await api.getNetworks(page, pageSize, debouncedFilter);
      setNetworks(response.data || []);
      setTotal(response.total || 0);
    } catch (error) {
      console.error('Failed to load networks:', error);
      setNetworks([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, [page, debouncedFilter]);

  useEffect(() => {
    void loadNetworks();
  }, [loadNetworks]);

  const handleNetworkClick = (network: Network) => {
    setSelectedNetwork(network);
    setIsDetailModalOpen(true);
  };

  const handleCreate = () => {
    setEditingNetwork(null);
    setIsModalOpen(true);
  };

  const handleModalClose = () => {
    setIsModalOpen(false);
    setEditingNetwork(null);
  };

  const handleModalSuccess = () => {
    loadNetworks();
  };

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div>
      <PageHeader 
        title="Networks" 
        subtitle={`${total} network${total !== 1 ? 's' : ''} total`}
        action={
          isAdmin ? (
            <button
              onClick={handleCreate}
              className="px-4 py-2.5 bg-gradient-to-r from-primary-600 to-accent-blue text-white rounded-xl hover:scale-105 active:scale-95 shadow-lg hover:shadow-xl flex items-center gap-2 cursor-pointer transition-all font-semibold"
            >
              <svg className="w-5 h-5 group-hover:rotate-90 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              Network
            </button>
          ) : undefined
        }
      />

      <div className="p-8">
        {/* Search */}
        <div className="mb-6">
          <input
            type="text"
            placeholder="Search networks by name or CIDR..."
            value={filter}
            onChange={(e) => {
              setFilter(e.target.value);
              setPage(1);
            }}
            className="w-full px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400"
          />
        </div>

        {/* Networks List */}
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-gray-500">Loading networks...</div>
          </div>
        ) : networks.length === 0 ? (
          <div className="bg-gradient-to-br from-white via-gray-50 to-white dark:from-gray-800 dark:via-gray-800/50 dark:to-gray-800 rounded-2xl border border-gray-200 dark:border-gray-700 p-16 text-center shadow-sm">
            <div className="inline-flex items-center justify-center w-20 h-20 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue mb-6">
              <FontAwesomeIcon icon={faNetworkWired} className="text-3xl text-white" />
            </div>
            <h3 className="text-xl font-bold text-gray-900 dark:text-white mb-2">No networks found</h3>
            <p className="text-gray-600 dark:text-gray-300">
              {filter ? 'Try adjusting your search criteria' : 'Get started by creating your first network'}
            </p>
          </div>
        ) : (
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead className="bg-gray-50 dark:bg-gray-700">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Name</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">CIDR</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Domain</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Peers</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Available</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Created</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Actions</th>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                {networks.map((network) => {
                  const capacity = computeCapacityFromCIDR(network.cidr);
                  const used = network.peer_count ?? 0;
                  const available = capacity !== null ? capacity - used : 0;
                  
                  return (
                    <tr
                      key={network.id}
                      onClick={() => handleNetworkClick(network)}
                      className="hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer"
                    >
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="flex items-center">
                          <div className="inline-flex items-center justify-center w-10 h-10 rounded-xl bg-gradient-to-br from-primary-500 to-accent-blue mr-3">
                            <FontAwesomeIcon icon={faNetworkWired} className="text-lg text-white" />
                          </div>
                          <div className="text-sm font-medium text-gray-900 dark:text-white">{network.name}</div>
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-mono text-gray-900 dark:text-white">
                        {network.cidr}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                        {getNetworkDomain(network)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                        {used}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                        {available} / {capacity ?? 0}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                        {new Date(network.created_at).toLocaleDateString()}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm">
                        {isAdmin && (
                          <div className="flex items-center gap-2">
                            <button
                              onClick={(e) => {
                                e.stopPropagation();
                                setEditingNetwork(network);
                                setIsModalOpen(true);
                              }}
                              className="text-primary-600 hover:text-primary-800 dark:text-primary-400 dark:hover:text-primary-300 transition-colors"
                              title="Edit network"
                            >
                              <FontAwesomeIcon icon={faPencil} />
                            </button>
                            <button
                              onClick={async (e) => {
                                e.stopPropagation();
                                if (confirm(`Are you sure you want to delete network "${network.name}"? This will also delete all associated peers.`)) {
                                  try {
                                    await api.deleteNetwork(network.id);
                                    loadNetworks();
                                  } catch (error) {
                                    console.error('Failed to delete network:', error);
                                    alert('Failed to delete network');
                                  }
                                }
                              }}
                              className="text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300 transition-colors"
                              title="Delete network"
                            >
                              <FontAwesomeIcon icon={faTrash} />
                            </button>
                          </div>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="mt-8 flex items-center justify-between">
            <div className="text-sm text-gray-500 dark:text-gray-400">
              Page {page} of {totalPages}
            </div>
            <div className="flex gap-2">
              <button
                onClick={() => setPage(Math.max(1, page - 1))}
                disabled={page === 1}
                className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                Previous
              </button>
              <button
                onClick={() => setPage(Math.min(totalPages, page + 1))}
                disabled={page >= totalPages}
                className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                Next
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Create/Edit Modal */}
      <NetworkModal
        isOpen={isModalOpen}
        onClose={handleModalClose}
        onSuccess={handleModalSuccess}
        network={editingNetwork}
      />

      {/* Detail Modal */}
      <NetworkDetailModal
        isOpen={isDetailModalOpen}
        onClose={() => setIsDetailModalOpen(false)}
        network={selectedNetwork}
        onUpdate={loadNetworks}
      />
    </div>
  );
}
