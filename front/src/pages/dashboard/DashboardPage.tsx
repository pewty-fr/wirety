import type { ComponentType, ReactNode, SVGProps } from 'react';
import { Link } from 'react-router-dom';
import { useQueries, useQuery } from '@tanstack/react-query';
import { ChartBarIcon, ComputerDesktopIcon, GlobeAltIcon } from '@heroicons/react/24/outline';
import api from '../../api/client';
import { Alert, Card, CardHeader, Page, PageHeader, Spinner, apiErrorMessage } from '../../components/ui';
import { useAuth } from '../../contexts/AuthContext';
import { queryKeys, useUsers } from '../../hooks/useQueries';
import type { Network } from '../../types';
import { computeCapacityFromCIDR, formatCapacity } from '../../utils/networkCapacity';

const plural = (n: number, word: string) => `${n.toLocaleString()} ${word}${n === 1 ? '' : 's'}`;

function StatCard({
  to,
  label,
  value,
  detail,
  icon: Icon,
}: {
  to: string;
  label: string;
  value: ReactNode;
  detail?: ReactNode;
  icon: ComponentType<SVGProps<SVGSVGElement>>;
}) {
  return (
    <Link
      to={to}
      className="group block rounded-panel border border-line bg-surface p-5 shadow-panel transition-colors hover:border-line-strong focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500"
    >
      <div className="flex items-center justify-between gap-4">
        <span className="text-sm font-medium text-ink-muted">{label}</span>
        <Icon className="size-5 text-ink-subtle transition-colors group-hover:text-primary-500" aria-hidden="true" />
      </div>
      <p className="mt-3 text-2xl font-semibold tracking-tight text-ink tabular-nums">{value}</p>
      {detail && <p className="mt-1 text-xs text-ink-muted">{detail}</p>}
    </Link>
  );
}

/** Rows of counts, each with a bar showing its share of `total`. */
function Breakdown({ title, total, rows }: { title: string; total: number; rows: { label: string; value: number }[] }) {
  return (
    <div>
      <h3 className="text-xs font-medium text-ink-muted">{title}</h3>
      <dl className="mt-3 space-y-3">
        {rows.map((r) => (
          <div key={r.label}>
            <div className="flex items-center justify-between text-sm">
              <dt className="text-ink-secondary">{r.label}</dt>
              <dd className="font-medium text-ink tabular-nums">{r.value.toLocaleString()}</dd>
            </div>
            <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-surface-muted" aria-hidden="true">
              <div
                className="h-full rounded-full bg-primary-500"
                style={{ width: `${total ? Math.round((r.value / total) * 100) : 0}%` }}
              />
            </div>
          </div>
        ))}
      </dl>
    </div>
  );
}

function ViewAll({ to, what }: { to: string; what: string }) {
  return (
    <Link
      to={to}
      aria-label={`View all ${what}`}
      className="text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
    >
      View all
    </Link>
  );
}

const byNewest = (a: Network, b: Network) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime();

export default function DashboardPage() {
  const { user } = useAuth();
  const isAdmin = user?.role === 'administrator';

  const networksQuery = useQuery({
    queryKey: ['networks', 'dashboard'],
    queryFn: () => api.getNetworks(1, 100),
  });
  const networks = networksQuery.data?.data ?? [];
  // The same queries as the topology and the peer pickers: they share the cache.
  const peerQueries = useQueries({
    queries: networks.map((n) => ({
      queryKey: queryKeys.networkPeers(n.id),
      queryFn: () => api.getAllNetworkPeers(n.id),
    })),
  });
  const { data: users = [] } = useUsers(isAdmin);

  const header = <PageHeader title="Dashboard" description="Overview of your WireGuard infrastructure." />;

  if (networksQuery.isLoading) {
    return (
      <Page>
        {header}
        <div className="flex justify-center py-16 text-ink-muted" role="status">
          <Spinner className="size-5" />
          <span className="sr-only">Loading the dashboard…</span>
        </div>
      </Page>
    );
  }

  if (networksQuery.isError) {
    return (
      <Page>
        {header}
        <Alert tone="danger" title="The dashboard could not be loaded">
          {apiErrorMessage(networksQuery.error)}
        </Alert>
      </Page>
    );
  }

  const peers = peerQueries.flatMap((q) => q.data ?? []);
  const peerTotal = networks.reduce((sum, n) => sum + (n.peer_count ?? 0), 0);
  const jump = peers.filter((p) => p.is_jump).length;
  const agent = peers.filter((p) => p.use_agent).length;

  // IPv4 capacity of every network (IPv6 ranges are not limiting).
  let capacity = 0;
  let used = 0;
  for (const n of networks) {
    const c = computeCapacityFromCIDR(n.cidr ?? '');
    if (c === null) continue;
    capacity += c;
    used += n.peer_count ?? 0;
  }
  const utilization = capacity > 0 ? Math.round((used / capacity) * 100) : 0;

  const admins = users.filter((u) => u.role === 'administrator').length;
  const recent = [...networks].sort(byNewest).slice(0, 5);

  return (
    <Page>
      {header}

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard to="/networks" label="Networks" value={(networksQuery.data?.total ?? 0).toLocaleString()} icon={GlobeAltIcon} />
        <StatCard
          to="/peers"
          label="Peers"
          value={peerTotal.toLocaleString()}
          detail={`${plural(jump, 'jump server')} · ${agent.toLocaleString()} with the agent`}
          icon={ComputerDesktopIcon}
        />
        <StatCard
          to="/ipam"
          label="IP utilization"
          value={`${utilization}%`}
          detail={`${used.toLocaleString()} of ${formatCapacity(capacity)} IPv4 addresses used`}
          icon={ChartBarIcon}
        />
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader title="Peer distribution" description={plural(peers.length, 'peer')} />
          <div className="grid gap-6 px-5 py-5 sm:grid-cols-2">
            <Breakdown
              title="By type"
              total={peers.length}
              rows={[
                { label: 'Jump servers', value: jump },
                { label: 'Regular', value: peers.length - jump },
              ]}
            />
            <Breakdown
              title="By management"
              total={peers.length}
              rows={[
                { label: 'Agent-managed', value: agent },
                { label: 'Static config', value: peers.length - agent },
              ]}
            />
          </div>
        </Card>

        {isAdmin && (
          <Card>
            <CardHeader title="Users" actions={<ViewAll to="/users" what="users" />} />
            <dl className="divide-y divide-line px-5">
              {[
                { label: 'Total users', value: users.length },
                { label: 'Administrators', value: admins },
                { label: 'Regular users', value: users.length - admins },
              ].map((r) => (
                <div key={r.label} className="flex items-center justify-between py-3 text-sm">
                  <dt className="text-ink-secondary">{r.label}</dt>
                  <dd className="font-medium text-ink tabular-nums">{r.value.toLocaleString()}</dd>
                </div>
              ))}
            </dl>
          </Card>
        )}

        <Card className={isAdmin ? 'lg:col-span-2' : undefined}>
          <CardHeader title="Recent networks" actions={<ViewAll to="/networks" what="networks" />} />
          {recent.length === 0 ? (
            <p className="px-5 py-8 text-center text-sm text-ink-muted">No network yet.</p>
          ) : (
            <ul aria-label="Recent networks" className="divide-y divide-line">
              {recent.map((n) => (
                <li key={n.id}>
                  <Link
                    to="/networks"
                    className="flex items-center justify-between gap-4 px-5 py-3 transition-colors hover:bg-surface-hover/60 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary-500"
                  >
                    <div className="min-w-0">
                      <div className="truncate text-sm font-medium text-ink">{n.name}</div>
                      <div className="truncate font-mono text-xs text-ink-muted">
                        {[n.cidr, n.cidr_v6].filter(Boolean).join(' · ')}
                      </div>
                    </div>
                    <span className="shrink-0 text-sm text-ink-muted tabular-nums">{plural(n.peer_count ?? 0, 'peer')}</span>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </Card>
      </div>
    </Page>
  );
}
