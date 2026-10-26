import { useMemo, useState } from 'react';
import { useMutation, useQueries, useQueryClient } from '@tanstack/react-query';
import type { UseQueryResult } from '@tanstack/react-query';
import { PencilSquareIcon, PlusIcon, TrashIcon } from '@heroicons/react/20/solid';
import { ArrowsRightLeftIcon } from '@heroicons/react/24/outline';
import api from '../../api/client';
import NetworkFilter from '../../components/NetworkFilter';
import {
  Alert,
  Button,
  EmptyState,
  IconButton,
  Page,
  PageHeader,
  Spinner,
  Table,
  TBody,
  TD,
  TH,
  THead,
  TR,
  apiErrorMessage,
  useConfirm,
  useToast,
} from '../../components/ui';
import { useAuth } from '../../contexts/AuthContext';
import { queryKeys, useNetworks, useRoutes } from '../../hooks/useQueries';
import type { Peer, Route } from '../../types';
import RouteModal from './RouteModal';

// Peer ids are unique across networks: one map resolves every jump peer.
function peerNames(results: UseQueryResult<Peer[]>[]): Map<string, string> {
  const names = new Map<string, string>();
  for (const r of results) for (const p of r.data ?? []) names.set(p.id, p.name);
  return names;
}

/** The names of the peers of the given networks, by id. */
function usePeerNames(networkIds: string[]) {
  return useQueries({
    queries: networkIds.map((id) => ({
      queryKey: queryKeys.networkPeers(id),
      queryFn: () => api.getAllNetworkPeers(id),
    })),
    combine: peerNames,
  });
}

export default function RoutesPage() {
  const { user } = useAuth();
  const isAdmin = user?.role === 'administrator';
  const [networkId, setNetworkId] = useState('');
  const [modal, setModal] = useState<{ open: boolean; route: Route | null }>({ open: false, route: null });
  const { data: networks = [] } = useNetworks();
  const { data: routes = [], isLoading } = useRoutes(networkId, isAdmin);
  // Jump peers are resolved in the networks of the listed routes, so the
  // "All networks" view shows names too.
  const routeNetworkIds = useMemo(() => [...new Set(routes.map((r) => r.network_id))].filter(Boolean), [routes]);
  const jumpPeerName = usePeerNames(routeNetworkIds);
  const confirm = useConfirm();
  const toast = useToast();
  const queryClient = useQueryClient();

  const deleteRoute = useMutation({
    mutationFn: (r: Route) => api.deleteRoute(r.network_id, r.id),
    onSuccess: (_, r) => {
      toast.success(`Route “${r.name}” deleted`);
      void queryClient.invalidateQueries({ queryKey: ['routes'] });
      void queryClient.invalidateQueries({ queryKey: ['groups'] });
      void queryClient.invalidateQueries({ queryKey: ['dns'] });
    },
    onError: (err) => toast.error(apiErrorMessage(err)),
  });

  const handleDelete = async (r: Route) => {
    const ok = await confirm({
      title: `Delete route “${r.name}”?`,
      message: 'The peers of its groups stop reaching its destinations, and its DNS records are deleted.',
      confirmLabel: 'Delete route',
      destructive: true,
    });
    if (ok) deleteRoute.mutate(r);
  };

  if (!isAdmin) {
    return (
      <Page>
        <PageHeader title="Routes" />
        <Alert tone="warning" title="Administrators only">
          Only administrators can manage routes.
        </Alert>
      </Page>
    );
  }

  const networkName = (id: string) => networks.find((n) => n.id === id)?.name ?? '—';
  const openNew = () => setModal({ open: true, route: null });

  return (
    <Page>
      <PageHeader
        title="Routes"
        description="Routes send the traffic of a group's peers to other networks, through a jump peer."
        actions={
          <Button icon={<PlusIcon className="size-4" />} onClick={openNew}>
            New route
          </Button>
        }
      />

      <NetworkFilter networks={networks} value={networkId} onChange={setNetworkId} />

      {isLoading ? (
        <div className="flex justify-center py-16 text-ink-muted" role="status">
          <Spinner className="size-5" />
          <span className="sr-only">Loading routes…</span>
        </div>
      ) : routes.length === 0 ? (
        <EmptyState
          icon={ArrowsRightLeftIcon}
          title="No route yet"
          description="Create a route to let the peers of a group reach a network behind a jump peer."
          action={
            <Button icon={<PlusIcon className="size-4" />} onClick={openNew}>
              New route
            </Button>
          }
        />
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Name</TH>
              <TH>Network</TH>
              <TH>Destination</TH>
              <TH>Jump peer</TH>
              <TH>Domain suffix</TH>
              <TH>
                <span className="sr-only">Actions</span>
              </TH>
            </tr>
          </THead>
          <TBody>
            {routes.map((r) => (
              <TR key={r.id} onClick={() => setModal({ open: true, route: r })}>
                <TD className="max-w-xs">
                  <div className="font-medium text-ink">{r.name}</div>
                  {r.description && <div className="truncate text-xs text-ink-muted">{r.description}</div>}
                </TD>
                <TD>{r.network_name ?? networkName(r.network_id)}</TD>
                <TD mono>
                  {r.destination_cidr && <div>{r.destination_cidr}</div>}
                  {r.destination_cidr_v6 && <div className="text-xs text-ink-muted">{r.destination_cidr_v6}</div>}
                  {!r.destination_cidr && !r.destination_cidr_v6 && <span className="text-ink-subtle">—</span>}
                </TD>
                <TD>
                  {jumpPeerName.get(r.jump_peer_id) ?? (
                    <span className="font-mono text-xs text-ink-muted">{r.jump_peer_id || '—'}</span>
                  )}
                </TD>
                <TD mono>{r.domain_suffix || 'internal'}</TD>
                <TD className="w-0 text-right" onClick={(e) => e.stopPropagation()}>
                  <div className="flex justify-end gap-1">
                    <IconButton label={`Edit ${r.name}`} onClick={() => setModal({ open: true, route: r })}>
                      <PencilSquareIcon className="size-4" />
                    </IconButton>
                    <IconButton label={`Delete ${r.name}`} tone="danger" onClick={() => handleDelete(r)}>
                      <TrashIcon className="size-4" />
                    </IconButton>
                  </div>
                </TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}

      <RouteModal
        isOpen={modal.open}
        onClose={() => setModal({ open: false, route: null })}
        route={modal.route}
        networks={networks}
        defaultNetworkId={networkId}
      />
    </Page>
  );
}
