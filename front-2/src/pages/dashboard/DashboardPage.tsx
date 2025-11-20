import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faNetworkWired, faDesktop, faChartBar, faShieldHalved, faCircleCheck } from '@fortawesome/free-solid-svg-icons';
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
        <Link to="/networks" className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6 hover:border-primary-300 dark:hover:border-primary-500 hover:shadow-md transition-all">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-gray-500 dark:text-gray-400">Networks</span>
            <span className="text-2xl">
              <FontAwesomeIcon icon={faNetworkWired} className="text-primary-600 dark:text-primary-400" />
            </span>
          </div>
          <div className="text-3xl font-bold text-gray-900 dark:text-white">{stats.networks.total}</div>
        </Link>

        {/* Peers */}
        <Link to="/peers" className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6 hover:border-primary-300 dark:hover:border-primary-500 hover:shadow-md transition-all">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-gray-500 dark:text-gray-400">Peers</span>
            <span className="text-2xl">
              <FontAwesomeIcon icon={faDesktop} className="text-primary-600 dark:text-primary-400" />
            </span>
          </div>
          <div className="text-3xl font-bold text-gray-900 dark:text-white">{stats.peers.total}</div>
        </Link>

        {/* IP Utilization */}
        <Link to="/ipam" className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6 hover:border-primary-300 dark:hover:border-primary-500 hover:shadow-md transition-all">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-gray-500 dark:text-gray-400">IP Utilization</span>
            <span className="text-2xl">
              <FontAwesomeIcon icon={faChartBar} className="text-primary-600 dark:text-primary-400" />
            </span>
          </div>
          <div className="text-3xl font-bold text-gray-900 dark:text-white">{stats.ipam.utilizationPercent}%</div>
          <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">
            {stats.ipam.allocated} / {stats.ipam.total} allocated
          </div>
        </Link>

        {/* Security */}
        <Link to="/security" className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6 hover:border-primary-300 dark:hover:border-primary-500 hover:shadow-md transition-all">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-gray-500 dark:text-gray-400">Security</span>
            <span className="text-2xl">
              <FontAwesomeIcon icon={faShieldHalved} className="text-primary-600 dark:text-primary-400" />
            </span>
          </div>
          <div className="text-3xl font-bold text-gray-900 dark:text-white">
            {stats.security.unresolved}
          </div>
          <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">
            {stats.security.unresolved > 0 ? 'unresolved incidents' : 'all clear'}
          </div>
        </Link>
      </div>

      {/* Two Column Layout */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
        {/* Peers Breakdown */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Peer Distribution</h2>
          
          <div className="space-y-3">
            <div>
              <div className="flex justify-between text-sm mb-1">
                <span className="text-gray-600 dark:text-gray-400">By Type</span>
              </div>
              <div className="space-y-2">
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-700 dark:text-gray-300">Jump Servers</span>
                  <span className="text-sm font-medium text-gray-900 dark:text-white">{stats.peers.byType.jump}</span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-700 dark:text-gray-300">Isolated</span>
                  <span className="text-sm font-medium text-gray-900 dark:text-white">{stats.peers.byType.isolated}</span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-700 dark:text-gray-300">Regular</span>
                  <span className="text-sm font-medium text-gray-900 dark:text-white">{stats.peers.byType.regular}</span>
                </div>
              </div>
            </div>

            <div className="pt-3 border-t border-gray-200 dark:border-gray-700">
              <div className="flex justify-between text-sm mb-1">
                <span className="text-gray-600 dark:text-gray-400">By Management</span>
              </div>
              <div className="space-y-2">
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-700 dark:text-gray-300">Agent-managed</span>
                  <span className="text-sm font-medium text-gray-900 dark:text-white">{stats.peers.byAgent.agent}</span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-700 dark:text-gray-300">Static config</span>
                  <span className="text-sm font-medium text-gray-900 dark:text-white">{stats.peers.byAgent.static}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Users Overview */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Users</h2>
            <Link to="/users" className="text-sm text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300">
              View all →
            </Link>
          </div>
          
          <div className="space-y-3">
            <div className="flex justify-between items-center">
              <span className="text-sm text-gray-700 dark:text-gray-300">Total Users</span>
              <span className="text-2xl font-bold text-gray-900 dark:text-white">{stats.users.total}</span>
            </div>
            <div className="flex justify-between items-center pt-3 border-t border-gray-200 dark:border-gray-700">
              <span className="text-sm text-gray-700 dark:text-gray-300">Administrators</span>
              <span className="text-sm font-medium text-gray-900 dark:text-white">{stats.users.administrators}</span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-sm text-gray-700 dark:text-gray-300">Regular Users</span>
              <span className="text-sm font-medium text-gray-900 dark:text-white">{stats.users.regularUsers}</span>
            </div>
          </div>
        </div>
      </div>

      {/* Recent Networks & Security Incidents */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Recent Networks */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Recent Networks</h2>
            <Link to="/networks" className="text-sm text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300">
              View all →
            </Link>
          </div>
          
          {stats.networks.recentNetworks.length === 0 ? (
            <p className="text-sm text-gray-500 dark:text-gray-400 text-center py-8">No networks yet</p>
          ) : (
            <div className="space-y-3">
              {stats.networks.recentNetworks.map(network => (
                <Link
                  key={network.id}
                  to={`/networks/${network.id}`}
                  className="block p-3 rounded-lg border border-gray-200 dark:border-gray-600 hover:border-primary-300 dark:hover:border-primary-500 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
                >
                  <div className="flex justify-between items-start">
                    <div className="flex-1">
                      <div className="font-medium text-gray-900 dark:text-white">{network.name}</div>
                      <div className="text-sm text-gray-500 dark:text-gray-400 font-mono">{network.cidr}</div>
                    </div>
                    <div className="text-sm text-gray-500 dark:text-gray-400">
                      {network.peer_count || 0} peers
                    </div>
                  </div>
                </Link>
              ))}
            </div>
          )}
        </div>

        {/* Recent Security Incidents */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Recent Security Incidents</h2>
            <Link to="/security" className="text-sm text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300">
              View all →
            </Link>
          </div>
          
          {stats.security.recentIncidents.length === 0 ? (
            <div className="text-center py-8">
              <div className="text-4xl mb-2">
                <FontAwesomeIcon icon={faCircleCheck} className="text-green-500" />
              </div>
              <p className="text-sm text-gray-500 dark:text-gray-400">No security incidents</p>
            </div>
          ) : (
            <div className="space-y-3">
              {stats.security.recentIncidents.map(incident => (
                <div
                  key={incident.id}
                  className="p-3 rounded-lg border border-gray-200 dark:border-gray-600"
                >
                  <div className="flex items-start justify-between mb-2">
                    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                      incident.incident_type === 'shared_config' ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200' :
                      incident.incident_type === 'session_conflict' ? 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200' :
                      'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
                    }`}>
                      {incident.incident_type.replace('_', ' ')}
                    </span>
                    {!incident.resolved && (
                      <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200">
                        Active
                      </span>
                    )}
                  </div>
                  <div className="text-sm text-gray-900 dark:text-white">{incident.peer_name}</div>
                  <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">
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
