import { useEffect, useMemo, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { networkOption } from '../../utils/networks';
import { getCIDRError } from '../../utils/validation';
import {
  AttachList,
  Badge,
  Combobox,
  FormModal,
  Spinner,
  Tabs,
  Textarea,
  TextInput,
  apiErrorMessage,
  useToast,
} from '../../components/ui';
import { useGroups, useNetworkPeers } from '../../hooks/useQueries';
import type { Network, Route } from '../../types';

interface RouteForm {
  networkId: string;
  name: string;
  description: string;
  cidr: string;
  cidrV6: string;
  jumpPeerId: string;
  domainSuffix: string;
  /** The groups the route is attached to; null while untouched in edit mode. */
  groupIds: string[] | null;
}

type FieldErrors = Partial<Record<'networkId' | 'name' | 'cidr' | 'cidrV6' | 'jumpPeerId' | 'domainSuffix', string>>;

function formFrom(route: Route | null, networkId: string): RouteForm {
  return {
    networkId: route?.network_id ?? networkId,
    name: route?.name ?? '',
    description: route?.description ?? '',
    cidr: route?.destination_cidr ?? '',
    cidrV6: route?.destination_cidr_v6 ?? '',
    jumpPeerId: route?.jump_peer_id ?? '',
    domainSuffix: route?.domain_suffix || 'internal',
    groupIds: route ? null : [],
  };
}

/** The route fields of the API, as the form would send them. */
function fieldsOf(form: RouteForm) {
  return {
    name: form.name.trim(),
    description: form.description.trim(),
    destination_cidr: form.cidr.trim(),
    destination_cidr_v6: form.cidrV6.trim(),
    jump_peer_id: form.jumpPeerId,
    domain_suffix: form.domainSuffix.trim() || 'internal',
  };
}

/** The error of a destination CIDR of the given family, if any. */
function cidrError(value: string, family: 4 | 6): string | undefined {
  if (!value) return undefined;
  const error = getCIDRError(value);
  if (error) return error;
  const isV6 = value.includes(':');
  if (family === 4 && isV6) return 'This is an IPv6 CIDR: enter it in the IPv6 field.';
  if (family === 6 && !isV6) return 'This is an IPv4 CIDR: enter it in the IPv4 field.';
  return undefined;
}

// Mirrors the server's rule: letters, digits, dots and hyphens, not at the ends.
const DOMAIN_SUFFIX = /^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$/i;

const without = (ids: string[], removed: string[]) => ids.filter((id) => !removed.includes(id));

const peersText = (n: number) => `${n} peer${n === 1 ? '' : 's'}`;

/**
 * Creates or edits a route in one form: its destinations, jump peer and the
 * groups it is attached to. Nothing is sent until the submit; then the
 * changes are applied one by one.
 */
export default function RouteModal({
  isOpen,
  onClose,
  route,
  networks,
  defaultNetworkId,
}: {
  isOpen: boolean;
  onClose: () => void;
  /** The route to edit, or null to create one. */
  route: Route | null;
  networks: Network[];
  /** Pre-selected network when creating. */
  defaultNetworkId: string;
}) {
  const toast = useToast();
  const queryClient = useQueryClient();
  // After a partial creation, the modal goes on editing the created route.
  const [current, setCurrent] = useState<Route | null>(route);
  const [form, setForm] = useState<RouteForm>(() => formFrom(route, defaultNetworkId));
  const [errors, setErrors] = useState<FieldErrors>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [tab, setTab] = useState(0);

  useEffect(() => {
    if (!isOpen) return;
    setCurrent(route);
    setForm(formFrom(route, defaultNetworkId));
    setErrors({});
    setSubmitError(null);
    setTab(0);
  }, [isOpen, route, defaultNetworkId]);

  const networkId = form.networkId;
  const { data: peers = [] } = useNetworkPeers(networkId, isOpen && !!networkId, false);
  const { data: groups = [], isLoading: groupsLoading } = useGroups(networkId, isOpen && !!networkId);

  const set = <K extends keyof RouteForm>(key: K, value: RouteForm[K]) => setForm((f) => ({ ...f, [key]: value }));

  const jumpPeers = useMemo(() => peers.filter((p) => p.is_jump), [peers]);
  const peerById = useMemo(() => new Map(peers.map((p) => [p.id, p])), [peers]);
  const groupById = useMemo(() => new Map(groups.map((g) => [g.id, g])), [groups]);

  // The groups the route is attached to on the server, then as staged.
  const attachedIds = useMemo(
    () => (current ? groups.filter((g) => g.route_ids?.includes(current.id)).map((g) => g.id) : []),
    [groups, current],
  );
  const groupIds = form.groupIds ?? attachedIds;
  const setGroupIds = (ids: string[]) => set('groupIds', ids);

  // A route cannot be given to a group its own jump peer belongs to.
  const circularIds = groupIds.filter((id) => form.jumpPeerId && groupById.get(id)?.peer_ids?.includes(form.jumpPeerId));
  const circular = circularIds.map((id) => groupById.get(id)!.name);
  const jumpPeerLabel = peerById.get(form.jumpPeerId)?.name ?? 'the jump peer';

  const validate = () => {
    const e: FieldErrors = {};
    const { destination_cidr: cidr, destination_cidr_v6: cidrV6, domain_suffix: suffix } = fieldsOf(form);
    if (!current && !form.networkId) e.networkId = 'Choose the network of the route.';
    if (!form.name.trim()) e.name = 'Give the route a name.';
    if (!cidr && !cidrV6) e.cidr = 'Enter an IPv4 or an IPv6 CIDR, or both.';
    e.cidr ??= cidrError(cidr, 4);
    e.cidrV6 = cidrError(cidrV6, 6);
    if (!form.jumpPeerId) e.jumpPeerId = 'Choose the jump peer the traffic goes through.';
    if (!DOMAIN_SUFFIX.test(suffix)) e.domainSuffix = 'Use letters, digits, dots and hyphens, not at the start or end.';
    const invalid = Object.values(e).some(Boolean);
    setErrors(invalid ? e : {});
    if (invalid) {
      setTab(0);
      return false;
    }
    if (circular.length) {
      setSubmitError(
        `A route cannot be attached to a group its jump peer belongs to: “${jumpPeerLabel}” is a member of ${circular
          .map((n) => `“${n}”`)
          .join(', ')}.`,
      );
      return false;
    }
    return true;
  };

  // Attaches the route to the staged groups and detaches it from the others.
  const applyGroups = async (r: Route, desired: string[]) => {
    const nid = r.network_id;
    const fresh = await api.getGroups(nid);
    const from = fresh.filter((g) => g.route_ids?.includes(r.id)).map((g) => g.id);
    for (const id of without(from, desired)) await api.detachRouteFromGroup(nid, id, r.id);
    for (const id of without(desired, from)) await api.attachRouteToGroup(nid, id, r.id);
  };

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ['routes'] });
    void queryClient.invalidateQueries({ queryKey: ['groups'] });
  };

  const handleSubmit = async () => {
    setSubmitError(null);
    if (!validate()) return;
    setSubmitting(true);
    const fields = fieldsOf(form);
    if (!current) {
      let created: Route;
      try {
        created = await api.createRoute(form.networkId, {
          ...fields,
          destination_cidr: fields.destination_cidr || undefined,
          destination_cidr_v6: fields.destination_cidr_v6 || undefined,
        });
        created = { ...created, network_id: created.network_id || form.networkId };
      } catch (err) {
        setSubmitError(apiErrorMessage(err));
        setSubmitting(false);
        return;
      }
      try {
        // In order, one at a time: the server checks each attachment.
        for (const id of groupIds) await api.attachRouteToGroup(created.network_id, id, created.id);
      } catch (err) {
        // The route exists: go on editing it rather than creating it again.
        setCurrent(created);
        setSubmitError(`The route was created, but not attached to all its groups: ${apiErrorMessage(err)}`);
        setSubmitting(false);
        refresh();
        return;
      }
      toast.success(`Route “${created.name}” created`);
    } else {
      try {
        const before = fieldsOf(formFrom(current, current.network_id));
        const changed = Object.fromEntries(
          Object.entries(fields).filter(([key, value]) => before[key as keyof typeof before] !== value),
        );
        if (Object.keys(changed).length) {
          const updated = await api.updateRoute(current.network_id, current.id, changed);
          // A later failure keeps the modal open on the saved details.
          setCurrent({ ...current, ...updated, network_id: current.network_id });
        }
        if (form.groupIds) await applyGroups(current, form.groupIds);
      } catch (err) {
        setSubmitError(apiErrorMessage(err));
        setSubmitting(false);
        refresh();
        return;
      }
      toast.success(`Route “${fields.name}” saved`);
    }
    setSubmitting(false);
    refresh();
    onClose();
  };

  const networkName = networks.find((n) => n.id === networkId)?.name;

  const groupItems = groupIds.map((id) => {
    const g = groupById.get(id);
    return {
      id,
      label: g?.name ?? id,
      description: g && [peersText(g.peer_ids?.length ?? 0), g.description].filter(Boolean).join(' · '),
      meta: circularIds.includes(id) ? <Badge tone="danger">Has the jump peer</Badge> : undefined,
    };
  });

  const groupsTab = !networkId ? (
    <p className="text-sm text-ink-muted">Choose the network of the route first.</p>
  ) : groupsLoading ? (
    <div className="flex justify-center py-6 text-ink-muted" role="status">
      <Spinner className="size-5" />
      <span className="sr-only">Loading groups…</span>
    </div>
  ) : (
    <AttachList
      label="Add a group"
      addLabel="Search groups…"
      emptyText="Not attached to any group: no peer uses this route yet."
      hint="The peers of these groups reach the destinations through the jump peer."
      items={groupItems}
      available={groups
        .filter((g) => !groupIds.includes(g.id))
        .map((g) => ({ value: g.id, label: g.name, description: peersText(g.peer_ids?.length ?? 0) }))}
      onAdd={(id) => setGroupIds([...groupIds, id])}
      onRemove={(id) => setGroupIds(groupIds.filter((x) => x !== id))}
    />
  );

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      size="lg"
      title={current ? `Edit route ${current.name}` : 'New route'}
      description={
        current
          ? `In network ${networkName ?? current.network_id}. Changes apply when you save.`
          : 'A route sends the traffic of its groups’ peers to a destination, through a jump peer.'
      }
      onSubmit={handleSubmit}
      submitLabel={current ? 'Save changes' : 'Create route'}
      submitting={submitting}
      error={submitError}
    >
      <Tabs
        selectedIndex={tab}
        onChange={setTab}
        tabs={[
          {
            label: 'Details',
            alert: Object.values(errors).some(Boolean),
            content: (
              <>
                {!current && (
                  <Combobox
                    label="Network"
                    required
                    value={form.networkId}
                    onChange={(id) => setForm((f) => ({ ...f, networkId: id, jumpPeerId: '', groupIds: [] }))}
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
                  placeholder="datacenter"
                  error={errors.name}
                  autoFocus={!current}
                />
                <Textarea
                  label="Description"
                  value={form.description}
                  onChange={(e) => set('description', e.target.value)}
                  placeholder="What this route gives access to"
                  rows={2}
                />
                <div className="grid gap-4 sm:grid-cols-2">
                  <TextInput
                    label="IPv4 destination"
                    mono
                    value={form.cidr}
                    onChange={(e) => set('cidr', e.target.value)}
                    placeholder="10.0.0.0/24"
                    error={errors.cidr}
                    hint="At least one destination, IPv4 or IPv6."
                  />
                  <TextInput
                    label="IPv6 destination"
                    mono
                    value={form.cidrV6}
                    onChange={(e) => set('cidrV6', e.target.value)}
                    placeholder="fd00::/64"
                    error={errors.cidrV6}
                  />
                </div>
                <Combobox
                  label="Jump peer"
                  required
                  value={form.jumpPeerId}
                  onChange={(id) => set('jumpPeerId', id)}
                  options={jumpPeers.map((p) => ({ value: p.id, label: p.name, description: p.address }))}
                  placeholder={!networkId ? 'Choose a network first' : jumpPeers.length ? 'Select a jump peer…' : 'No jump peer in this network'}
                  disabled={!networkId}
                  error={errors.jumpPeerId}
                  hint={
                    networkId && !jumpPeers.length
                      ? 'This network has no jump peer: create one on the Peers page first.'
                      : 'The peer the traffic to the destinations goes through.'
                  }
                />
                <TextInput
                  label="Domain suffix"
                  mono
                  value={form.domainSuffix}
                  onChange={(e) => set('domainSuffix', e.target.value)}
                  placeholder="internal"
                  error={errors.domainSuffix}
                />
              </>
            ),
          },
          {
            label: 'Groups',
            count: groupIds.length,
            alert: circular.length > 0 && Boolean(submitError),
            content: groupsTab,
          },
        ]}
      />
    </FormModal>
  );
}
