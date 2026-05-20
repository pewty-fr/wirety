import { useState, useEffect, useCallback, useMemo } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faGlobe, faPencil, faTrash, faCheck, faTimes } from '@fortawesome/free-solid-svg-icons';
import PageHeader from '../../components/PageHeader';
import SearchableSelect from '../../components/SearchableSelect';
import api from '../../api/client';
import type { Network, Route, DNSMapping } from '../../types';

interface DNSRow extends DNSMapping {
  route_name?: string;
  route_destination_cidr?: string;
  // The selected network's identity is the source of truth for the FQDN; the
  // network's name + domain_suffix are captured here when the rows are loaded
  // so the table can render the resolved name without re-querying.
  network_name?: string;
  network_domain_suffix?: string;
}

// sanitizeDNSLabel mirrors server-side sanitizeDNSLabel:
// lowercase + replace any non-alphanumeric / non-hyphen char with '-'.
// Used so the FQDN we display in the UI matches exactly what the agent's
// DNS server actually serves.
function sanitizeDNSLabel(s: string): string {
  if (!s) return 'peer';
  const out = s
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, '-');
  return out || 'peer';
}

// buildFQDN constructs the full DNS name a peer would query to resolve this
// record: <name>.<network-name>.<network-domain-suffix or "internal">.
// The route the record belongs to intentionally does NOT appear — routes are
// an internal grouping concept and renaming a route must not affect the
// resolved name.
function buildFQDN(row: DNSRow): string {
  const label = sanitizeDNSLabel(row.name);
  const network = sanitizeDNSLabel(row.network_name || '');
  const suffix = (row.network_domain_suffix && row.network_domain_suffix.trim()) || 'internal';
  if (!network) return `${label}.${suffix}`;
  return `${label}.${network}.${suffix}`;
}

export default function DNSRecordsPage() {
  const [networks, setNetworks] = useState<Network[]>([]);
  const [selectedNetworkId, setSelectedNetworkId] = useState<string>('');
  const [routes, setRoutes] = useState<Route[]>([]);
  const [records, setRecords] = useState<DNSRow[]>([]);
  const [loading, setLoading] = useState(false);

  // Inline edit state — only one row at a time
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState('');
  const [editIP, setEditIP] = useState('');
  const [editIPv6, setEditIPv6] = useState('');
  const [savingId, setSavingId] = useState<string | null>(null);

  // New record form — both ip_address and ip_address_v6 are independent.
  // At least one must be provided; the route must carry the matching family
  // CIDR (an IPv6 address on an IPv4-only route is rejected server-side).
  const [showNewForm, setShowNewForm] = useState(false);
  const [newRouteId, setNewRouteId] = useState('');
  const [newName, setNewName] = useState('');
  const [newIP, setNewIP] = useState('');
  const [newIPv6, setNewIPv6] = useState('');
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');

  // Load networks once
  useEffect(() => {
    void api.getNetworks(1, 200).then(r => {
      const list = r.data ?? [];
      setNetworks(list);
      if (list.length > 0 && !selectedNetworkId) {
        setSelectedNetworkId(list[0].id);
      }
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const loadData = useCallback(async (networkId: string, networkList: Network[]) => {
    if (!networkId) return;
    setLoading(true);
    try {
      // Load routes first
      const routesRes = await api.getRoutes(networkId).catch(() => [] as Route[]);
      setRoutes(routesRes);

      // Look up the selected network's name + domain_suffix.  These drive the
      // FQDN in the UI (the route's name/suffix is no longer part of it).
      const selectedNetwork = networkList.find(n => n.id === networkId);
      const networkName = selectedNetwork?.name;
      const networkDomainSuffix = selectedNetwork?.domain_suffix;

      // For each route, load its DNS mappings (which have real `id` and `route_id`)
      const routeById = new Map(routesRes.map(r => [r.id, r]));
      const perRouteResults = await Promise.all(
        routesRes.map(r =>
          api.getDNSMappings(networkId, r.id).catch(() => [] as DNSMapping[])
        )
      );
      const allMappings: DNSRow[] = perRouteResults.flat().map(m => ({
        ...m,
        route_name: routeById.get(m.route_id)?.name,
        route_destination_cidr: routeById.get(m.route_id)?.destination_cidr,
        network_name: networkName,
        network_domain_suffix: networkDomainSuffix,
      }));
      setRecords(allMappings);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (selectedNetworkId) void loadData(selectedNetworkId, networks);
  }, [selectedNetworkId, networks, loadData]);

  const startEdit = (row: DNSRow) => {
    setEditingId(row.id);
    setEditName(row.name);
    setEditIP(row.ip_address || '');
    setEditIPv6(row.ip_address_v6 || '');
  };

  const cancelEdit = () => {
    setEditingId(null);
    setEditName('');
    setEditIP('');
    setEditIPv6('');
  };

  const saveEdit = async (row: DNSRow) => {
    if (!selectedNetworkId) return;
    if (!editIP.trim() && !editIPv6.trim()) {
      alert('A DNS record must have at least one address (IPv4 or IPv6).');
      return;
    }
    setSavingId(row.id);
    try {
      await api.updateDNSMapping(selectedNetworkId, row.route_id, row.id, {
        name: editName,
        ip_address: editIP.trim() || undefined,
        ip_address_v6: editIPv6.trim() || undefined,
      });
      await loadData(selectedNetworkId, networks);
      cancelEdit();
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } }; message?: string };
      alert(e?.response?.data?.error || e?.message || 'Failed to update DNS record');
    } finally {
      setSavingId(null);
    }
  };

  const deleteRecord = async (row: DNSRow) => {
    if (!selectedNetworkId) return;
    const addrSummary = [row.ip_address, row.ip_address_v6].filter(Boolean).join(' / ');
    if (!window.confirm(`Delete DNS record "${buildFQDN(row)}" → ${addrSummary || '(no address)'}?`)) return;
    try {
      await api.deleteDNSMapping(selectedNetworkId, row.route_id, row.id);
      await loadData(selectedNetworkId, networks);
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } }; message?: string };
      alert(e?.response?.data?.error || e?.message || 'Failed to delete DNS record');
    }
  };

  const submitNew = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreateError('');
    if (!selectedNetworkId || !newRouteId || !newName.trim()) {
      setCreateError('Network, route and name are required');
      return;
    }
    if (!newIP.trim() && !newIPv6.trim()) {
      setCreateError('Provide at least one address (IPv4 or IPv6).');
      return;
    }
    setCreating(true);
    try {
      await api.createDNSMapping(selectedNetworkId, newRouteId, {
        name: newName.trim(),
        ip_address: newIP.trim() || undefined,
        ip_address_v6: newIPv6.trim() || undefined,
      });
      setShowNewForm(false);
      setNewRouteId('');
      setNewName('');
      setNewIP('');
      setNewIPv6('');
      await loadData(selectedNetworkId, networks);
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } }; message?: string };
      setCreateError(e?.response?.data?.error || e?.message || 'Failed to create DNS record');
    } finally {
      setCreating(false);
    }
  };

  const networkOptions = useMemo(() => networks.map(n => ({ value: n.id, label: `${n.name} (${n.cidr})` })), [networks]);
  const routeOptions = useMemo(
    () =>
      routes.map(r => {
        // Dual-stack routes carry both CIDRs; show whichever are set.
        const cidrs = [r.destination_cidr, r.destination_cidr_v6].filter(Boolean).join(' / ');
        return { value: r.id, label: cidrs ? `${r.name} — ${cidrs}` : r.name };
      }),
    [routes]
  );

  return (
    <div>
      <PageHeader
        title="DNS Records"
        subtitle={`${records.length} record${records.length === 1 ? '' : 's'}${selectedNetworkId ? '' : ' — pick a network to start'}`}
        action={
          <button
            onClick={() => setShowNewForm(s => !s)}
            disabled={!selectedNetworkId || routes.length === 0}
            className="px-4 py-2.5 bg-gradient-to-r from-primary-600 to-accent-blue text-white rounded-xl hover:scale-105 active:scale-95 shadow-lg hover:shadow-xl flex items-center gap-2 cursor-pointer transition-all font-semibold disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
          </button>
        }
      />

      <div className="p-4 sm:p-8">
        {/* Network Filter Block */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 mb-6">
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Network</label>
              <SearchableSelect
                value={selectedNetworkId}
                onChange={(v) => setSelectedNetworkId(v)}
                options={networkOptions}
                placeholder="Select a network..."
              />
            </div>
          </div>
        </div>

        {showNewForm && (
          <form onSubmit={submitNew} className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4 mb-6">
            <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-3">New DNS record</h3>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
              <div>
                <label className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">Route</label>
                <SearchableSelect
                  value={newRouteId}
                  onChange={(v) => setNewRouteId(v)}
                  options={routeOptions}
                  placeholder="Select a route"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">Name (hostname label)</label>
                <input
                  type="text"
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  placeholder="e.g. nas"
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
                />
                {newRouteId && newName.trim() && (() => {
                  // The FQDN is anchored on the network's name + domain
                  // suffix; the chosen route only determines the routing
                  // CIDR, not the resolved name.
                  const route = routes.find(r => r.id === newRouteId);
                  if (!route) return null;
                  const net = networks.find(n => n.id === selectedNetworkId);
                  if (!net) return null;
                  const fqdn = `${sanitizeDNSLabel(newName)}.${sanitizeDNSLabel(net.name)}.${(net.domain_suffix && net.domain_suffix.trim()) || 'internal'}`;
                  return (
                    <div className="text-xs text-gray-500 dark:text-gray-400 font-mono mt-1">
                      → resolves as <span className="text-gray-700 dark:text-gray-200">{fqdn}</span>
                    </div>
                  );
                })()}
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
                  IP address <span className="text-gray-400 font-normal">(at least one of v4 / v6)</span>
                </label>
                <div className="flex flex-col gap-2">
                  <input
                    type="text"
                    value={newIP}
                    onChange={(e) => setNewIP(e.target.value)}
                    placeholder="IPv4 — e.g. 10.0.0.50"
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary-500"
                  />
                  <input
                    type="text"
                    value={newIPv6}
                    onChange={(e) => setNewIPv6(e.target.value)}
                    placeholder="IPv6 — e.g. fd00::50"
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary-500"
                  />
                </div>
              </div>
            </div>
            {createError && <p className="text-sm text-red-500 mt-2">{createError}</p>}
            <div className="flex justify-end gap-2 mt-3">
              <button type="button" onClick={() => setShowNewForm(false)} className="px-3 py-1.5 text-sm text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600">
                Cancel
              </button>
              <button type="submit" disabled={creating} className="px-3 py-1.5 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50">
                {creating ? 'Creating...' : 'Create'}
              </button>
            </div>
          </form>
        )}

        {loading ? (
          <div className="flex flex-col items-center justify-center py-16">
            <div className="inline-block animate-spin rounded-full h-12 w-12 border-4 border-solid border-current border-r-transparent text-primary-600" />
            <p className="text-gray-600 dark:text-gray-300 mt-4">Loading DNS records...</p>
          </div>
        ) : records.length === 0 ? (
          <div className="bg-gradient-to-br from-white to-gray-50 dark:from-gray-800 dark:to-gray-800 rounded-2xl border border-gray-200 dark:border-gray-700 p-16 text-center shadow-sm">
            <div className="inline-flex items-center justify-center w-20 h-20 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-blue mb-6">
              <FontAwesomeIcon icon={faGlobe} className="text-3xl text-white" />
            </div>
            <h3 className="text-xl font-bold text-gray-900 dark:text-gray-100 mb-2">No DNS records</h3>
            <p className="text-gray-600 dark:text-gray-300">
              {!selectedNetworkId
                ? 'Select a network to view DNS records.'
                : routes.length === 0
                ? 'Create a route first, then add DNS records that resolve inside the VPN.'
                : 'Click "+ Record" to create your first DNS record.'}
            </p>
          </div>
        ) : (
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead className="bg-gray-50 dark:bg-gray-700">
                <tr>
                  <th className="px-3 sm:px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Name</th>
                  <th className="px-3 sm:px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">IP Address</th>
                  <th className="px-3 sm:px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Route</th>
                  <th className="px-3 sm:px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Actions</th>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                {records.map(row => {
                  const editing = editingId === row.id;
                  const fqdn = buildFQDN(row);
                  return (
                    <tr key={row.id} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                      <td className="px-3 sm:px-6 py-4 whitespace-nowrap">
                        {editing ? (
                          <div>
                            <input
                              autoFocus
                              type="text"
                              value={editName}
                              onChange={(e) => setEditName(e.target.value)}
                              className="w-full px-2 py-1 border border-primary-500 rounded text-sm bg-white dark:bg-gray-800"
                              placeholder="hostname label only"
                            />
                            <div className="text-xs text-gray-400 dark:text-gray-500 font-mono mt-1">
                              FQDN: {sanitizeDNSLabel(editName || row.name)}.{sanitizeDNSLabel(row.network_name || '')}.{(row.network_domain_suffix && row.network_domain_suffix.trim()) || 'internal'}
                            </div>
                          </div>
                        ) : (
                          <div>
                            <div className="text-sm font-mono font-medium text-gray-900 dark:text-gray-100">{fqdn}</div>
                            {row.name !== fqdn && (
                              <div className="text-xs text-gray-400 dark:text-gray-500">
                                label: {row.name}
                              </div>
                            )}
                          </div>
                        )}
                      </td>
                      <td className="px-3 sm:px-6 py-4 whitespace-nowrap font-mono text-sm">
                        {editing ? (
                          <div className="flex flex-col gap-1">
                            <input
                              type="text"
                              value={editIP}
                              onChange={(e) => setEditIP(e.target.value)}
                              placeholder="IPv4"
                              className="w-full px-2 py-1 border border-primary-500 rounded text-sm bg-white dark:bg-gray-800 font-mono"
                            />
                            <input
                              type="text"
                              value={editIPv6}
                              onChange={(e) => setEditIPv6(e.target.value)}
                              placeholder="IPv6"
                              className="w-full px-2 py-1 border border-primary-500 rounded text-sm bg-white dark:bg-gray-800 font-mono"
                            />
                          </div>
                        ) : (
                          <div className="flex flex-col">
                            {row.ip_address && <span className="text-gray-900 dark:text-gray-100">{row.ip_address}</span>}
                            {row.ip_address_v6 && (
                              <span className="text-gray-500 dark:text-gray-400 text-xs">{row.ip_address_v6}</span>
                            )}
                          </div>
                        )}
                      </td>
                      <td className="px-3 sm:px-6 py-4 whitespace-nowrap text-sm text-gray-600 dark:text-gray-300">
                        {row.route_name || row.route_id}
                        {row.route_destination_cidr && (
                          <span className="ml-2 text-xs font-mono text-gray-400">{row.route_destination_cidr}</span>
                        )}
                      </td>
                      <td className="px-3 sm:px-6 py-4 whitespace-nowrap text-right">
                        {editing ? (
                          <div className="flex justify-end gap-2">
                            <button
                              onClick={() => saveEdit(row)}
                              disabled={savingId === row.id}
                              className="text-green-600 dark:text-green-400 hover:text-green-800 disabled:opacity-50"
                              title="Save"
                            >
                              <FontAwesomeIcon icon={faCheck} />
                            </button>
                            <button
                              onClick={cancelEdit}
                              disabled={savingId === row.id}
                              className="text-gray-500 hover:text-gray-700 disabled:opacity-50"
                              title="Cancel"
                            >
                              <FontAwesomeIcon icon={faTimes} />
                            </button>
                          </div>
                        ) : (
                          <div className="flex justify-end gap-3">
                            <button
                              onClick={() => startEdit(row)}
                              className="text-primary-600 dark:text-primary-400 hover:text-primary-800"
                              title="Edit"
                            >
                              <FontAwesomeIcon icon={faPencil} />
                            </button>
                            <button
                              onClick={() => deleteRecord(row)}
                              className="text-red-600 dark:text-red-400 hover:text-red-800"
                              title="Delete"
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
      </div>
    </div>
  );
}
