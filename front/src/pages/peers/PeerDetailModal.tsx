import { useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ArrowDownTrayIcon,
  ArrowPathIcon,
  CheckCircleIcon,
  ClipboardDocumentIcon,
  PencilSquareIcon,
  TrashIcon,
  XCircleIcon,
} from '@heroicons/react/20/solid';
import api from '../../api/client';
import CaptivePortalHistory from '../../components/CaptivePortalHistory';
import {
  Alert,
  Badge,
  Button,
  Modal,
  ModalBody,
  ModalFooter,
  Spinner,
  Tabs,
  apiErrorMessage,
  cx,
  useConfirm,
  useToast,
} from '../../components/ui';
import { useAuth } from '../../contexts/AuthContext';
import { useGroups, useNetwork, usePeer, usePolicies, useRoutes, useUsers } from '../../hooks/useQueries';
import type { Network, Peer, PeerAccess, PolicyRule } from '../../types';
import JumpPeerModal from './JumpPeerModal';
import { agentStatus, peerStatus, portalStatus, refreshPeers } from './peerStatus';
import type { PeerStatus } from './peerStatus';
import RegularPeerModal from './RegularPeerModal';

function StatusBadge({ status }: { status: PeerStatus }) {
  return (
    <Badge tone={status.tone} dot className={cx(status.pending && '[&>span]:animate-pulse')}>
      {status.label}
    </Badge>
  );
}

function Section({ title, aside, children }: { title: ReactNode; aside?: ReactNode; children: ReactNode }) {
  return (
    <section className="space-y-2.5">
      <div className="flex items-baseline justify-between gap-3">
        <h3 className="text-sm font-semibold text-ink">{title}</h3>
        {aside && <span className="text-xs text-ink-muted">{aside}</span>}
      </div>
      {children}
    </section>
  );
}

/** Label/value rows; a null value hides its row. */
function Details({ items, columns = 2 }: { items: [string, ReactNode][]; columns?: 1 | 2 }) {
  return (
    <dl className={cx('grid gap-x-6 gap-y-3', columns === 2 && 'sm:grid-cols-2')}>
      {items
        .filter(([, value]) => value !== null && value !== undefined && value !== '')
        .map(([label, value]) => (
          <div key={label} className="min-w-0">
            <dt className="text-xs font-medium text-ink-muted">{label}</dt>
            <dd className="mt-0.5 text-sm break-all text-ink">{value}</dd>
          </div>
        ))}
    </dl>
  );
}

const listClass = 'divide-y divide-line rounded-control border border-line';
const muted = (text: string) => <p className="text-sm text-ink-muted">{text}</p>;
const mono = (text: string | undefined) => (text ? <span className="font-mono text-[13px]">{text}</span> : null);
const date = (iso: string | undefined) => (iso ? new Date(iso).toLocaleString() : null);

function RuleBadges({ action, direction }: Pick<PolicyRule, 'action' | 'direction'>) {
  return (
    <>
      <Badge tone={action === 'allow' ? 'success' : 'danger'}>{action === 'allow' ? 'Allow' : 'Deny'}</Badge>
      <Badge tone="info">{direction === 'input' ? 'Input' : 'Output'}</Badge>
    </>
  );
}

const accessReasons: Record<string, string> = {
  acl_disabled: 'Allowed (no ACL)',
  blocked: 'Blocked',
  deny_rule: 'Deny rule',
  allow_rule: 'Allow rule',
};
const accessReason = (pa: PeerAccess) => accessReasons[pa.reason] ?? 'Default allow';

/**
 * A peer's details: configuration (addresses, status, WireGuard config or
 * agent token), the access control that applies to it, and what it can
 * reach. Its owner and administrators can edit, delete it or reset its
 * captive-portal authentication from here.
 */
export default function PeerDetailModal({
  isOpen,
  onClose,
  peer,
  networks,
}: {
  isOpen: boolean;
  onClose: () => void;
  /** The peer from the list (it carries network_id and network_name). */
  peer: Peer | null;
  networks: Network[];
}) {
  const { user } = useAuth();
  const isAdmin = user?.role === 'administrator';
  const toast = useToast();
  const confirm = useConfirm();
  const queryClient = useQueryClient();
  const [tab, setTab] = useState(0);
  const [edit, setEdit] = useState<{ open: boolean; peer: Peer | null }>({ open: false, peer: null });
  const [busy, setBusy] = useState<'delete' | 'reset' | 'copy' | 'download' | null>(null);
  const [configText, setConfigText] = useState<string | null>(null);
  const [now, setNow] = useState(() => Date.now());

  const networkId = peer?.network_id ?? '';
  const peerId = peer?.id ?? '';
  // Only while open: a closed modal must not refetch a peer that was just deleted.
  const { data: fresh } = usePeer(isOpen ? networkId : '', isOpen ? peerId : '', isOpen);
  const { data: network } = useNetwork(networkId, isOpen);
  const { data: users = [] } = useUsers(isOpen && isAdmin);

  // The peer endpoint does not return the network: keep the list's.
  const p = useMemo<Peer | null>(
    () => (fresh && peer ? { ...fresh, network_id: peer.network_id, network_name: peer.network_name } : peer),
    [fresh, peer],
  );

  const groupIds = useMemo(() => p?.group_ids ?? [], [p]);
  const showAccess = isOpen && isAdmin && groupIds.length > 0;
  const { data: allGroups = [], isLoading: groupsLoading } = useGroups(networkId, showAccess);
  const { data: allPolicies = [] } = usePolicies(networkId, showAccess);
  const { data: allRoutes = [] } = useRoutes(networkId, showAccess);

  const reachability = useQuery({
    queryKey: ['peerReachability', networkId, peerId],
    queryFn: () => api.getPeerReachability(networkId, peerId),
    enabled: isOpen && tab === 2 && !!networkId && !!peerId,
  });

  useEffect(() => {
    if (!isOpen) return;
    const id = setInterval(() => setNow(Date.now()), 30 * 1000);
    return () => clearInterval(id);
  }, [isOpen]);

  // Groups by priority (lower first); their policies in order, each once;
  // their rules in order — the order the jump peer applies them in.
  const access = useMemo(() => {
    const groups = allGroups.filter((g) => groupIds.includes(g.id)).sort((a, b) => a.priority - b.priority);
    const seen = new Set<string>();
    const rules: (PolicyRule & { policyName: string; groupName: string; groupPriority: number })[] = [];
    for (const g of groups) {
      for (const policyId of g.policy_ids ?? []) {
        if (seen.has(policyId)) continue;
        seen.add(policyId);
        const policy = allPolicies.find((x) => x.id === policyId);
        for (const rule of policy?.rules ?? []) {
          rules.push({ ...rule, policyName: policy!.name, groupName: g.name, groupPriority: g.priority });
        }
      }
    }
    const routeIds = new Set(groups.flatMap((g) => g.route_ids ?? []));
    return { groups, rules, routes: allRoutes.filter((r) => routeIds.has(r.id)) };
  }, [allGroups, allPolicies, allRoutes, groupIds]);

  const handleClose = () => {
    setTab(0);
    setConfigText(null);
    setEdit((e) => ({ ...e, open: false }));
    onClose();
  };

  if (!p) return null;

  const canEdit = isAdmin || (!!p.owner_id && p.owner_id === user?.id);
  const networkName = network?.name ?? p.network_name ?? networks.find((n) => n.id === p.network_id)?.name ?? '';
  const fqdn = `${p.name}.${networkName || 'network'}.${network?.domain_suffix || 'internal'}`;
  const ownerName = !p.owner_id
    ? null
    : p.owner_id === user?.id
      ? user?.name
      : (users.find((u) => u.id === p.owner_id)?.name ?? p.owner_id);
  const session = p.session_status;

  const fetchConfig = async () => {
    if (configText) return configText;
    const text = await api.getPeerConfig(networkId, p.id);
    setConfigText(text);
    return text;
  };

  const copyConfig = async () => {
    setBusy('copy');
    try {
      await navigator.clipboard.writeText(await fetchConfig());
      toast.success('Configuration copied to the clipboard');
    } catch (err) {
      toast.error(apiErrorMessage(err, 'The configuration could not be copied.'));
    } finally {
      setBusy(null);
    }
  };

  const downloadConfig = async () => {
    setBusy('download');
    try {
      const text = await fetchConfig();
      const url = URL.createObjectURL(new Blob([text], { type: 'text/plain' }));
      const a = document.createElement('a');
      a.href = url;
      a.download = `${fqdn}.conf`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      toast.error(apiErrorMessage(err, 'The configuration could not be downloaded.'));
    } finally {
      setBusy(null);
    }
  };

  const copyToken = async () => {
    try {
      await navigator.clipboard.writeText(p.token!);
      toast.success('Token copied to the clipboard');
    } catch (err) {
      toast.error(apiErrorMessage(err, 'The token could not be copied.'));
    }
  };

  const handleDelete = async () => {
    const ok = await confirm({
      title: `Delete peer “${p.name}”?`,
      message: p.is_jump
        ? 'The peers of the network lose the routes that go through this jump peer. This cannot be undone.'
        : 'Its WireGuard configuration stops working. This cannot be undone.',
      confirmLabel: 'Delete peer',
      destructive: true,
    });
    if (!ok) return;
    setBusy('delete');
    try {
      await api.deletePeer(networkId, p.id);
    } catch (err) {
      toast.error(apiErrorMessage(err));
      setBusy(null);
      return;
    }
    setBusy(null);
    toast.success(`Peer “${p.name}” deleted`);
    refreshPeers(queryClient);
    void queryClient.invalidateQueries({ queryKey: ['groups'] });
    handleClose();
  };

  const handleResetAuth = async () => {
    const ok = await confirm({
      title: `Reset the authentication of “${p.name}”?`,
      message: (
        <div className="space-y-2">
          <p>This removes the peer from the captive portal's allowed list, cancels its pending sign-ins, clears its failed sign-in count and lifts any quarantine.</p>
          <p>
            The peer stays in the network: its next request goes to the captive portal, where its owner signs in again. Use it when its
            configuration may have leaked, or to release a quarantined peer.
          </p>
        </div>
      ),
      confirmLabel: 'Reset authentication',
      destructive: true,
    });
    if (!ok) return;
    setBusy('reset');
    try {
      await api.revokePeerAuthentication(networkId, p.id);
      toast.success(`Authentication of “${p.name}” reset`);
      refreshPeers(queryClient);
      void queryClient.invalidateQueries({ queryKey: ['peerCaptivePortalEvents'] });
    } catch (err) {
      toast.error(apiErrorMessage(err));
    } finally {
      setBusy(null);
    }
  };

  const configuration = (
    <>
      <Details
        items={[
          ['Network', networkName || null],
          ['Domain', mono(fqdn)],
          ['IPv4 address', mono(p.address)],
          ['IPv6 address', mono(p.address_v6)],
          ['Owner', ownerName],
          ['Endpoint', p.is_jump ? mono(p.endpoint) ?? '—' : null],
          ['Listen port', p.is_jump ? mono(p.listen_port ? String(p.listen_port) : undefined) ?? '—' : null],
          ['Created', date(p.created_at)],
          ['Last updated', date(p.updated_at)],
          ['ID', mono(p.id)],
        ]}
      />

      <Section title="Status">
        <div className="space-y-3 rounded-control border border-line bg-surface-muted/50 px-4 py-3">
          <Details
            columns={1}
            items={[
              ['Agent', <StatusBadge status={agentStatus(p)} />],
              ['Captive portal', p.is_jump ? null : <StatusBadge status={portalStatus(p)} />],
              ['Reported endpoint', mono(session?.current_session?.reported_endpoint)],
              ['Hostname', mono(session?.current_session?.hostname)],
              ['Last seen', date(session?.current_session?.last_seen)],
              ['Last status check', date(session?.last_checked)],
            ]}
          />
          {!p.is_jump && p.network_id && <CaptivePortalHistory networkId={p.network_id} peerId={p.id} enabled={isOpen} />}
        </div>
      </Section>

      {!p.use_agent && (
        <Section title="WireGuard configuration">
          {!p.owner_id && (
            <Alert tone="warning">The configuration cannot be downloaded: this peer has no owner. Assign it one to enable it.</Alert>
          )}
          <div className="flex flex-wrap gap-2">
            <Button
              variant="secondary"
              icon={<ClipboardDocumentIcon className="size-4" />}
              loading={busy === 'copy'}
              disabled={!p.owner_id || busy !== null}
              onClick={copyConfig}
            >
              Copy configuration
            </Button>
            <Button
              variant="secondary"
              icon={<ArrowDownTrayIcon className="size-4" />}
              loading={busy === 'download'}
              disabled={!p.owner_id || busy !== null}
              onClick={downloadConfig}
            >
              Download .conf
            </Button>
          </div>
        </Section>
      )}

      {p.use_agent && p.token && (
        <Section title="Agent token">
          <p className="text-sm text-ink-muted">The agent enrolls with this token. Keep it secret.</p>
          <Button variant="secondary" icon={<ClipboardDocumentIcon className="size-4" />} onClick={copyToken}>
            Copy token
          </Button>
        </Section>
      )}
    </>
  );

  const accessControl = !isAdmin ? (
    <Alert tone="info">
      Only administrators can see the groups, policies and routes of a peer. The Reachability tab shows what it can reach.
    </Alert>
  ) : groupIds.length === 0 ? (
    muted('This peer is in no group: no policy or route applies to it.')
  ) : groupsLoading ? (
    <div className="flex justify-center py-8 text-ink-muted" role="status">
      <Spinner className="size-5" />
      <span className="sr-only">Loading access control…</span>
    </div>
  ) : (
    <>
      <Section title="Groups">
        {access.groups.length === 0 ? (
          muted('No group found.')
        ) : (
          <ul className={listClass} aria-label="Groups">
            {access.groups.map((g) => (
              <li key={g.id} className="flex items-center gap-3 px-3 py-2">
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium text-ink">{g.name}</div>
                  {g.description && <div className="truncate text-xs text-ink-muted">{g.description}</div>}
                </div>
                <Badge tone={g.priority === 0 ? 'warning' : 'neutral'}>Priority {g.priority}</Badge>
              </li>
            ))}
          </ul>
        )}
      </Section>
      <Section title={`Effective rules (${access.rules.length})`} aside={access.rules.length > 0 ? 'Applied in this order by the jump peer' : undefined}>
        {access.rules.length === 0 ? (
          muted("No rule comes from this peer's groups.")
        ) : (
          <ol className={listClass} aria-label="Effective rules">
            {access.rules.map((rule, index) => (
              <li key={`${rule.policyName}-${index}`} className="flex gap-3 px-3 py-2">
                <span className="w-5 shrink-0 pt-0.5 text-center text-xs font-medium text-ink-subtle tabular-nums">{index + 1}</span>
                <div className="min-w-0 flex-1 space-y-1">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <RuleBadges action={rule.action} direction={rule.direction} />
                    <span className="font-mono text-xs text-ink">
                      {rule.target_type}: {rule.target}
                    </span>
                  </div>
                  {rule.description && <div className="text-xs text-ink-secondary">{rule.description}</div>}
                  <div className="text-xs text-ink-muted">
                    From policy “{rule.policyName}” in group “{rule.groupName}” (priority {rule.groupPriority})
                  </div>
                </div>
              </li>
            ))}
          </ol>
        )}
      </Section>
      <Section title="Effective routes">
        {access.routes.length === 0 ? (
          muted("No route comes from this peer's groups.")
        ) : (
          <ul className={listClass} aria-label="Effective routes">
            {access.routes.map((r) => (
              <li key={r.id} className="px-3 py-2">
                <div className="text-sm font-medium text-ink">{r.name}</div>
                <div className="font-mono text-xs text-ink-muted">
                  {[r.destination_cidr, r.destination_cidr_v6].filter(Boolean).join(' · ')}
                </div>
                {r.description && <div className="text-xs text-ink-muted">{r.description}</div>}
              </li>
            ))}
          </ul>
        )}
      </Section>
    </>
  );

  const reach = reachability.data;
  const peerAccess = reach?.peer_access ?? [];
  const firewall = reach?.rules ?? [];
  const externalRoutes = reach?.routes ?? [];
  const reachabilityTab = reachability.isLoading ? (
    <div className="flex justify-center py-8 text-ink-muted" role="status">
      <Spinner className="size-5" />
      <span className="sr-only">Computing reachability…</span>
    </div>
  ) : reachability.isError || !reach ? (
    <Alert tone="danger">The reachability could not be computed: {apiErrorMessage(reachability.error)}</Alert>
  ) : (
    <>
      {reach.is_jump && (
        <Alert tone="info">This is a jump peer: the gateway that enforces the firewall rules of the other peers of the network.</Alert>
      )}
      <Section
        title="Peers"
        aside={`${peerAccess.filter((x) => x.allowed).length} allowed, ${peerAccess.filter((x) => !x.allowed).length} denied`}
      >
        {peerAccess.length === 0 ? (
          muted('No other peer in this network.')
        ) : (
          <ul className={listClass} aria-label="Peer access">
            {peerAccess.map((pa) => (
              <li key={pa.peer_id} className="flex items-center gap-3 px-3 py-2">
                {pa.allowed ? (
                  <CheckCircleIcon className="size-4 shrink-0 text-emerald-500" aria-label="Allowed" />
                ) : (
                  <XCircleIcon className="size-4 shrink-0 text-red-500" aria-label="Denied" />
                )}
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="truncate text-sm font-medium text-ink">{pa.peer_name}</span>
                    {pa.is_jump && <Badge tone="primary">Jump</Badge>}
                  </div>
                  <div className="font-mono text-xs text-ink-muted">{pa.address}</div>
                </div>
                <Badge tone={pa.allowed ? 'success' : 'danger'}>{accessReason(pa)}</Badge>
              </li>
            ))}
          </ul>
        )}
      </Section>
      {firewall.length > 0 && (
        <Section title={`Firewall rules (${firewall.length})`} aside="Enforced on the jump peer">
          <ul className={listClass} aria-label="Firewall rules">
            {firewall.map((rule, i) => (
              <li key={i} className="space-y-1 px-3 py-2">
                <div className="flex flex-wrap items-center gap-1.5">
                  <RuleBadges action={rule.action} direction={rule.direction} />
                  <span className="font-mono text-xs text-ink">
                    {rule.target_type}: {rule.target}
                  </span>
                </div>
                {(rule.addresses?.length ?? 0) > 0 && rule.target_type !== 'cidr' && (
                  <div className="flex flex-wrap gap-1">
                    {rule.addresses.map((addr) => (
                      <Badge key={addr} className="font-mono">
                        {addr}
                      </Badge>
                    ))}
                  </div>
                )}
                {rule.description && <div className="text-xs text-ink-secondary">{rule.description}</div>}
                <div className="text-xs text-ink-muted">
                  {rule.policy_name} · {rule.group_name}
                </div>
              </li>
            ))}
          </ul>
        </Section>
      )}
      {externalRoutes.length > 0 && (
        <Section title={`External routes (${externalRoutes.length})`}>
          <ul className={listClass} aria-label="External routes">
            {externalRoutes.map((r) => (
              <li key={r.route_id} className="flex items-start justify-between gap-3 px-3 py-2">
                <div className="min-w-0">
                  <div className="text-sm font-medium text-ink">{r.route_name}</div>
                  <div className="font-mono text-xs text-ink-muted">
                    {[r.destination_cidr, r.destination_cidr_v6].filter(Boolean).join(' · ')}
                  </div>
                  <div className="text-xs text-ink-muted">{r.group_name}</div>
                </div>
                <span className="shrink-0 text-xs text-ink-muted">
                  via <span className="font-medium text-ink">{r.jump_peer_name}</span>
                </span>
              </li>
            ))}
          </ul>
        </Section>
      )}
      {firewall.length === 0 && externalRoutes.length === 0 && !reach.is_jump && muted('No policy rule or route applies to this peer.')}
    </>
  );

  const EditModal = edit.peer?.is_jump ? JumpPeerModal : RegularPeerModal;

  return (
    <>
      <Modal
        isOpen={isOpen}
        onClose={handleClose}
        size="lg"
        title={p.name}
        description={`${p.is_jump ? 'Jump peer' : 'Peer'}${networkName ? ` in network ${networkName}` : ''}`}
      >
        <ModalBody>
          <div className="flex flex-wrap items-center gap-2">
            <Badge tone={p.is_jump ? 'primary' : 'neutral'}>{p.is_jump ? 'Jump' : 'Regular'}</Badge>
            <StatusBadge status={peerStatus(p, now)} />
          </div>
          <Tabs
            selectedIndex={tab}
            onChange={setTab}
            tabs={[
              { label: 'Configuration', content: configuration },
              { label: 'Access control', count: isAdmin ? groupIds.length : undefined, content: accessControl },
              { label: 'Reachability', content: reachabilityTab },
            ]}
          />
        </ModalBody>
        {canEdit && (
          <ModalFooter>
            <div className="flex flex-wrap gap-2 sm:mr-auto">
              <Button
                variant="secondary"
                icon={<TrashIcon className="size-4" />}
                loading={busy === 'delete'}
                disabled={busy !== null}
                onClick={handleDelete}
              >
                Delete
              </Button>
              {!p.is_jump && (
                <Button
                  variant="secondary"
                  icon={<ArrowPathIcon className="size-4" />}
                  loading={busy === 'reset'}
                  disabled={busy !== null}
                  onClick={handleResetAuth}
                  title="Clears the captive-portal allowed list, pending sign-ins and quarantine of this peer"
                >
                  Reset authentication
                </Button>
              )}
            </div>
            <Button icon={<PencilSquareIcon className="size-4" />} onClick={() => setEdit({ open: true, peer: p })}>
              Edit
            </Button>
          </ModalFooter>
        )}
      </Modal>

      <EditModal
        isOpen={edit.open}
        onClose={() => setEdit((e) => ({ ...e, open: false }))}
        peer={edit.peer}
        networks={networks}
      />
    </>
  );
}
