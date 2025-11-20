import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import PageHeader from '../../components/PageHeader';
import NetworkModal from '../../components/NetworkModal';
import api from '../../api/client';
import type { Network } from '../../types';
import { computeCapacityFromCIDR } from '../../utils/networkCapacity';

export default function NetworksPage() {
  const [networks, setNetworks] = useState<Network[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [filter, setFilter] = useState('');
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingNetwork, setEditingNetwork] = useState<Network | null>(null);

  const pageSize = 20;

  useEffect(() => {
    loadNetworks();
  }, [page, filter]);

  const loadNetworks = async () => {
    setLoading(true);
    try {
      const response = await api.getNetworks(page, pageSize, filter);
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

  const handleDelete = async (network: Network) => {
    if (!confirm(`Are you sure you want to delete network "${network.name}"? This action cannot be undone.`)) {
      return;
    }
    
    try {
      await api.deleteNetwork(network.id);
      loadNetworks();
    } catch (error: any) {
      alert(error.response?.data?.error || 'Failed to delete network');
    }
  };

  const handleEdit = (network: Network) => {
    setEditingNetwork(network);
    setIsModalOpen(true);
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
          <button
            onClick={handleCreate}
            className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 flex items-center gap-2"
          >
            <span className="text-xl">+</span>
            Create Network
          </button>
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
            className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent"
          />
        </div>

        {/* Networks Grid */}
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-gray-500">Loading networks...</div>
          </div>
        ) : networks.length === 0 ? (
          <div className="bg-white rounded-lg border border-gray-200 p-12 text-center">
            <div className="text-gray-400 text-5xl mb-4">🌐</div>
            <h3 className="text-lg font-medium text-gray-900 mb-2">No networks found</h3>
            <p className="text-gray-500">
              {filter ? 'Try adjusting your search criteria' : 'Get started by creating your first network'}
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {networks.map((network) => (
              <div
                key={network.id}
                className="bg-white rounded-lg border border-gray-200 hover:border-primary-300 hover:shadow-md transition-all"
              >
                <Link
                  to={`/networks/${network.id}`}
                  className="block p-6"
                >
                  <div className="flex items-start justify-between mb-4">
                    <div className="flex-1">
                      <h3 className="text-lg font-semibold text-gray-900 mb-1">
                        {network.name}
                      </h3>
                      <p className="text-sm text-gray-500">{network.cidr}</p>
                    </div>
                    <span className="text-2xl">🌐</span>
                  </div>
                  
                  <div className="space-y-2">
                    <div className="flex justify-between text-sm">
                      <span className="text-gray-500">Domain</span>
                      <span className="text-gray-900 font-medium">{network.domain}</span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-gray-500">Peers</span>
                      <span className="text-gray-900 font-medium">
                        {network.peer_count ?? 0}
                      </span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-gray-500">Available Slots</span>
                      <span className="text-gray-900 font-medium">
                        {(() => {
                          const capacity = computeCapacityFromCIDR(network.cidr);
                          const used = network.peer_count ?? 0;
                          const available = capacity !== null ? capacity - used : 0;
                          return `${available} / ${capacity ?? 0}`;
                        })()}
                      </span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-gray-500">Created</span>
                      <span className="text-gray-900 font-medium">
                        {new Date(network.created_at).toLocaleDateString()}
                      </span>
                    </div>
                  </div>
                </Link>
                
                {/* Action Buttons */}
                <div className="px-6 pb-4 flex gap-2 border-t border-gray-100 pt-4">
                  <button
                    onClick={(e) => {
                      e.preventDefault();
                      handleEdit(network);
                    }}
                    className="flex-1 px-3 py-2 text-sm text-gray-700 bg-gray-100 rounded hover:bg-gray-200 transition-colors"
                  >
                    Edit
                  </button>
                  <button
                    onClick={(e) => {
                      e.preventDefault();
                      handleDelete(network);
                    }}
                    className="flex-1 px-3 py-2 text-sm text-red-600 bg-red-50 rounded hover:bg-red-100 transition-colors"
                  >
                    Delete
                  </button>
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

      {/* Network Modal */}
      <NetworkModal
        isOpen={isModalOpen}
        onClose={handleModalClose}
        onSuccess={handleModalSuccess}
        network={editingNetwork}
      />
    </div>
  );
}
