import { useEffect, useMemo, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { networkOption } from '../../utils/networks';
import { Alert, AttachList, Badge, Combobox, FormModal, Tabs, Textarea, TextInput, apiErrorMessage, useToast } from '../../components/ui';
import { useNetworkPeers, usePolicies, useRoutes } from '../../hooks/useQueries';
import type { Group, Network } from '../../types';

interface GroupForm {
  networkId: string;
  name: string;
  description: string;
  priority: string;
  peerIds: string[];
  policyIds: string[];
  routeIds: string[];
}

function formFrom(group: Group | null, networkId: string): GroupForm {
  return {
    networkId: group?.network_id ?? networkId,
    name: group?.name ?? '',
    description: group?.description ?? '',
    priority: String(group?.priority ?? 100),
    peerIds: group?.peer_ids ?? [],
    policyIds: group?.policy_ids ?? [],
    routeIds: group?.route_ids ?? [],
  };
}

const without = (ids: string[], removed: string[]) => ids.filter((id) => !removed.includes(id));

/**
 * Creates or edits a group in one form: its details and the peers, policies
 * (in evaluation order) and routes attached to it. Nothing is sent until the
 * submit; then the changes are applied one by one.
 */
export default function GroupModal({
  isOpen,
  onClose,
  group,
  networks,
  defaultNetworkId,
}: {
  isOpen: boolean;
  onClose: () => void;
  /** The group to edit, or null to create one. */
  group: Group | null;
  networks: Network[];
  /** Pre-selected network when creating. */
  defaultNetworkId: string;
}) {
  const toast = useToast();
  const queryClient = useQueryClient();
  // After a partial creation, the modal goes on editing the created group.
  const [current, setCurrent] = useState<Group | null>(group);
  const [form, setForm] = useState<GroupForm>(() => formFrom(group, defaultNetworkId));
  const [errors, setErrors] = useState<Partial<Record<'networkId' | 'name' | 'priority', string>>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [tab, setTab] = useState(0);

  useEffect(() => {
    if (!isOpen) return;
    setCurrent(group);
    setForm(formFrom(group, defaultNetworkId));
    setErrors({});
    setSubmitError(null);
    setTab(0);
  }, [isOpen, group, defaultNetworkId]);

  const networkId = form.networkId;
  const { data: peers = [] } = useNetworkPeers(networkId, isOpen && !!networkId, false);
  const { data: policies = [] } = usePolicies(networkId, isOpen && !!networkId);
  const { data: routes = [] } = useRoutes(networkId, isOpen && !!networkId);

  const isQuarantine = current?.name === 'quarantine';
  const isDefault = current?.name === 'default';
  const set = <K extends keyof GroupForm>(key: K, value: GroupForm[K]) => setForm((f) => ({ ...f, [key]: value }));

  const peerById = useMemo(() => new Map(peers.map((p) => [p.id, p])), [peers]);
  const policyById = useMemo(() => new Map(policies.map((p) => [p.id, p])), [policies]);
  const routeById = useMemo(() => new Map(routes.map((r) => [r.id, r])), [routes]);

  // A group cannot route through one of its own members.
  const circular = form.routeIds
    .map((id) => routeById.get(id))
    .filter((r) => r && form.peerIds.includes(r.jump_peer_id))
    .map((r) => `route “${r!.name}” goes through “${peerById.get(r!.jump_peer_id)?.name}”, a member of this group`);

  const validate = () => {
    const e: typeof errors = {};
    if (!current && !form.networkId) e.networkId = 'Choose the network of the group.';
    if (!form.name.trim()) e.name = 'Give the group a name.';
    const priority = Number(form.priority);
    if (!isQuarantine && (!Number.isInteger(priority) || priority < 1 || priority > 999)) {
      e.priority = 'Enter a whole number between 1 and 999.';
    }
    setErrors(e);
    if (e.networkId || e.name || e.priority) {
      setTab(0);
      return false;
    }
    if (circular.length) {
      setSubmitError(`A group cannot route through one of its own members: ${circular.join('; ')}.`);
      return false;
    }
    return true;
  };

  // Applies the attachments of `target` to the group, from `from`.
  const applyAttachments = async (g: Group, from: Pick<GroupForm, 'peerIds' | 'policyIds' | 'routeIds'>) => {
    const nid = g.network_id;
    for (const id of without(from.peerIds, form.peerIds)) await api.removePeerFromGroup(nid, g.id, id);
    for (const id of without(from.routeIds, form.routeIds)) await api.detachRouteFromGroup(nid, g.id, id);
    for (const id of without(from.policyIds, form.policyIds)) await api.detachPolicyFromGroup(nid, g.id, id);
    for (const id of without(form.peerIds, from.peerIds)) await api.addPeerToGroup(nid, g.id, id);
    for (const id of without(form.policyIds, from.policyIds)) await api.attachPolicyToGroup(nid, g.id, id);
    for (const id of without(form.routeIds, from.routeIds)) await api.attachRouteToGroup(nid, g.id, id);
    const kept = from.policyIds.filter((id) => form.policyIds.includes(id));
    const order = [...kept, ...without(form.policyIds, from.policyIds)];
    if (form.policyIds.length > 1 && order.join() !== form.policyIds.join()) {
      await api.reorderGroupPolicies(nid, g.id, form.policyIds);
    }
  };

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ['groups'] });
    void queryClient.invalidateQueries({ queryKey: ['networkPeers'] });
  };

  const handleSubmit = async () => {
    setSubmitError(null);
    if (!validate()) return;
    setSubmitting(true);
    const fields = {
      name: form.name.trim(),
      description: form.description.trim(),
      ...(isQuarantine ? {} : { priority: Number(form.priority) }),
    };
    if (!current) {
      let created: Group;
      try {
        created = await api.createGroup(form.networkId, fields);
      } catch (err) {
        setSubmitError(apiErrorMessage(err));
        setSubmitting(false);
        return;
      }
      try {
        await applyAttachments(created, { peerIds: [], policyIds: [], routeIds: [] });
      } catch (err) {
        // The group exists: go on editing it rather than creating it again.
        const fresh = await api.getGroup(created.network_id, created.id).catch(() => created);
        setCurrent(fresh);
        setSubmitError(`The group was created, but not all its attachments: ${apiErrorMessage(err)}`);
        setSubmitting(false);
        refresh();
        return;
      }
      toast.success(`Group “${created.name}” created`);
    } else {
      try {
        const changed = Object.entries(fields).some(([k, v]) => current[k as keyof Group] !== v);
        if (changed) await api.updateGroup(current.network_id, current.id, fields);
        const latest = await api.getGroup(current.network_id, current.id);
        await applyAttachments(latest, { peerIds: latest.peer_ids ?? [], policyIds: latest.policy_ids ?? [], routeIds: latest.route_ids ?? [] });
      } catch (err) {
        setSubmitError(apiErrorMessage(err));
        setSubmitting(false);
        refresh();
        return;
      }
      toast.success(`Group “${form.name.trim()}” saved`);
    }
    setSubmitting(false);
    refresh();
    onClose();
  };

  const networkName = networks.find((n) => n.id === networkId)?.name;
  const needsNetwork = <p className="text-sm text-ink-muted">Choose the network of the group first.</p>;

  const peerItems = form.peerIds.map((id) => {
    const p = peerById.get(id);
    return {
      id,
      label: p?.name ?? id,
      description: p && <span className="font-mono">{[p.address, p.address_v6].filter(Boolean).join(' · ')}</span>,
      meta: p?.is_jump ? <Badge tone="primary">Jump</Badge> : undefined,
    };
  });
  const policyItems = form.policyIds.map((id) => {
    const p = policyById.get(id);
    const rules = p?.rules?.length ?? 0;
    return { id, label: p?.name ?? id, description: p && [`${rules} rule${rules === 1 ? '' : 's'}`, p.description].filter(Boolean).join(' · ') };
  });
  const routeItems = form.routeIds.map((id) => {
    const r = routeById.get(id);
    return {
      id,
      label: r?.name ?? id,
      description: r && (
        <>
          <span className="font-mono">{[r.destination_cidr, r.destination_cidr_v6].filter(Boolean).join(' · ')}</span>
          {peerById.get(r.jump_peer_id) && ` via ${peerById.get(r.jump_peer_id)!.name}`}
        </>
      ),
    };
  });

  const move = (id: string, direction: -1 | 1) =>
    setForm((f) => {
      const ids = [...f.policyIds];
      const i = ids.indexOf(id);
      [ids[i], ids[i + direction]] = [ids[i + direction], ids[i]];
      return { ...f, policyIds: ids };
    });

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      size="lg"
      title={current ? `Edit group ${current.name}` : 'New group'}
      description={
        current
          ? `In network ${networkName ?? current.network_id}. Changes apply when you save.`
          : 'A group gives its peers policies and routes.'
      }
      onSubmit={handleSubmit}
      submitLabel={current ? 'Save changes' : 'Create group'}
      submitting={submitting}
      error={submitError}
    >
      <Tabs
        selectedIndex={tab}
        onChange={setTab}
        tabs={[
          {
            label: 'Details',
            alert: Boolean(errors.name || errors.priority || errors.networkId),
            content: (
              <>
                {isQuarantine && (
                  <Alert tone="warning" title="Quarantine group">
                    Its peers are isolated from the network. It always comes first (priority 0) and keeps its name.
                  </Alert>
                )}
                {isDefault && (
                  <Alert tone="info" title="Default group">
                    New peers join it automatically. It keeps its name.
                  </Alert>
                )}
                {!current && (
                  <Combobox
                    label="Network"
                    required
                    value={form.networkId}
                    onChange={(id) => setForm((f) => ({ ...f, networkId: id, peerIds: [], policyIds: [], routeIds: [] }))}
                    options={networks.map(networkOption)}
                    placeholder="Select a network…"
                    error={errors.networkId}
                  />
                )}
                <div className="grid gap-4 sm:grid-cols-[1fr_9rem]">
                  <TextInput
                    label="Name"
                    required
                    value={form.name}
                    onChange={(e) => set('name', e.target.value)}
                    disabled={isQuarantine || isDefault}
                    placeholder="engineering"
                    error={errors.name}
                    autoFocus={!current}
                  />
                  {!isQuarantine && (
                    <TextInput
                      label="Priority"
                      type="number"
                      inputMode="numeric"
                      min={1}
                      max={999}
                      value={form.priority}
                      onChange={(e) => set('priority', e.target.value)}
                      error={errors.priority}
                      hint="1–999, lower first"
                    />
                  )}
                </div>
                <Textarea
                  label="Description"
                  value={form.description}
                  onChange={(e) => set('description', e.target.value)}
                  placeholder="Who is in this group and why"
                />
              </>
            ),
          },
          {
            label: 'Peers',
            count: form.peerIds.length,
            content: networkId ? (
              <AttachList
                label="Add a peer"
                addLabel="Search peers…"
                emptyText="No peer in this group yet."
                items={peerItems}
                available={peers
                  .filter((p) => !form.peerIds.includes(p.id))
                  .map((p) => ({ value: p.id, label: p.name, description: p.address }))}
                onAdd={(id) => set('peerIds', [...form.peerIds, id])}
                onRemove={(id) => set('peerIds', form.peerIds.filter((x) => x !== id))}
              />
            ) : (
              needsNetwork
            ),
          },
          {
            label: 'Policies',
            count: form.policyIds.length,
            content: networkId ? (
              <AttachList
                label="Attach a policy"
                addLabel="Search policies…"
                emptyText="No policy attached: the group's peers get no access rule from it."
                hint="Policies are evaluated from top to bottom."
                items={policyItems}
                available={policies
                  .filter((p) => !form.policyIds.includes(p.id))
                  .map((p) => ({ value: p.id, label: p.name, description: `${p.rules?.length ?? 0} rules` }))}
                onAdd={(id) => set('policyIds', [...form.policyIds, id])}
                onRemove={(id) => set('policyIds', form.policyIds.filter((x) => x !== id))}
                onMove={move}
              />
            ) : (
              needsNetwork
            ),
          },
          {
            label: 'Routes',
            count: form.routeIds.length,
            content: networkId ? (
              <AttachList
                label="Attach a route"
                addLabel="Search routes…"
                emptyText="No route attached."
                items={routeItems}
                available={routes
                  .filter((r) => !form.routeIds.includes(r.id))
                  .map((r) => ({ value: r.id, label: r.name, description: r.destination_cidr || r.destination_cidr_v6 }))}
                onAdd={(id) => set('routeIds', [...form.routeIds, id])}
                onRemove={(id) => set('routeIds', form.routeIds.filter((x) => x !== id))}
              />
            ) : (
              needsNetwork
            ),
          },
        ]}
      />
    </FormModal>
  );
}
