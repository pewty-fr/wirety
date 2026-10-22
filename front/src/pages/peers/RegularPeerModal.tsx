import { useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { Checkbox, Combobox, FormModal, TextInput, apiErrorMessage, useToast } from '../../components/ui';
import { useAuth } from '../../contexts/AuthContext';
import { useUsers } from '../../hooks/useQueries';
import type { Network, Peer } from '../../types';
import { networkOption } from '../../utils/networks';
import { joinDefaultGroup } from './bootstrap';
import { refreshPeers, userOptions } from './peerStatus';

interface PeerForm {
  networkId: string;
  name: string;
  ownerId: string;
  useAgent: boolean;
}

type Errors = Partial<Record<'networkId' | 'name', string>>;

function formFrom(peer: Peer | null, networkId: string, ownerId: string): PeerForm {
  return {
    networkId: peer?.network_id ?? networkId,
    name: peer?.name ?? '',
    ownerId: peer ? (peer.owner_id ?? '') : ownerId,
    useAgent: peer?.use_agent ?? false,
  };
}

/**
 * Creates or edits a regular peer (a user's device or a server). A new peer
 * created by an administrator joins its network's "default" group; for other
 * users the server adds it to the network's default groups.
 */
export default function RegularPeerModal({
  isOpen,
  onClose,
  peer,
  networks,
  defaultNetworkId = '',
}: {
  isOpen: boolean;
  onClose: () => void;
  /** The peer to edit (with its network_id), or null to create one. */
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
  const [form, setForm] = useState<PeerForm>(() => formFrom(peer, defaultNetworkId, user?.id ?? ''));
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
  const set = <K extends keyof PeerForm>(key: K, value: PeerForm[K]) => setForm((f) => ({ ...f, [key]: value }));

  const validate = () => {
    const e: Errors = {};
    if (!current && !networkId) e.networkId = 'Choose the network of the peer.';
    if (!form.name.trim()) e.name = 'Give the peer a name.';
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const refresh = () => {
    refreshPeers(queryClient);
    void queryClient.invalidateQueries({ queryKey: ['groups'] });
  };

  const handleSubmit = async () => {
    setSubmitError(null);
    if (!validate()) return;
    setSubmitting(true);
    const name = form.name.trim();

    if (current) {
      try {
        // An empty owner in an update removes it.
        const ownerChanged = isAdmin && form.ownerId !== (current.owner_id ?? '');
        await api.updatePeer(current.network_id!, current.id, {
          name,
          ...(ownerChanged ? { owner_id: form.ownerId } : {}),
        });
      } catch (err) {
        setSubmitError(apiErrorMessage(err));
        setSubmitting(false);
        return;
      }
      toast.success(`Peer “${name}” saved`);
    } else {
      let created: Peer;
      try {
        created = await api.createPeer(networkId, {
          name,
          is_jump: false,
          use_agent: form.useAgent,
          // Non-admins always own the peers they create (the server sets it).
          ...(isAdmin && form.ownerId ? { owner_id: form.ownerId } : {}),
        });
      } catch (err) {
        setSubmitError(apiErrorMessage(err));
        setSubmitting(false);
        return;
      }
      // Groups are reserved to administrators.
      if (isAdmin) {
        try {
          await joinDefaultGroup(networkId, created);
        } catch (err) {
          // The peer exists: go on editing it rather than creating it again.
          setCurrent({ ...created, network_id: networkId });
          setSubmitError(`The peer was created, but it could not be added to the “default” group: ${apiErrorMessage(err)}`);
          setSubmitting(false);
          refresh();
          return;
        }
      }
      toast.success(`Peer “${name}” created`);
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
      title={current ? 'Edit peer' : 'New peer'}
      description={
        current
          ? `“${current.name}”${networkName ? ` in network ${networkName}` : ''}.`
          : isAdmin
            ? 'A device or a server connected to a network.'
            : 'A device or a server connected to a network. It will belong to you.'
      }
      onSubmit={handleSubmit}
      submitLabel={current ? 'Save changes' : 'Create peer'}
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
        placeholder="alice-laptop"
        error={errors.name}
        autoFocus={!current}
      />
      {isAdmin && (
        <Combobox
          label="Owner"
          value={form.ownerId}
          onChange={(id) => set('ownerId', id)}
          options={ownerChoices}
          placeholder="No owner"
          hint="Without an owner, the peer cannot sign in through the captive portal and its configuration cannot be downloaded."
        />
      )}
      {!current && (
        <Checkbox
          label="Use the agent"
          description="The agent keeps the peer configured from the server. Without it, you download its WireGuard configuration."
          checked={form.useAgent}
          onChange={(e) => set('useAgent', e.target.checked)}
        />
      )}
    </FormModal>
  );
}
