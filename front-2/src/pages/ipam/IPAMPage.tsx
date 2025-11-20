import { useState, useEffect } from 'react';
import PageHeader from '../../components/PageHeader';
import api from '../../api/client';
import type { IPAMAllocation } from '../../types';

export default function IPAMPage() {
  const [allocations, setAllocations] = useState<IPAMAllocation[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [filter, setFilter] = useState('');

  const pageSize = 50;

  useEffect(() => {
    loadAllocations();
  }, [page, filter]);

  const loadAllocations = async () => {
    setLoading(true);
    try {
      const response = await api.getIPAMAllocations(page, pageSize, filter);
      setAllocations(response.data || []);
      setTotal(response.total || 0);
    } catch (error) {
      console.error('Failed to load IPAM allocations:', error);
      setAllocations([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  const totalPages = Math.ceil(total / pageSize);
  const allocatedCount = allocations.filter(a => a.allocated).length;
  const availableCount = allocations.filter(a => !a.allocated).length;

  return (
    <div>
      <PageHeader 
        title="IP Address Management" 
        subtitle={`${total} IP addresses tracked`}
      />

      <div className="p-8">
        {/* Stats */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-6">
          <div className="bg-white rounded-lg border border-gray-200 p-6">
            <div className="text-sm font-medium text-gray-500 mb-1">Total IPs</div>
            <div className="text-3xl font-bold text-gray-900">{total}</div>
          </div>
          <div className="bg-white rounded-lg border border-gray-200 p-6">
            <div className="text-sm font-medium text-gray-500 mb-1">Allocated</div>
            <div className="text-3xl font-bold text-green-600">{allocatedCount}</div>
          </div>
          <div className="bg-white rounded-lg border border-gray-200 p-6">
            <div className="text-sm font-medium text-gray-500 mb-1">Available</div>
            <div className="text-3xl font-bold text-blue-600">{availableCount}</div>
          </div>
        </div>

        {/* Search */}
        <div className="mb-6">
          <input
            type="text"
            placeholder="Search by network, IP, or peer name..."
            value={filter}
            onChange={(e) => {
              setFilter(e.target.value);
              setPage(1);
            }}
            className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent"
          />
        </div>

        {/* Allocations Table */}
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-gray-500">Loading IP allocations...</div>
          </div>
        ) : allocations.length === 0 ? (
          <div className="bg-white rounded-lg border border-gray-200 p-12 text-center">
            <div className="text-gray-400 text-5xl mb-4">📊</div>
            <h3 className="text-lg font-medium text-gray-900 mb-2">No IP allocations found</h3>
            <p className="text-gray-500">
              {filter ? 'Try adjusting your search criteria' : 'IP allocations will appear here'}
            </p>
          </div>
        ) : (
          <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    IP Address
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Network
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Peer
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Status
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {allocations.map((allocation, idx) => (
                  <tr key={`${allocation.network_id}-${allocation.ip}-${idx}`} className="hover:bg-gray-50">
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm font-mono font-medium text-gray-900">{allocation.ip}</div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm text-gray-900">{allocation.network_name}</div>
                      <div className="text-sm text-gray-500 font-mono">{allocation.network_cidr}</div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                      {allocation.peer_name || '-'}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm">
                      {allocation.allocated ? (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                          Allocated
                        </span>
                      ) : (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800">
                          Available
                        </span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
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
