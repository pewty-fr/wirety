import { useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { Checkbox, Combobox, FormModal, TextInput, apiErrorMessage, useToast } from '../../components/ui';
import { useAuth } from '../../contexts/AuthContext';
import { useUsers } from '../../hooks/useQueries';
import type { Network, Peer } from '../../types';
import { networkOption } from '../../utils/networks';
import { groupsUnavailable, setUpDefaultAccess } from './bootstrap';
import { refreshPeers, userOptions } from './peerStatus';

interface JumpPeerForm {
  networkId: string;
  name: string;
  endpoint: string;
  listenPort: string;
  ownerId: string;
  useAgent: boolean;
}

type Errors = Partial<Record<'networkId' | 'name' | 'listenPort', string>>;

function formFrom(peer: Peer | null, networkId: string, ownerId: string): JumpPeerForm {
  return {
    networkId: peer?.network_id ?? networkId,
    name: peer?.name ?? '',
    endpoint: peer?.endpoint ?? '',
    listenPort: String(peer ? (peer.listen_port ?? '') : 51820),
    ownerId: peer ? (peer.owner_id ?? '') : ownerId,
    useAgent: peer?.use_agent ?? true,
  };
}

/**
 * Creates or edits a jump peer, the gateway of a network. The first jump peer
 * of a network also sets up its default access (see setUpDefaultAccess).
 */
export default function JumpPeerModal({
  isOpen,
  onClose,
  peer,
  networks,
  defaultNetworkId = '',
}: {
  isOpen: boolean;
  onClose: () => void;
  /** The jump peer to edit (with its network_id), or null to create one. */
  peer: Peer | null;
  networks: Network[];
  /** Pre-selected network when creating. */
  defaultNetworkId?: string;
}) {
  const { user } = useAuth();
  const isAdmin = user?.role === 'administrator';
  const toast = useToast();
  const queryClient = useQueryClient();
  const { data: users = [] } = useUsers(isOpen && isAdmin);
  // After a partial creation, the modal goes on editing the created peer.
  const [current, setCurrent] = useState<Peer | null>(peer);
  const [form, setForm] = useState<JumpPeerForm>(() => formFrom(peer, defaultNetworkId, user?.id ?? ''));
  const [errors, setErrors] = useState<Errors>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!isOpen) return;
    setCurrent(peer);
    setForm(formFrom(peer, defaultNetworkId, user?.id ?? ''));
    setErrors({});
    setSubmitError(null);
  }, [isOpen, peer, defaultNetworkId, user?.id]);

  // With a single network, it is the obvious choice.
  const networkId = form.networkId || (!current && networks.length === 1 ? networks[0].id : '');
  const set = <K extends keyof JumpPeerForm>(key: K, value: JumpPeerForm[K]) => setForm((f) => ({ ...f, [key]: value }));

  const validate = () => {
    const e: Errors = {};
    if (!current && !networkId) e.networkId = 'Choose the network of the jump peer.';
    if (!form.name.trim()) e.name = 'Give the jump peer a name.';
    const port = form.listenPort.trim();
    if (port && !(/^\d+$/.test(port) && Number(port) >= 1 && Number(port) <= 65535)) {
      e.listenPort = 'Enter a port between 1 and 65535.';
    }
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const refresh = () => {
    refreshPeers(queryClient);
    for (const key of ['groups', 'policies', 'routes']) void queryClient.invalidateQueries({ queryKey: [key] });
  };

  const handleSubmit = async () => {
    setSubmitError(null);
    if (!validate()) return;
    setSubmitting(true);
    const name = form.name.trim();
    const endpoint = form.endpoint.trim() || undefined;
    const listen_port = form.listenPort.trim() ? Number(form.listenPort) : undefined;

    if (current) {
      try {
        const ownerChanged = isAdmin && form.ownerId !== (current.owner_id ?? '');
        const newEndpoint = endpoint ?? '';
        await api.updatePeer(current.network_id!, current.id, {
          name,
          // An empty endpoint or owner in an update removes it.
          ...(newEndpoint !== (current.endpoint ?? '') ? { endpoint: newEndpoint } : {}),
          listen_port,
          ...(ownerChanged ? { owner_id: form.ownerId } : {}),
        });
      } catch (err) {
        setSubmitError(apiErrorMessage(err));
        setSubmitting(false);
        return;
      }
      toast.success(`Jump peer “${name}” saved`);
    } else {
      const network = networks.find((n) => n.id === networkId);
      let created: Peer;
      try {
        created = await api.createPeer(networkId, {
          name,
          endpoint,
          listen_port,
          is_jump: true,
          use_agent: form.useAgent,
          ...(isAdmin && form.ownerId ? { owner_id: form.ownerId } : {}),
        });
      } catch (err) {
        setSubmitError(apiErrorMessage(err));
        setSubmitting(false);
        return;
      }
      let bootstrapped = false;
      try {
        if (network) bootstrapped = await setUpDefaultAccess(network, created);
      } catch (err) {
        if (!groupsUnavailable(err)) {
          // The peer exists: go on editing it rather than creating it again.
          setCurrent({ ...created, network_id: networkId });
          setSubmitError(
            `The jump peer was created, but the network's default access (group, policy and route) could not be set up: ${apiErrorMessage(err)}`,
          );
          setSubmitting(false);
          refresh();
          return;
        }
      }
      toast.success(
        bootstrapped
          ? `Jump peer “${name}” created, with the network's default group, policy and route`
          : `Jump peer “${name}” created`,
      );
    }
    setSubmitting(false);
    refresh();
    onClose();
  };

  const networkName = current && (networks.find((n) => n.id === current.network_id)?.name ?? current.network_name);
  const ownerChoices = [{ value: '', label: 'No owner' }, ...userOptions(users)];

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={current ? 'Edit jump peer' : 'New jump peer'}
      description={
        current
          ? `“${current.name}”${networkName ? ` in network ${networkName}` : ''}.`
          : 'A jump peer is the gateway of a network: the other peers connect through it.'
      }
      onSubmit={handleSubmit}
      submitLabel={current ? 'Save changes' : 'Create jump peer'}
      submitting={submitting}
      error={submitError}
    >
      {!current && (
        <Combobox
          label="Network"
          required
          value={networkId}
          onChange={(id) => set('networkId', id)}
          options={networks.map(networkOption)}
          placeholder="Select a network…"
          error={errors.networkId}
        />
      )}
      <TextInput
        label="Name"
        required
        value={form.name}
        onChange={(e) => set('name', e.target.value)}
        placeholder="jump-paris"
        error={errors.name}
        autoFocus={!current}
      />
      <div className="grid gap-4 sm:grid-cols-[1fr_9rem]">
        <TextInput
          label="Endpoint"
          mono
          value={form.endpoint}
          onChange={(e) => set('endpoint', e.target.value)}
          placeholder="vpn.example.com"
          hint="Public IP address or domain name, without the port."
        />
        <TextInput
          label="Listen port"
          type="number"
          inputMode="numeric"
          min={1}
          max={65535}
          value={form.listenPort}
          onChange={(e) => set('listenPort', e.target.value)}
          error={errors.listenPort}
          hint="WireGuard UDP port"
        />
      </div>
      {isAdmin && (
        <Combobox
          label="Owner"
          value={form.ownerId}
          onChange={(id) => set('ownerId', id)}
          options={ownerChoices}
          placeholder="No owner"
          hint="Without an owner, the configuration of a jump peer without agent cannot be downloaded."
        />
      )}
      {!current && (
        <Checkbox
          label="Use the agent"
          description="The agent keeps the jump peer configured from the server. Without it, you install its WireGuard configuration yourself."
          checked={form.useAgent}
          onChange={(e) => set('useAgent', e.target.checked)}
        />
      )}
    </FormModal>
  );
}
