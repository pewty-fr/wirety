import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { PencilSquareIcon, PlusIcon, TrashIcon, UserGroupIcon } from '@heroicons/react/20/solid';
import api from '../../api/client';
import NetworkFilter from '../../components/NetworkFilter';
import {
  Alert,
  Badge,
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
import { useGroups, useNetworks } from '../../hooks/useQueries';
import type { Group } from '../../types';
import GroupModal from './GroupModal';

export default function GroupsPage() {
  const { user } = useAuth();
  const isAdmin = user?.role === 'administrator';
  const [networkId, setNetworkId] = useState('');
  const [modal, setModal] = useState<{ open: boolean; group: Group | null }>({ open: false, group: null });
  const { data: networks = [] } = useNetworks();
  const { data: groups = [], isLoading } = useGroups(networkId, isAdmin);
  const confirm = useConfirm();
  const toast = useToast();
  const queryClient = useQueryClient();

  const deleteGroup = useMutation({
    mutationFn: (g: Group) => api.deleteGroup(g.network_id, g.id),
    onSuccess: (_, g) => {
      toast.success(`Group “${g.name}” deleted`);
      void queryClient.invalidateQueries({ queryKey: ['groups'] });
    },
    onError: (err) => toast.error(apiErrorMessage(err)),
  });

  const handleDelete = async (g: Group) => {
    const ok = await confirm({
      title: `Delete group “${g.name}”?`,
      message: 'Its peers stay in the network but lose the policies and routes the group gives them.',
      confirmLabel: 'Delete group',
      destructive: true,
    });
    if (ok) deleteGroup.mutate(g);
  };

  if (!isAdmin) {
    return (
      <Page>
        <PageHeader title="Groups" />
        <Alert tone="warning" title="Administrators only">
          Only administrators can manage groups.
        </Alert>
      </Page>
    );
  }

  const networkName = (id: string) => networks.find((n) => n.id === id)?.name ?? '—';

  return (
    <Page>
      <PageHeader
        title="Groups"
        description="Groups give their peers policies and routes."
        actions={
          <Button icon={<PlusIcon className="size-4" />} onClick={() => setModal({ open: true, group: null })}>
            New group
          </Button>
        }
      />

      <NetworkFilter networks={networks} value={networkId} onChange={setNetworkId} />

      {isLoading ? (
        <div className="flex justify-center py-16 text-ink-muted" role="status">
          <Spinner className="size-5" />
          <span className="sr-only">Loading groups…</span>
        </div>
      ) : groups.length === 0 ? (
        <EmptyState
          icon={UserGroupIcon}
          title="No group yet"
          description="Create a group to give a set of peers the same policies and routes."
          action={
            <Button icon={<PlusIcon className="size-4" />} onClick={() => setModal({ open: true, group: null })}>
              New group
            </Button>
          }
        />
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Name</TH>
              <TH>Network</TH>
              <TH>Priority</TH>
              <TH className="text-right">Peers</TH>
              <TH className="text-right">Policies</TH>
              <TH className="text-right">Routes</TH>
              <TH>
                <span className="sr-only">Actions</span>
              </TH>
            </tr>
          </THead>
          <TBody>
            {groups.map((g) => (
              <TR key={g.id} onClick={() => setModal({ open: true, group: g })}>
                <TD className="max-w-xs">
                  <div className="font-medium text-ink">{g.name}</div>
                  {g.description && <div className="truncate text-xs text-ink-muted">{g.description}</div>}
                </TD>
                <TD>{g.network_name ?? networkName(g.network_id)}</TD>
                <TD>
                  <Badge tone={g.priority === 0 ? 'warning' : 'neutral'}>{g.priority}</Badge>
                </TD>
                <TD className="text-right tabular-nums">{g.peer_ids?.length ?? 0}</TD>
                <TD className="text-right tabular-nums">{g.policy_ids?.length ?? 0}</TD>
                <TD className="text-right tabular-nums">{g.route_ids?.length ?? 0}</TD>
                <TD className="w-0 text-right" onClick={(e) => e.stopPropagation()}>
                  <div className="flex justify-end gap-1">
                    <IconButton label={`Edit ${g.name}`} onClick={() => setModal({ open: true, group: g })}>
                      <PencilSquareIcon className="size-4" />
                    </IconButton>
                    <IconButton label={`Delete ${g.name}`} tone="danger" onClick={() => handleDelete(g)}>
                      <TrashIcon className="size-4" />
                    </IconButton>
                  </div>
                </TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}

      <GroupModal
        isOpen={modal.open}
        onClose={() => setModal({ open: false, group: null })}
        group={modal.group}
        networks={networks}
        defaultNetworkId={networkId}
      />
    </Page>
  );
}
