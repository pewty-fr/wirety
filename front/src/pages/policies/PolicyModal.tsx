import { useEffect, useMemo, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { networkOption } from '../../utils/networks';
import { AttachList, Combobox, FormModal, Tabs, Textarea, TextInput, apiErrorMessage, useToast } from '../../components/ui';
import { useGroups, useNetworkPeers, useRoutes } from '../../hooks/useQueries';
import type { Network, Policy } from '../../types';
import RulesEditor from './RulesEditor';
import { blankDraft, formRule, isPending, ruleFields } from './rules';
import type { FormRule, RuleDraft } from './rules';

interface PolicyForm {
  networkId: string;
  name: string;
  description: string;
  /** The rules, in order: saved ones keep their id, added ones have none yet. */
  rules: FormRule[];
  /** Groups to attach the policy to, and to detach it from, when saving. */
  attachGroupIds: string[];
  detachGroupIds: string[];
}

function formFrom(policy: Policy | null, networkId: string): PolicyForm {
  return {
    networkId: policy?.network_id ?? networkId,
    name: policy?.name ?? '',
    description: policy?.description ?? '',
    rules: (policy?.rules ?? []).map(formRule),
    attachGroupIds: [],
    detachGroupIds: [],
  };
}

/**
 * Creates or edits a policy in one form: its details, its rules (in order)
 * and the groups it is attached to. Nothing is sent until the submit; then a
 * new policy is created with its rules, and the changes are applied one by
 * one.
 */
export default function PolicyModal({
  isOpen,
  onClose,
  policy,
  networks,
  defaultNetworkId,
}: {
  isOpen: boolean;
  onClose: () => void;
  /** The policy to edit, or null to create one. */
  policy: Policy | null;
  networks: Network[];
  /** Pre-selected network when creating. */
  defaultNetworkId: string;
}) {
  const toast = useToast();
  const queryClient = useQueryClient();
  // After a partial creation, the modal goes on editing the created policy.
  const [current, setCurrent] = useState<Policy | null>(policy);
  const [form, setForm] = useState<PolicyForm>(() => formFrom(policy, defaultNetworkId));
  const [draft, setDraft] = useState<RuleDraft>(blankDraft);
  const [draftError, setDraftError] = useState<string | null>(null);
  const [errors, setErrors] = useState<Partial<Record<'networkId' | 'name', string>>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [tab, setTab] = useState(0);

  useEffect(() => {
    if (!isOpen) return;
    setCurrent(policy);
    setForm(formFrom(policy, defaultNetworkId));
    setDraft(blankDraft);
    setDraftError(null);
    setErrors({});
    setSubmitError(null);
    setTab(0);
  }, [isOpen, policy, defaultNetworkId]);

  const networkId = form.networkId;
  const enabled = isOpen && !!networkId;
  const { data: peers = [] } = useNetworkPeers(networkId, enabled, false);
  const { data: groups = [] } = useGroups(networkId, enabled);
  const { data: routes = [] } = useRoutes(networkId, enabled);
  const network = networks.find((n) => n.id === networkId);

  const set = <K extends keyof PolicyForm>(key: K, value: PolicyForm[K]) => setForm((f) => ({ ...f, [key]: value }));

  // The groups the policy will be attached to once saved.
  const groupById = useMemo(() => new Map(groups.map((g) => [g.id, g])), [groups]);
  const savedGroupIds = current ? groups.filter((g) => g.policy_ids?.includes(current.id)).map((g) => g.id) : [];
  const groupIds = [...new Set([...savedGroupIds, ...form.attachGroupIds])].filter((id) => !form.detachGroupIds.includes(id));

  const attachGroup = (id: string) =>
    setForm((f) => ({
      ...f,
      detachGroupIds: f.detachGroupIds.filter((x) => x !== id),
      attachGroupIds: f.attachGroupIds.includes(id) ? f.attachGroupIds : [...f.attachGroupIds, id],
    }));
  const detachGroup = (id: string) =>
    setForm((f) => ({
      ...f,
      attachGroupIds: f.attachGroupIds.filter((x) => x !== id),
      detachGroupIds: f.detachGroupIds.includes(id) ? f.detachGroupIds : [...f.detachGroupIds, id],
    }));

  const validate = () => {
    const e: typeof errors = {};
    if (!current && !form.networkId) e.networkId = 'Choose the network of the policy.';
    if (!form.name.trim()) e.name = 'Give the policy a name.';
    setErrors(e);
    if (e.networkId || e.name) {
      setTab(0);
      return false;
    }
    if (isPending(draft)) {
      setDraftError('Add this rule with “Add rule”, or clear it, before saving.');
      setTab(1);
      return false;
    }
    return true;
  };

  // Removes the rules taken out of `p` and adds the new ones, in order.
  const applyRules = async (p: Policy) => {
    const kept = new Set(form.rules.map((r) => r.id));
    for (const r of p.rules ?? []) {
      if (!kept.has(r.id)) await api.removeRuleFromPolicy(p.network_id, p.id, r.id);
    }
    for (const r of form.rules) {
      if (r.id) continue;
      const saved = await api.addRuleToPolicy(p.network_id, p.id, ruleFields(r));
      // A retry after a failure must not add it again.
      setForm((f) => ({ ...f, rules: f.rules.map((x) => (x.key === r.key ? { ...x, id: saved.id } : x)) }));
    }
  };

  // Attaches the policy to / detaches it from the staged groups, in its own network.
  const applyGroups = async (p: Policy) => {
    if (!form.attachGroupIds.length && !form.detachGroupIds.length) return;
    const latest = await api.getGroups(p.network_id);
    const attached = new Set(latest.filter((g) => g.policy_ids?.includes(p.id)).map((g) => g.id));
    for (const id of form.detachGroupIds) {
      if (attached.has(id)) await api.detachPolicyFromGroup(p.network_id, id, p.id);
    }
    for (const id of form.attachGroupIds) {
      if (!attached.has(id)) await api.attachPolicyToGroup(p.network_id, id, p.id);
    }
  };

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ['policies'] });
    void queryClient.invalidateQueries({ queryKey: ['groups'] });
  };

  const handleSubmit = async () => {
    setSubmitError(null);
    if (!validate()) return;
    setSubmitting(true);
    const fields = { name: form.name.trim(), description: form.description.trim() };
    if (!current) {
      let created: Policy;
      try {
        created = await api.createPolicy(form.networkId, {
          ...fields,
          // The API takes rules without ids.
          rules: form.rules.map(ruleFields),
        });
      } catch (err) {
        setSubmitError(apiErrorMessage(err));
        setSubmitting(false);
        return;
      }
      // The rules are saved with the policy: a retry must not add them again.
      setForm((f) => ({ ...f, rules: (created.rules ?? []).map(formRule) }));
      try {
        await applyGroups(created);
      } catch (err) {
        // The policy exists: go on editing it rather than creating it again.
        setCurrent(created);
        setSubmitError(`The policy was created, but not attached to all its groups: ${apiErrorMessage(err)}`);
        setSubmitting(false);
        refresh();
        return;
      }
      toast.success(`Policy “${created.name}” created`);
    } else {
      try {
        if (fields.name !== current.name || fields.description !== (current.description ?? '')) {
          await api.updatePolicy(current.network_id, current.id, fields);
        }
        await applyRules(current);
        await applyGroups(current);
      } catch (err) {
        // Diff the next save against what was actually saved.
        const fresh = await api.getPolicy(current.network_id, current.id).catch(() => null);
        if (fresh) setCurrent(fresh);
        setSubmitError(apiErrorMessage(err));
        setSubmitting(false);
        refresh();
        return;
      }
      toast.success(`Policy “${fields.name}” saved`);
    }
    setSubmitting(false);
    refresh();
    onClose();
  };

  const networkName = network?.name;
  const needsNetwork = <p className="text-sm text-ink-muted">Choose the network of the policy first.</p>;

  const groupItems = groupIds.map((id) => {
    const g = groupById.get(id);
    const n = g?.peer_ids?.length ?? 0;
    return { id, label: g?.name ?? id, description: g && [`${n} peer${n === 1 ? '' : 's'}`, g.description].filter(Boolean).join(' · ') };
  });

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      size="lg"
      title={current ? `Edit policy ${current.name}` : 'New policy'}
      description={
        current
          ? `In network ${networkName ?? current.network_id}. Changes apply when you save.`
          : 'A policy is a list of firewall rules, given to the peers of the groups it is attached to.'
      }
      onSubmit={handleSubmit}
      submitLabel={current ? 'Save changes' : 'Create policy'}
      submitting={submitting}
      error={submitError}
    >
      <Tabs
        selectedIndex={tab}
        onChange={setTab}
        tabs={[
          {
            label: 'Details',
            alert: Boolean(errors.name || errors.networkId),
            content: (
              <>
                {!current && (
                  <Combobox
                    label="Network"
                    required
                    value={form.networkId}
                    onChange={(id) => {
                      if (id === form.networkId) return;
                      // Rules and groups belong to the network.
                      setForm((f) => ({ ...f, networkId: id, rules: [], attachGroupIds: [], detachGroupIds: [] }));
                      setDraft(blankDraft);
                      setDraftError(null);
                    }}
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
                  placeholder="allow-web"
                  error={errors.name}
                  autoFocus={!current}
                />
                <Textarea
                  label="Description"
                  value={form.description}
                  onChange={(e) => set('description', e.target.value)}
                  placeholder="What this policy allows or denies"
                />
                {current && (
                  <p className="text-xs text-ink-muted">
                    ID <span className="font-mono">{current.id}</span>
                  </p>
                )}
              </>
            ),
          },
          {
            label: 'Rules',
            count: form.rules.length,
            alert: Boolean(draftError),
            content: networkId ? (
              <RulesEditor
                rules={form.rules}
                onRulesChange={(rules) => set('rules', rules)}
                draft={draft}
                onDraftChange={setDraft}
                error={draftError}
                onErrorChange={setDraftError}
                network={network}
                peers={peers}
                groups={groups}
                routes={routes}
              />
            ) : (
              needsNetwork
            ),
          },
          {
            label: 'Groups',
            count: groupIds.length,
            content: networkId ? (
              <AttachList
                label="Attach to a group"
                addLabel="Search groups…"
                emptyText="Not attached to any group: the policy applies to no peer."
                hint="The policy applies to the peers of these groups."
                items={groupItems}
                available={groups
                  .filter((g) => !groupIds.includes(g.id))
                  .map((g) => ({ value: g.id, label: g.name, description: `${g.peer_ids?.length ?? 0} peers` }))}
                onAdd={attachGroup}
                onRemove={detachGroup}
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
