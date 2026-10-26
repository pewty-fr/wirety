import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { PencilSquareIcon, PlusIcon, TrashIcon } from '@heroicons/react/20/solid';
import { ShieldCheckIcon } from '@heroicons/react/24/outline';
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
import { useNetworks, usePolicies } from '../../hooks/useQueries';
import type { Policy } from '../../types';
import PolicyModal from './PolicyModal';

export default function PoliciesPage() {
  const { user } = useAuth();
  const isAdmin = user?.role === 'administrator';
  const [networkId, setNetworkId] = useState('');
  const [modal, setModal] = useState<{ open: boolean; policy: Policy | null }>({ open: false, policy: null });
  const { data: networks = [] } = useNetworks();
  const { data: policies = [], isLoading } = usePolicies(networkId, isAdmin);
  const confirm = useConfirm();
  const toast = useToast();
  const queryClient = useQueryClient();

  const deletePolicy = useMutation({
    mutationFn: (p: Policy) => api.deletePolicy(p.network_id, p.id),
    onSuccess: (_, p) => {
      toast.success(`Policy “${p.name}” deleted`);
      void queryClient.invalidateQueries({ queryKey: ['policies'] });
      void queryClient.invalidateQueries({ queryKey: ['groups'] });
    },
    onError: (err) => toast.error(apiErrorMessage(err)),
  });

  const handleDelete = async (p: Policy) => {
    const ok = await confirm({
      title: `Delete policy “${p.name}”?`,
      message: 'The groups it is attached to lose its rules. This cannot be undone.',
      confirmLabel: 'Delete policy',
      destructive: true,
    });
    if (ok) deletePolicy.mutate(p);
  };

  if (!isAdmin) {
    return (
      <Page>
        <PageHeader title="Policies" />
        <Alert tone="warning" title="Administrators only">
          Only administrators can manage policies.
        </Alert>
      </Page>
    );
  }

  const networkName = (id: string) => networks.find((n) => n.id === id)?.name ?? '—';
  const create = () => setModal({ open: true, policy: null });

  return (
    <Page>
      <PageHeader
        title="Policies"
        description="Firewall rules, given to the peers of the groups they are attached to."
        actions={
          <Button icon={<PlusIcon className="size-4" />} onClick={create}>
            New policy
          </Button>
        }
      />

      <NetworkFilter networks={networks} value={networkId} onChange={setNetworkId} />

      {isLoading ? (
        <div className="flex justify-center py-16 text-ink-muted" role="status">
          <Spinner className="size-5" />
          <span className="sr-only">Loading policies…</span>
        </div>
      ) : policies.length === 0 ? (
        <EmptyState
          icon={ShieldCheckIcon}
          title="No policy yet"
          description="Create a policy to allow or deny traffic to address ranges, peers, groups and routes."
          action={
            <Button icon={<PlusIcon className="size-4" />} onClick={create}>
              New policy
            </Button>
          }
        />
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Name</TH>
              <TH>Network</TH>
              <TH className="text-right">Rules</TH>
              <TH>
                <span className="sr-only">Actions</span>
              </TH>
            </tr>
          </THead>
          <TBody>
            {policies.map((p) => (
              <TR key={p.id} onClick={() => setModal({ open: true, policy: p })}>
                <TD className="max-w-md">
                  <div className="font-medium text-ink">{p.name}</div>
                  {p.description && <div className="truncate text-xs text-ink-muted">{p.description}</div>}
                </TD>
                <TD>{p.network_name ?? networkName(p.network_id)}</TD>
                <TD className="text-right tabular-nums">{p.rules?.length ?? 0}</TD>
                <TD className="w-0 text-right" onClick={(e) => e.stopPropagation()}>
                  <div className="flex justify-end gap-1">
                    <IconButton label={`Edit ${p.name}`} onClick={() => setModal({ open: true, policy: p })}>
                      <PencilSquareIcon className="size-4" />
                    </IconButton>
                    <IconButton label={`Delete ${p.name}`} tone="danger" onClick={() => handleDelete(p)}>
                      <TrashIcon className="size-4" />
                    </IconButton>
                  </div>
                </TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}

      <PolicyModal
        isOpen={modal.open}
        onClose={() => setModal({ open: false, policy: null })}
        policy={modal.policy}
        networks={networks}
        defaultNetworkId={networkId}
      />
    </Page>
  );
}
