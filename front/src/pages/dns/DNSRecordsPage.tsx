import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { PencilSquareIcon, PlusIcon, TrashIcon } from '@heroicons/react/20/solid';
import { ArrowsRightLeftIcon, GlobeAltIcon, ServerStackIcon } from '@heroicons/react/24/outline';
import api from '../../api/client';
import { networkOption } from '../../utils/networks';
import {
  Alert,
  Button,
  Combobox,
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
import { useNetworks, useRoutes } from '../../hooks/useQueries';
import type { DNSMapping, Route } from '../../types';
import DNSRecordModal from './DNSRecordModal';
import { recordFQDN } from './dnsNames';

/**
 * The DNS records of a network, route by route: the per-route endpoint gives
 * each record its id and route, which editing and deleting need.
 */
function useDNSRecords(networkId: string, routes: Route[], enabled: boolean) {
  const routeIds = routes.map((r) => r.id);
  return useQuery({
    queryKey: ['dns', networkId, routeIds],
    queryFn: async () => (await Promise.all(routeIds.map((id) => api.getDNSMappings(networkId, id)))).flat(),
    enabled: enabled && !!networkId,
  });
}

function Loading({ label }: { label: string }) {
  return (
    <div className="flex justify-center py-16 text-ink-muted" role="status">
      <Spinner className="size-5" />
      <span className="sr-only">{label}</span>
    </div>
  );
}

export default function DNSRecordsPage() {
  const { user } = useAuth();
  const isAdmin = user?.role === 'administrator';
  const navigate = useNavigate();
  const [pickedNetworkId, setPickedNetworkId] = useState('');
  const [modal, setModal] = useState<{ open: boolean; record: DNSMapping | null }>({ open: false, record: null });
  const { data: networks = [], isLoading: networksLoading } = useNetworks();
  // DNS records live in one network: the first one until another is picked.
  const network = networks.find((n) => n.id === pickedNetworkId) ?? networks[0];
  const networkId = network?.id ?? '';
  const { data: routes = [], isLoading: routesLoading } = useRoutes(networkId, isAdmin && !!networkId);
  const records = useDNSRecords(networkId, routes, isAdmin && !routesLoading);
  const routeById = useMemo(() => new Map(routes.map((r) => [r.id, r])), [routes]);
  const confirm = useConfirm();
  const toast = useToast();
  const queryClient = useQueryClient();

  const fqdnOf = (r: DNSMapping) => recordFQDN(r.name, network?.name ?? '', network?.domain_suffix);
  const addressesOf = (r: DNSMapping) => [r.ip_address, r.ip_address_v6].filter(Boolean).join(', ');

  const deleteRecord = useMutation({
    mutationFn: (r: DNSMapping) => api.deleteDNSMapping(networkId, r.route_id, r.id),
    onSuccess: (_, r) => {
      toast.success(`DNS record “${fqdnOf(r)}” deleted`);
      void queryClient.invalidateQueries({ queryKey: ['dns'] });
    },
    onError: (err) => toast.error(apiErrorMessage(err)),
  });

  const handleDelete = async (r: DNSMapping) => {
    const ok = await confirm({
      title: `Delete DNS record “${fqdnOf(r)}”?`,
      message: (
        <>
          <p className="font-mono text-[13px] text-ink">
            {fqdnOf(r)} → {addressesOf(r) || 'no address'}
          </p>
          <p className="mt-1">Peers will no longer resolve this name.</p>
        </>
      ),
      confirmLabel: 'Delete record',
      destructive: true,
    });
    if (ok) deleteRecord.mutate(r);
  };

  if (!isAdmin) {
    return (
      <Page>
        <PageHeader title="DNS records" />
        <Alert tone="warning" title="Administrators only">
          Only administrators can manage DNS records.
        </Alert>
      </Page>
    );
  }

  const openNew = () => setModal({ open: true, record: null });
  const canCreate = !!network && routes.length > 0;
  const newButton = (
    <Button icon={<PlusIcon className="size-4" />} onClick={openNew} disabled={!canCreate}>
      New record
    </Button>
  );

  let content;
  if (networksLoading || routesLoading || records.isLoading) {
    content = <Loading label="Loading DNS records…" />;
  } else if (!network) {
    content = (
      <EmptyState
        icon={ServerStackIcon}
        title="No network yet"
        description="DNS records belong to a network: create a network, then a route, first."
        action={<Button onClick={() => navigate('/networks')}>Go to networks</Button>}
      />
    );
  } else if (routes.length === 0) {
    content = (
      <EmptyState
        icon={ArrowsRightLeftIcon}
        title={`No route in ${network.name}`}
        description="A DNS record points to addresses behind a route: create a route in this network first."
        action={<Button onClick={() => navigate('/routes')}>Go to routes</Button>}
      />
    );
  } else if (records.isError) {
    content = (
      <Alert tone="danger" title="The DNS records could not be loaded">
        {apiErrorMessage(records.error)}
      </Alert>
    );
  } else if (!records.data?.length) {
    content = (
      <EmptyState
        icon={GlobeAltIcon}
        title="No DNS record yet"
        description={`Give an address behind a route a name the peers of ${network.name} can resolve.`}
        action={newButton}
      />
    );
  } else {
    content = (
      <Table>
        <THead>
          <tr>
            <TH>Name</TH>
            <TH>Addresses</TH>
            <TH>Route</TH>
            <TH>
              <span className="sr-only">Actions</span>
            </TH>
          </tr>
        </THead>
        <TBody>
          {records.data.map((r) => {
            const fqdn = fqdnOf(r);
            const route = routeById.get(r.route_id);
            return (
              <TR key={r.id} onClick={() => setModal({ open: true, record: r })}>
                <TD>
                  <div className="font-mono text-[13px] font-medium text-ink">{fqdn}</div>
                  <div className="text-xs text-ink-muted">Label {r.name}</div>
                </TD>
                <TD mono>
                  {r.ip_address && <div>{r.ip_address}</div>}
                  {r.ip_address_v6 && <div className="text-xs text-ink-muted">{r.ip_address_v6}</div>}
                </TD>
                <TD>
                  <div>{route?.name ?? r.route_id}</div>
                  {route && (
                    <div className="font-mono text-xs text-ink-muted">
                      {[route.destination_cidr, route.destination_cidr_v6].filter(Boolean).join(' · ')}
                    </div>
                  )}
                </TD>
                <TD className="w-0 text-right" onClick={(e) => e.stopPropagation()}>
                  <div className="flex justify-end gap-1">
                    <IconButton label={`Edit ${fqdn}`} onClick={() => setModal({ open: true, record: r })}>
                      <PencilSquareIcon className="size-4" />
                    </IconButton>
                    <IconButton label={`Delete ${fqdn}`} tone="danger" onClick={() => handleDelete(r)}>
                      <TrashIcon className="size-4" />
                    </IconButton>
                  </div>
                </TD>
              </TR>
            );
          })}
        </TBody>
      </Table>
    );
  }

  return (
    <Page>
      <PageHeader
        title="DNS records"
        description="Names the peers of a network resolve inside the VPN, for addresses behind a route."
        actions={newButton}
      />

      {networks.length > 0 && (
        <Combobox
          label="Network"
          value={networkId}
          onChange={(id) => id && setPickedNetworkId(id)}
          options={networks.map(networkOption)}
          placeholder="Select a network…"
          className="w-full sm:w-72"
        />
      )}

      {content}

      {network && (
        <DNSRecordModal
          isOpen={modal.open}
          onClose={() => setModal({ open: false, record: null })}
          record={modal.record}
          network={network}
          routes={routes}
        />
      )}
    </Page>
  );
}
