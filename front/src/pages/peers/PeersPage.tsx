import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ComputerDesktopIcon, PencilSquareIcon, PlusIcon, ServerIcon, ServerStackIcon, TrashIcon } from '@heroicons/react/20/solid';
import { ComputerDesktopIcon as ComputerDesktopOutline, GlobeAltIcon } from '@heroicons/react/24/outline';
import NetworkFilter from '../../components/NetworkFilter';
import {
  Badge,
  Button,
  EmptyState,
  IconButton,
  Page,
  PageHeader,
  Pagination,
  Select,
  Spinner,
  Table,
  TBody,
  TD,
  TH,
  THead,
  TR,
  TextInput,
  apiErrorMessage,
  cx,
  useConfirm,
  useToast,
} from '../../components/ui';
import api from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';
import { useACLs, useNetworks, usePeers, useUsers } from '../../hooks/useQueries';
import type { Peer } from '../../types';
import JumpPeerModal from './JumpPeerModal';
import PeerDetailModal from './PeerDetailModal';
import { peerStatus, refreshPeers } from './peerStatus';
import RegularPeerModal from './RegularPeerModal';

const PAGE_SIZE = 20;
// Every peer, then filtered and paginated here (the same query as the
// captive-portal check of the layout, so it is fetched once).
const ALL_PEERS = 1000;

const typeOptions = [
  { value: 'jump', label: 'Jump' },
  { value: 'regular', label: 'Regular' },
];

const agentOptions = [
  { value: 'agent-connected', label: 'Agent connected' },
  { value: 'agent-disconnected', label: 'Agent disconnected' },
  { value: 'agent-none', label: 'No agent' },
];

interface Filters {
  networkId: string;
  type: string;
  agent: string;
  ip: string;
}

const noFilters: Filters = { networkId: '', type: '', agent: '', ip: '' };

function matches(peer: Peer, f: Filters): boolean {
  if (f.networkId && peer.network_id !== f.networkId) return false;
  if (f.type === 'jump' && !peer.is_jump) return false;
  if (f.type === 'regular' && peer.is_jump) return false;
  const agentConnected = peer.use_agent && peer.session_status?.has_active_agent;
  if (f.agent === 'agent-connected' && !agentConnected) return false;
  if (f.agent === 'agent-disconnected' && (!peer.use_agent || agentConnected)) return false;
  if (f.agent === 'agent-none' && peer.use_agent) return false;
  const ip = f.ip.trim();
  if (ip && !peer.address?.includes(ip) && !peer.address_v6?.includes(ip)) return false;
  return true;
}

function PeerIcon({ peer }: { peer: Peer }) {
  const Icon = peer.is_jump ? ServerStackIcon : peer.use_agent ? ServerIcon : ComputerDesktopIcon;
  return (
    <span className="flex size-8 shrink-0 items-center justify-center rounded-md bg-surface-muted text-ink-muted">
      <Icon className="size-4" aria-hidden="true" />
    </span>
  );
}

type FormKind = 'jump' | 'regular';

export default function PeersPage() {
  const { user } = useAuth();
  const isAdmin = user?.role === 'administrator';
  const confirm = useConfirm();
  const toast = useToast();
  const queryClient = useQueryClient();

  const [filters, setFilters] = useState<Filters>(noFilters);
  const [page, setPage] = useState(1);
  const [form, setForm] = useState<{ open: boolean; kind: FormKind; peer: Peer | null }>({ open: false, kind: 'regular', peer: null });
  const [detail, setDetail] = useState<{ open: boolean; peer: Peer | null }>({ open: false, peer: null });

  const { data: networks = [], isLoading: networksLoading } = useNetworks();
  const { data: peersData, isLoading: peersLoading } = usePeers(1, ALL_PEERS);
  // ACLs and users are reserved to administrators.
  const { data: acls = {} } = useACLs(isAdmin ? networks : []);
  const { data: users = [] } = useUsers(isAdmin);

  // The Status column depends on the age of the last handshake: re-evaluate
  // it every 30 s without refetching (Date.now() is impure during render).
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 30 * 1000);
    return () => clearInterval(id);
  }, []);

  const peers = useMemo(() => peersData?.peers ?? [], [peersData]);
  const filtered = useMemo(() => peers.filter((p) => matches(p, filters)), [peers, filters]);
  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const currentPage = Math.min(page, pageCount);
  const visible = filtered.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE);
  const filtering = Object.values(filters).some(Boolean);

  const userName = useMemo(() => new Map(users.map((u) => [u.id, u.name])), [users]);
  const isBlocked = (p: Peer) => {
    const acl = p.network_id ? acls[p.network_id] : undefined;
    return Boolean(acl?.enabled && acl.blocked_peers?.[p.id]);
  };
  const canEdit = (p: Peer) => isAdmin || (!!p.owner_id && p.owner_id === user?.id);

  const setFilter = <K extends keyof Filters>(key: K, value: Filters[K]) => {
    setFilters((f) => ({ ...f, [key]: value }));
    setPage(1);
  };

  const deletePeer = useMutation({
    mutationFn: (p: Peer) => api.deletePeer(p.network_id!, p.id),
    onSuccess: (_, p) => {
      toast.success(`Peer “${p.name}” deleted`);
      refreshPeers(queryClient);
      void queryClient.invalidateQueries({ queryKey: ['groups'] });
    },
    onError: (err) => toast.error(apiErrorMessage(err)),
  });

  const handleDelete = async (p: Peer) => {
    const ok = await confirm({
      title: `Delete peer “${p.name}”?`,
      message: p.is_jump
        ? 'The peers of the network lose the routes that go through this jump peer. This cannot be undone.'
        : 'Its WireGuard configuration stops working. This cannot be undone.',
      confirmLabel: 'Delete peer',
      destructive: true,
    });
    if (ok) deletePeer.mutate(p);
  };

  const noNetwork = !networksLoading && networks.length === 0;
  const openCreate = (kind: FormKind) => setForm({ open: true, kind, peer: null });
  const createButtons = (
    <>
      <Button
        variant="secondary"
        icon={<PlusIcon className="size-4" />}
        disabled={noNetwork}
        title={noNetwork ? 'Create a network first' : undefined}
        onClick={() => openCreate('jump')}
      >
        New jump peer
      </Button>
      <Button
        icon={<PlusIcon className="size-4" />}
        disabled={noNetwork}
        title={noNetwork ? 'Create a network first' : undefined}
        onClick={() => openCreate('regular')}
      >
        New peer
      </Button>
    </>
  );

  const total = peers.length;
  const PeerFormModal = form.kind === 'jump' ? JumpPeerModal : RegularPeerModal;

  return (
    <Page>
      <PageHeader
        title="Peers"
        description={
          peersLoading
            ? 'Devices and servers connected to your networks.'
            : `${total} peer${total === 1 ? '' : 's'} in your networks: devices and servers, and the jump peers they connect through.`
        }
        actions={createButtons}
      />

      <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-end">
        <NetworkFilter networks={networks} value={filters.networkId} onChange={(id) => setFilter('networkId', id)} />
        <Select
          label="Type"
          className="w-full sm:w-40"
          value={filters.type}
          onChange={(e) => setFilter('type', e.target.value)}
          options={typeOptions}
          placeholder="All types"
        />
        <Select
          label="Agent"
          className="w-full sm:w-48"
          value={filters.agent}
          onChange={(e) => setFilter('agent', e.target.value)}
          options={agentOptions}
          placeholder="Any"
        />
        <TextInput
          label="IP address"
          type="search"
          mono
          className="w-full sm:w-52"
          value={filters.ip}
          onChange={(e) => setFilter('ip', e.target.value)}
          placeholder="10.42.0."
        />
        {filtering && (
          <Button
            variant="ghost"
            onClick={() => {
              setFilters(noFilters);
              setPage(1);
            }}
          >
            Clear filters
          </Button>
        )}
      </div>

      {networksLoading || peersLoading ? (
        <div className="flex justify-center py-16 text-ink-muted" role="status">
          <Spinner className="size-5" />
          <span className="sr-only">Loading peers…</span>
        </div>
      ) : noNetwork ? (
        <EmptyState
          icon={GlobeAltIcon}
          title="No network yet"
          description={
            isAdmin
              ? 'Peers belong to a network: create one first, then add its jump peer and its peers.'
              : 'Peers belong to a network, and you have access to none yet. Ask an administrator to give you access to one.'
          }
          action={
            isAdmin && (
              <Link
                to="/networks"
                className="inline-flex h-9 items-center rounded-control bg-primary-600 px-3.5 text-sm font-medium text-white shadow-sm hover:bg-primary-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500"
              >
                Go to networks
              </Link>
            )
          }
        />
      ) : peers.length === 0 ? (
        <EmptyState
          icon={ComputerDesktopOutline}
          title="No peer yet"
          description="Start with a jump peer, the gateway of a network, then add the devices and servers that connect through it."
          action={<div className="flex flex-wrap justify-center gap-2">{createButtons}</div>}
        />
      ) : filtered.length === 0 ? (
        <EmptyState
          icon={ComputerDesktopOutline}
          title="No peer matches these filters"
          description="Change or clear the filters to see more peers."
        />
      ) : (
        <div className="space-y-4">
          <Table>
            <THead>
              <tr>
                <TH>Name</TH>
                <TH>Network</TH>
                <TH>Address</TH>
                {isAdmin && <TH>Owner</TH>}
                <TH>Status</TH>
                <TH>Type</TH>
                <TH>
                  <span className="sr-only">Actions</span>
                </TH>
              </tr>
            </THead>
            <TBody>
              {visible.map((p) => {
                const status = peerStatus(p, now);
                const blocked = isBlocked(p);
                return (
                  <TR key={p.id} onClick={() => setDetail({ open: true, peer: p })}>
                    <TD>
                      <div className="flex items-center gap-3">
                        <PeerIcon peer={p} />
                        <span className="font-medium text-ink">{p.name}</span>
                      </div>
                    </TD>
                    <TD>{p.network_name ?? p.network_id}</TD>
                    <TD mono>
                      <div className="text-ink">{p.address}</div>
                      {p.address_v6 && <div className="text-xs text-ink-muted">{p.address_v6}</div>}
                    </TD>
                    {isAdmin && <TD>{p.owner_id ? (userName.get(p.owner_id) ?? p.owner_id) : <span className="text-ink-subtle">—</span>}</TD>}
                    <TD>
                      <div className="flex items-center gap-1.5">
                        <Badge tone={status.tone} dot className={cx(status.pending && '[&>span]:animate-pulse')}>
                          {status.label}
                        </Badge>
                        {blocked && <Badge tone="danger">Blocked by ACL</Badge>}
                      </div>
                    </TD>
                    <TD>
                      <Badge tone={p.is_jump ? 'primary' : 'neutral'}>{p.is_jump ? 'Jump' : 'Regular'}</Badge>
                    </TD>
                    <TD className="w-0 text-right" onClick={(e) => e.stopPropagation()}>
                      {canEdit(p) && (
                        <div className="flex justify-end gap-1">
                          <IconButton label={`Edit ${p.name}`} onClick={() => setForm({ open: true, kind: p.is_jump ? 'jump' : 'regular', peer: p })}>
                            <PencilSquareIcon className="size-4" />
                          </IconButton>
                          <IconButton label={`Delete ${p.name}`} tone="danger" onClick={() => handleDelete(p)}>
                            <TrashIcon className="size-4" />
                          </IconButton>
                        </div>
                      )}
                    </TD>
                  </TR>
                );
              })}
            </TBody>
          </Table>
          <Pagination page={currentPage} pageSize={PAGE_SIZE} total={filtered.length} onPageChange={setPage} />
        </div>
      )}

      <PeerFormModal
        isOpen={form.open}
        onClose={() => setForm((f) => ({ ...f, open: false }))}
        peer={form.peer}
        networks={networks}
        defaultNetworkId={filters.networkId}
      />
      <PeerDetailModal
        isOpen={detail.open}
        onClose={() => setDetail((d) => ({ ...d, open: false }))}
        peer={detail.peer}
        networks={networks}
      />
    </Page>
  );
}
