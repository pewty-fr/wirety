import { useState, useEffect } from 'react';
import PageHeader from '../../components/PageHeader';
import JumpPeerModal from '../../components/JumpPeerModal';
import RegularPeerModal from '../../components/RegularPeerModal';
import api from '../../api/client';
import type { Peer, Network } from '../../types';

export default function PeersPage() {
  const [peers, setPeers] = useState<Peer[]>([]);
  const [networks, setNetworks] = useState<Network[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [isJumpModalOpen, setIsJumpModalOpen] = useState(false);
  const [isRegularModalOpen, setIsRegularModalOpen] = useState(false);
  const [editingPeer, setEditingPeer] = useState<Peer | null>(null);
  const [selectedNetworkId, setSelectedNetworkId] = useState<string>('');

  const pageSize = 20;

  useEffect(() => {
    loadNetworks();
  }, []);

  useEffect(() => {
    loadPeers();
  }, [page]);

  const loadNetworks = async () => {
    try {
      const response = await api.getNetworks(1, 100);
      setNetworks(response.data || []);
    } catch (error) {
      console.error('Failed to load networks:', error);
    }
  };

  const loadPeers = async () => {
    setLoading(true);
    try {
      const response = await api.getAllPeers(page, pageSize);
      setPeers(response.peers || []);
      setTotal(response.total || 0);
    } catch (error) {
      console.error('Failed to load peers:', error);
      setPeers([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (peer: Peer) => {
    if (!confirm(`Are you sure you want to delete peer "${peer.name}"?`)) {
      return;
    }
    
    try {
      await api.deletePeer(peer.network_id!, peer.id);
      loadPeers();
    } catch (error: any) {
      alert(error.response?.data?.error || 'Failed to delete peer');
    }
  };

  const handleEdit = (peer: Peer) => {
    setEditingPeer(peer);
    setSelectedNetworkId(peer.network_id || '');
    if (peer.is_jump) {
      setIsJumpModalOpen(true);
    } else {
      setIsRegularModalOpen(true);
    }
  };

  const handleCreateJump = () => {
    if (networks.length === 0) {
      alert('Please create a network first');
      return;
    }
    setEditingPeer(null);
    setSelectedNetworkId(networks[0].id);
    setIsJumpModalOpen(true);
  };

  const handleCreateRegular = () => {
    if (networks.length === 0) {
      alert('Please create a network first');
      return;
    }
    setEditingPeer(null);
    setSelectedNetworkId(networks[0].id);
    setIsRegularModalOpen(true);
  };

  const handleModalClose = () => {
    setIsJumpModalOpen(false);
    setIsRegularModalOpen(false);
    setEditingPeer(null);
    setSelectedNetworkId('');
  };

  const handleModalSuccess = () => {
    loadPeers();
  };

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div>
      <PageHeader 
        title="Peers" 
        subtitle={`${total} peer${total !== 1 ? 's' : ''} total`}
        action={
          <div className="flex gap-2">
            <button
              onClick={handleCreateJump}
              className="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 flex items-center gap-2"
            >
              <span className="text-xl">+</span>
              Jump Peer
            </button>
            <button
              onClick={handleCreateRegular}
              className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 flex items-center gap-2"
            >
              <span className="text-xl">+</span>
              Regular Peer
            </button>
          </div>
        }
      />

      <div className="p-8">
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-gray-500">Loading peers...</div>
          </div>
        ) : peers.length === 0 ? (
          <div className="bg-white rounded-lg border border-gray-200 p-12 text-center">
            <div className="text-gray-400 text-5xl mb-4">💻</div>
            <h3 className="text-lg font-medium text-gray-900 mb-2">No peers found</h3>
            <p className="text-gray-500">Peers will appear here once they are created</p>
          </div>
        ) : (
          <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Name
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Network
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Address
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Type
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Status
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {peers.map((peer) => (
                  <tr key={peer.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex items-center">
                        <span className="text-2xl mr-3">💻</span>
                        <div>
                          <div className="text-sm font-medium text-gray-900">{peer.name}</div>
                          <div className="text-sm text-gray-500">{peer.endpoint || 'No endpoint'}</div>
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                      {peer.network_name || peer.network_id}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 font-mono">
                      {peer.address}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm">
                      {peer.is_jump ? (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-purple-100 text-purple-800">
                          Jump Server
                        </span>
                      ) : peer.is_isolated ? (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800">
                          Isolated
                        </span>
                      ) : (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                          Regular
                        </span>
                      )}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm">
                      <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
                        {peer.use_agent ? 'Agent' : 'Static'}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm">
                      <div className="flex gap-2">
                        <button
                          onClick={() => handleEdit(peer)}
                          className="text-gray-700 hover:text-gray-900"
                        >
                          Edit
                        </button>
                        <button
                          onClick={() => handleDelete(peer)}
                          className="text-red-600 hover:text-red-800"
                        >
                          Delete
                        </button>
                      </div>
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

      {/* Peer Modals */}
      {selectedNetworkId && (
        <>
          <JumpPeerModal
            isOpen={isJumpModalOpen}
            onClose={handleModalClose}
            onSuccess={handleModalSuccess}
            networkId={selectedNetworkId}
            peer={editingPeer}
          />
          <RegularPeerModal
            isOpen={isRegularModalOpen}
            onClose={handleModalClose}
            onSuccess={handleModalSuccess}
            networkId={selectedNetworkId}
            peer={editingPeer}
          />
        </>
      )}
    </div>
  );
}
