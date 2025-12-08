import { useState, useEffect } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faChartBar } from '@fortawesome/free-solid-svg-icons';
import PageHeader from '../../components/PageHeader';
import api from '../../api/client';
import type { IPAMAllocation } from '../../types';
import { useDebounce } from '../../hooks/useDebounce';

export default function IPAMPage() {
  const [allocations, setAllocations] = useState<IPAMAllocation[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [filter, setFilter] = useState('');

  const pageSize = 50;
  const debouncedFilter = useDebounce(filter, 500);

  useEffect(() => {
    const loadAllocations = async () => {
    setLoading(true);
    try {
      const response = await api.getIPAMAllocations(page, pageSize, debouncedFilter);
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

    loadAllocations();
  }, [page, debouncedFilter]);

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div>
      <PageHeader 
        title="IP Address Management" 
        subtitle={`${total} IP addresses tracked`}
      />

      <div className="p-8">

        {/* Search */}
        <div className="mb-6">
          <div className="relative">
            <div className="absolute inset-y-0 left-0 pl-4 flex items-center pointer-events-none">
              <svg className="h-5 w-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
            </div>
            <input
              type="text"
              placeholder="Search by network, IP, or peer name..."
              value={filter}
              onChange={(e) => {
                setFilter(e.target.value);
                setPage(1);
              }}
              className="w-full pl-11 pr-4 py-3 border border-gray-300 dark:border-gray-600 rounded-2xl bg-white/80 dark:bg-gray-800/80 backdrop-blur-sm text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:ring-2 focus:ring-primary-500 focus:border-transparent hover:shadow-md transition-all"
            />
          </div>
        </div>

        {/* Allocations Table */}
        {loading ? (
          <div className="flex flex-col items-center justify-center py-16">
            <div className="inline-block animate-spin rounded-full h-12 w-12 border-4 border-solid border-current border-r-transparent align-[-0.125em] text-primary-600 dark:text-primary-400 motion-reduce:animate-[spin_1.5s_linear_infinite] mb-4"></div>
            <p className="text-gray-600 dark:text-gray-300 font-medium">Loading IP allocations...</p>
          </div>
        ) : allocations.length === 0 ? (
          <div className="bg-gradient-to-br from-white via-gray-50 to-white dark:from-gray-800 dark:via-gray-800/50 dark:to-gray-800 rounded-2xl border border-gray-200 dark:border-gray-700 p-16 text-center shadow-sm">
            <div className="inline-flex items-center justify-center w-20 h-20 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue mb-6">
              <FontAwesomeIcon icon={faChartBar} className="text-3xl text-white" />
            </div>
            <h3 className="text-xl font-bold text-gray-900 dark:text-gray-100 mb-2">No IP allocations found</h3>
            <p className="text-gray-600 dark:text-gray-300">
              {filter ? 'Try adjusting your search criteria' : 'IP allocations will appear here'}
            </p>
          </div>
        ) : (
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead className="bg-gray-50 dark:bg-gray-700">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    IP Address
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    Network
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    Peer
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    Status
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                {allocations.map((allocation, idx) => (
                  <tr key={`${allocation.network_id}-${allocation.ip}-${idx}`} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm font-mono font-medium text-gray-900 dark:text-gray-100">{allocation.ip}</div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm text-gray-900 dark:text-gray-100">{allocation.network_name}</div>
                      <div className="text-sm text-gray-600 dark:text-gray-300 font-mono">{allocation.network_cidr}</div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100">
                      {allocation.peer_name || '-'}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm">
                      {allocation.allocated ? (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">
                          Allocated
                        </span>
                      ) : (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200">
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
            <div className="text-sm font-medium text-gray-600 dark:text-gray-300">
              Page {page} of {totalPages}
            </div>
            <div className="flex gap-2">
              <button
                onClick={() => setPage(Math.max(1, page - 1))}
                disabled={page === 1}
                className="px-5 py-2.5 text-sm font-semibold text-gray-700 dark:text-gray-200 bg-white/80 dark:bg-gray-800/80 backdrop-blur-sm border border-gray-300 dark:border-gray-600 rounded-xl hover:bg-white dark:hover:bg-gray-800 hover:shadow-md disabled:opacity-50 disabled:cursor-not-allowed transition-all"
              >
                Previous
              </button>
              <button
                onClick={() => setPage(Math.min(totalPages, page + 1))}
                disabled={page >= totalPages}
                className="px-5 py-2.5 text-sm font-semibold text-gray-700 dark:text-gray-200 bg-white/80 dark:bg-gray-800/80 backdrop-blur-sm border border-gray-300 dark:border-gray-600 rounded-xl hover:bg-white dark:hover:bg-gray-800 hover:shadow-md disabled:opacity-50 disabled:cursor-not-allowed transition-all"
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
