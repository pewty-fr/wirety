import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import api from '../../api/client';
import type { Network, SecurityIncident } from '../../types';
import { computeCapacityFromCIDR } from '../../utils/networkCapacity';

interface DashboardStats {
  networks: {
    total: number;
    recentNetworks: Network[];
  };
  peers: {
    total: number;
    byType: {
      jump: number;
      isolated: number;
      regular: number;
    };
    byAgent: {
      agent: number;
      static: number;
    };
  };
  ipam: {
    total: number;
    allocated: number;
    available: number;
    utilizationPercent: number;
  };
  security: {
    total: number;
    unresolved: number;
    recentIncidents: SecurityIncident[];
  };
  users: {
    total: number;
    administrators: number;
    regularUsers: number;
  };
}

export default function DashboardPage() {
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadDashboardData();
  }, []);

  const loadDashboardData = async () => {
    setLoading(true);
    try {
      const [networksRes, peersRes, _ipamRes, securityRes, usersRes] = await Promise.all([
        api.getNetworks(1, 5).catch(() => null),
        api.getAllPeers(1, 1000).catch(() => null),
        api.getIPAMAllocations(1, 1000).catch(() => null),
        api.getSecurityIncidents(1, 5).catch(() => null),
        api.getUsers(1, 100).catch(() => null),
      ]);

      const peers = peersRes?.peers || [];
      const securityData = securityRes?.data || [];
      const usersData = usersRes?.data || [];
      const networks = networksRes?.data || [];

      // Calculate total capacity and used slots across all networks
      let totalCapacity = 0;
      let totalUsedSlots = 0;
      
      networks.forEach(network => {
        const capacity = computeCapacityFromCIDR(network.cidr);
        if (capacity !== null) {
          totalCapacity += capacity;
          totalUsedSlots += network.peer_count || 0;
        }
      });

      const dashboardStats: DashboardStats = {
        networks: {
          total: networksRes?.total || 0,
          recentNetworks: networks,
        },
        peers: {
          total: peersRes?.total || 0,
          byType: {
            jump: peers.filter(p => p.is_jump).length,
            isolated: peers.filter(p => !p.is_jump && p.is_isolated).length,
            regular: peers.filter(p => !p.is_jump && !p.is_isolated).length,
          },
          byAgent: {
            agent: peers.filter(p => p.use_agent).length,
            static: peers.filter(p => !p.use_agent).length,
          },
        },
        ipam: {
          total: totalCapacity,
          allocated: totalUsedSlots,
          available: totalCapacity - totalUsedSlots,
          utilizationPercent: totalCapacity > 0 
            ? Math.round((totalUsedSlots / totalCapacity) * 100)
            : 0,
        },
        security: {
          total: securityRes?.total || 0,
          unresolved: securityData.filter(i => !i.resolved).length,
          recentIncidents: securityData.slice(0, 5),
        },
        users: {
          total: usersRes?.total || 0,
          administrators: usersData.filter(u => u.role === 'administrator').length,
          regularUsers: usersData.filter(u => u.role === 'user').length,
        },
      };

      setStats(dashboardStats);
    } catch (error) {
      console.error('Failed to load dashboard data:', error);
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-gray-500">Loading dashboard...</div>
      </div>
    );
  }

  if (!stats) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-gray-500">Failed to load dashboard data</div>
      </div>
    );
  }

  return (
    <div className="p-8">
      {/* Header */}
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900 mb-2">Dashboard</h1>
        <p className="text-gray-500">Overview of your WireGuard infrastructure</p>
      </div>

      {/* Main Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
        {/* Networks */}
        <Link to="/networks" className="bg-white rounded-lg border border-gray-200 p-6 hover:border-primary-300 hover:shadow-md transition-all">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-gray-500">Networks</span>
            <span className="text-2xl">🌐</span>
          </div>
          <div className="text-3xl font-bold text-gray-900">{stats.networks.total}</div>
        </Link>

        {/* Peers */}
        <Link to="/peers" className="bg-white rounded-lg border border-gray-200 p-6 hover:border-primary-300 hover:shadow-md transition-all">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-gray-500">Peers</span>
            <span className="text-2xl">💻</span>
          </div>
          <div className="text-3xl font-bold text-gray-900">{stats.peers.total}</div>
        </Link>

        {/* IP Utilization */}
        <Link to="/ipam" className="bg-white rounded-lg border border-gray-200 p-6 hover:border-primary-300 hover:shadow-md transition-all">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-gray-500">IP Utilization</span>
            <span className="text-2xl">📊</span>
          </div>
          <div className="text-3xl font-bold text-gray-900">{stats.ipam.utilizationPercent}%</div>
          <div className="text-xs text-gray-500 mt-1">
            {stats.ipam.allocated} / {stats.ipam.total} allocated
          </div>
        </Link>

        {/* Security */}
        <Link to="/security" className="bg-white rounded-lg border border-gray-200 p-6 hover:border-primary-300 hover:shadow-md transition-all">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-gray-500">Security</span>
            <span className="text-2xl">🔒</span>
          </div>
          <div className="text-3xl font-bold text-gray-900">
            {stats.security.unresolved}
          </div>
          <div className="text-xs text-gray-500 mt-1">
            {stats.security.unresolved > 0 ? 'unresolved incidents' : 'all clear'}
          </div>
        </Link>
      </div>

      {/* Two Column Layout */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
        {/* Peers Breakdown */}
        <div className="bg-white rounded-lg border border-gray-200 p-6">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">Peer Distribution</h2>
          
          <div className="space-y-3">
            <div>
              <div className="flex justify-between text-sm mb-1">
                <span className="text-gray-600">By Type</span>
              </div>
              <div className="space-y-2">
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-700">Jump Servers</span>
                  <span className="text-sm font-medium text-gray-900">{stats.peers.byType.jump}</span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-700">Isolated</span>
                  <span className="text-sm font-medium text-gray-900">{stats.peers.byType.isolated}</span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-700">Regular</span>
                  <span className="text-sm font-medium text-gray-900">{stats.peers.byType.regular}</span>
                </div>
              </div>
            </div>

            <div className="pt-3 border-t border-gray-200">
              <div className="flex justify-between text-sm mb-1">
                <span className="text-gray-600">By Management</span>
              </div>
              <div className="space-y-2">
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-700">Agent-managed</span>
                  <span className="text-sm font-medium text-gray-900">{stats.peers.byAgent.agent}</span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-700">Static config</span>
                  <span className="text-sm font-medium text-gray-900">{stats.peers.byAgent.static}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Users Overview */}
        <div className="bg-white rounded-lg border border-gray-200 p-6">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold text-gray-900">Users</h2>
            <Link to="/users" className="text-sm text-primary-600 hover:text-primary-700">
              View all →
            </Link>
          </div>
          
          <div className="space-y-3">
            <div className="flex justify-between items-center">
              <span className="text-sm text-gray-700">Total Users</span>
              <span className="text-2xl font-bold text-gray-900">{stats.users.total}</span>
            </div>
            <div className="flex justify-between items-center pt-3 border-t border-gray-200">
              <span className="text-sm text-gray-700">Administrators</span>
              <span className="text-sm font-medium text-gray-900">{stats.users.administrators}</span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-sm text-gray-700">Regular Users</span>
              <span className="text-sm font-medium text-gray-900">{stats.users.regularUsers}</span>
            </div>
          </div>
        </div>
      </div>

      {/* Recent Networks & Security Incidents */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Recent Networks */}
        <div className="bg-white rounded-lg border border-gray-200 p-6">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold text-gray-900">Recent Networks</h2>
            <Link to="/networks" className="text-sm text-primary-600 hover:text-primary-700">
              View all →
            </Link>
          </div>
          
          {stats.networks.recentNetworks.length === 0 ? (
            <p className="text-sm text-gray-500 text-center py-8">No networks yet</p>
          ) : (
            <div className="space-y-3">
              {stats.networks.recentNetworks.map(network => (
                <Link
                  key={network.id}
                  to={`/networks/${network.id}`}
                  className="block p-3 rounded-lg border border-gray-200 hover:border-primary-300 hover:bg-gray-50 transition-colors"
                >
                  <div className="flex justify-between items-start">
                    <div className="flex-1">
                      <div className="font-medium text-gray-900">{network.name}</div>
                      <div className="text-sm text-gray-500 font-mono">{network.cidr}</div>
                    </div>
                    <div className="text-sm text-gray-500">
                      {network.peer_count || 0} peers
                    </div>
                  </div>
                </Link>
              ))}
            </div>
          )}
        </div>

        {/* Recent Security Incidents */}
        <div className="bg-white rounded-lg border border-gray-200 p-6">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold text-gray-900">Recent Security Incidents</h2>
            <Link to="/security" className="text-sm text-primary-600 hover:text-primary-700">
              View all →
            </Link>
          </div>
          
          {stats.security.recentIncidents.length === 0 ? (
            <div className="text-center py-8">
              <div className="text-4xl mb-2">✅</div>
              <p className="text-sm text-gray-500">No security incidents</p>
            </div>
          ) : (
            <div className="space-y-3">
              {stats.security.recentIncidents.map(incident => (
                <div
                  key={incident.id}
                  className="p-3 rounded-lg border border-gray-200"
                >
                  <div className="flex items-start justify-between mb-2">
                    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                      incident.incident_type === 'shared_config' ? 'bg-red-100 text-red-800' :
                      incident.incident_type === 'session_conflict' ? 'bg-orange-100 text-orange-800' :
                      'bg-yellow-100 text-yellow-800'
                    }`}>
                      {incident.incident_type.replace('_', ' ')}
                    </span>
                    {!incident.resolved && (
                      <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-800">
                        Active
                      </span>
                    )}
                  </div>
                  <div className="text-sm text-gray-900">{incident.peer_name}</div>
                  <div className="text-xs text-gray-500 mt-1">
                    {new Date(incident.detected_at).toLocaleDateString()}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
