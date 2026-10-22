import { useState } from 'react';
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { MagnifyingGlassIcon, PencilSquareIcon, PlusIcon, TrashIcon } from '@heroicons/react/20/solid';
import { GlobeAltIcon } from '@heroicons/react/24/outline';
import api from '../../api/client';
import {
  Alert,
  Button,
  EmptyState,
  IconButton,
  Page,
  PageHeader,
  Pagination,
  Spinner,
  Table,
  TBody,
  TD,
  TH,
  THead,
  TR,
  TextInput,
  apiErrorMessage,
  useConfirm,
  useToast,
} from '../../components/ui';
import { useAuth } from '../../contexts/AuthContext';
import { useDebounce } from '../../hooks/useDebounce';
import { queryKeys } from '../../hooks/useQueries';
import type { Network } from '../../types';
import { getNetworkDomain } from '../../types';
import { computeCapacityFromCIDR, formatCapacity } from '../../utils/networkCapacity';
import NetworkDetailModal from './NetworkDetailModal';
import NetworkModal from './NetworkModal';

const PAGE_SIZE = 20;

interface ModalState {
  open: boolean;
  network: Network | null;
}

export default function NetworksPage() {
  const { user } = useAuth();
  const isAdmin = user?.role === 'administrator';
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const filter = useDebounce(search.trim(), 300);
  // The modals keep their network while they close, so their content stays.
  const [form, setForm] = useState<ModalState>({ open: false, network: null });
  const [detail, setDetail] = useState<ModalState>({ open: false, network: null });
  const confirm = useConfirm();
  const toast = useToast();
  const queryClient = useQueryClient();

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['networks', 'page', page, PAGE_SIZE, filter],
    queryFn: () => api.getNetworks(page, PAGE_SIZE, filter || undefined),
    placeholderData: keepPreviousData,
  });
  const networks = data?.data ?? [];
  const total = data?.total ?? 0;

  const deleteNetwork = useMutation({
    mutationFn: (n: Network) => api.deleteNetwork(n.id),
    onSuccess: (_, n) => {
      toast.success(`Network “${n.name}” deleted`);
      setDetail((d) => (d.network?.id === n.id ? { ...d, open: false } : d));
      // The last network of a page: go back to the previous one.
      if (networks.length === 1 && page > 1) setPage(page - 1);
      queryClient.removeQueries({ queryKey: queryKeys.network(n.id) });
      void queryClient.invalidateQueries({ queryKey: ['networks'] });
      void queryClient.invalidateQueries({ queryKey: ['peers'] });
      void queryClient.invalidateQueries({ queryKey: ['ipam'] });
    },
    onError: (err) => toast.error(apiErrorMessage(err)),
  });

  const handleDelete = async (n: Network) => {
    const ok = await confirm({
      title: `Delete network “${n.name}”?`,
      message: 'This also deletes all its peers, which lose their access. It cannot be undone.',
      confirmLabel: 'Delete network',
      destructive: true,
    });
    if (ok) deleteNetwork.mutate(n);
  };

  const openCreate = () => setForm({ open: true, network: null });
  const openEdit = (n: Network) => setForm({ open: true, network: n });
  const openDetail = (n: Network) => setDetail({ open: true, network: n });
  // The listed copy is refreshed after a change; the detail falls back on the
  // one it was opened with when the network left the page.
  const detailNetwork = networks.find((n) => n.id === detail.network?.id) ?? detail.network;

  const newButton = isAdmin && (
    <Button icon={<PlusIcon className="size-4" />} onClick={openCreate}>
      New network
    </Button>
  );

  return (
    <Page>
      <PageHeader
        title="Networks"
        description="WireGuard networks: their peers get their addresses from the network's ranges."
        actions={newButton || undefined}
      />

      <TextInput
        label="Search"
        type="search"
        value={search}
        onChange={(e) => {
          setSearch(e.target.value);
          setPage(1);
        }}
        placeholder="Name or CIDR…"
        leading={<MagnifyingGlassIcon />}
        className="w-full sm:w-72"
        autoComplete="off"
      />

      {isError ? (
        <Alert tone="danger" title="The networks could not be loaded">
          {apiErrorMessage(error)}
        </Alert>
      ) : isLoading ? (
        <div className="flex justify-center py-16 text-ink-muted" role="status">
          <Spinner className="size-5" />
          <span className="sr-only">Loading networks…</span>
        </div>
      ) : networks.length === 0 ? (
        filter ? (
          <EmptyState icon={GlobeAltIcon} title={`No network matches “${filter}”`} description="Try another name or CIDR." />
        ) : (
          <EmptyState
            icon={GlobeAltIcon}
            title="No network yet"
            description={
              isAdmin ? 'Create a network to start adding peers.' : 'No network is shared with you yet.'
            }
            action={newButton || undefined}
          />
        )
      ) : (
        <>
          <Table>
            <THead>
              <tr>
                <TH>Name</TH>
                <TH>CIDRs</TH>
                <TH>Domain</TH>
                <TH className="text-right">Peers</TH>
                <TH className="text-right">Free IPv4</TH>
                {isAdmin && (
                  <TH>
                    <span className="sr-only">Actions</span>
                  </TH>
                )}
              </tr>
            </THead>
            <TBody>
              {networks.map((n) => {
                const capacity = computeCapacityFromCIDR(n.cidr ?? '');
                const used = n.peer_count ?? 0;
                return (
                  <TR key={n.id} onClick={() => openDetail(n)}>
                    <TD>
                      <button
                        type="button"
                        className="rounded font-medium text-ink hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500"
                        onClick={(e) => {
                          e.stopPropagation();
                          openDetail(n);
                        }}
                      >
                        {n.name}
                      </button>
                    </TD>
                    <TD mono>
                      {n.cidr && <div>{n.cidr}</div>}
                      {n.cidr_v6 && <div className={n.cidr ? 'text-ink-muted' : undefined}>{n.cidr_v6}</div>}
                    </TD>
                    <TD mono>{getNetworkDomain({ ...n, domain_suffix: n.domain_suffix || 'internal' })}</TD>
                    <TD className="text-right tabular-nums">{used}</TD>
                    <TD className="text-right tabular-nums">
                      {capacity === null ? (
                        <span className="text-ink-subtle">—</span>
                      ) : (
                        `${formatCapacity(Math.max(0, capacity - used))} / ${formatCapacity(capacity)}`
                      )}
                    </TD>
                    {isAdmin && (
                      <TD className="w-0 text-right" onClick={(e) => e.stopPropagation()}>
                        <div className="flex justify-end gap-1">
                          <IconButton label={`Edit ${n.name}`} onClick={() => openEdit(n)}>
                            <PencilSquareIcon className="size-4" />
                          </IconButton>
                          <IconButton label={`Delete ${n.name}`} tone="danger" onClick={() => handleDelete(n)}>
                            <TrashIcon className="size-4" />
                          </IconButton>
                        </div>
                      </TD>
                    )}
                  </TR>
                );
              })}
            </TBody>
          </Table>
          <Pagination page={page} pageSize={PAGE_SIZE} total={total} onPageChange={setPage} />
        </>
      )}

      <NetworkDetailModal
        isOpen={detail.open}
        onClose={() => setDetail((d) => ({ ...d, open: false }))}
        network={detailNetwork}
        canManage={isAdmin}
        onEdit={openEdit}
        onDelete={handleDelete}
      />
      <NetworkModal isOpen={form.open} onClose={() => setForm((f) => ({ ...f, open: false }))} network={form.network} />
    </Page>
  );
}
