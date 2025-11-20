import { useState, useEffect } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faNetworkWired } from '@fortawesome/free-solid-svg-icons';
import PageHeader from '../../components/PageHeader';
import NetworkModal from '../../components/NetworkModal';
import NetworkDetailModal from '../../components/NetworkDetailModal';
import api from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';
import type { Network } from '../../types';
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

  useEffect(() => {
    loadNetworks();
  }, [page, debouncedFilter]);

  const loadNetworks = async () => {
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
  };

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
              className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 flex items-center gap-2 cursor-pointer transition-colors"
            >
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              Create Network
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

        {/* Networks Grid */}
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-gray-500">Loading networks...</div>
          </div>
        ) : networks.length === 0 ? (
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-12 text-center">
            <div className="text-gray-400 dark:text-gray-500 text-5xl mb-4">
              <FontAwesomeIcon icon={faNetworkWired} />
            </div>
            <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">No networks found</h3>
            <p className="text-gray-500 dark:text-gray-400">
              {filter ? 'Try adjusting your search criteria' : 'Get started by creating your first network'}
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {networks.map((network) => (
              <div
                key={network.id}
                onClick={() => handleNetworkClick(network)}
                className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6 hover:border-primary-300 dark:hover:border-primary-500 hover:shadow-md transition-all cursor-pointer"
              >
                  <div className="flex items-start justify-between mb-4">
                    <div className="flex-1">
                      <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-1">
                        {network.name}
                      </h3>
                      <p className="text-sm text-gray-500 dark:text-gray-400">{network.cidr}</p>
                    </div>
                    <span className="text-2xl">
                      <FontAwesomeIcon icon={faNetworkWired} className="text-primary-600 dark:text-primary-400" />
                    </span>
                  </div>
                  
                  <div className="space-y-2">
                    <div className="flex justify-between text-sm">
                      <span className="text-gray-500 dark:text-gray-400">Domain</span>
                      <span className="text-gray-900 dark:text-white font-medium">{network.domain}</span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-gray-500 dark:text-gray-400">Peers</span>
                      <span className="text-gray-900 dark:text-white font-medium">
                        {network.peer_count ?? 0}
                      </span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-gray-500 dark:text-gray-400">Available Slots</span>
                      <span className="text-gray-900 dark:text-white font-medium">
                        {(() => {
                          const capacity = computeCapacityFromCIDR(network.cidr);
                          const used = network.peer_count ?? 0;
                          const available = capacity !== null ? capacity - used : 0;
                          return `${available} / ${capacity ?? 0}`;
                        })()}
                      </span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-gray-500 dark:text-gray-400">Created</span>
                      <span className="text-gray-900 dark:text-white font-medium">
                        {new Date(network.created_at).toLocaleDateString()}
                      </span>
                    </div>
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
