import { useId, useMemo } from 'react';
import type { KeyboardEvent, ReactNode } from 'react';
import { PlusIcon, XMarkIcon } from '@heroicons/react/20/solid';
import { Badge, Button, Combobox, IconButton, Select, TextInput, cx } from '../../components/ui';
import type { SelectOption } from '../../components/ui';
import type { Group, Network, Peer, Route } from '../../types';
import { describeTarget, family, isPending, networkCidrs, newRuleKey, routeTargets, ruleFromDraft } from './rules';
import type { FormRule, RuleDraft, TargetKind } from './rules';

const directions: SelectOption[] = [
  { value: 'output', label: 'Output — from the peers to the target' },
  { value: 'input', label: 'Input — from the target to the peers' },
];

const actions: SelectOption[] = [
  { value: 'allow', label: 'Allow' },
  { value: 'deny', label: 'Deny' },
];

const kinds: SelectOption[] = [
  { value: 'network', label: 'Network' },
  { value: 'cidr', label: 'CIDR' },
  { value: 'peer', label: 'Peer' },
  { value: 'group', label: 'Group' },
  { value: 'route', label: 'Route' },
];

const hints: Record<TargetKind, string> = {
  network: 'The address range of the network, saved as a CIDR rule.',
  cidr: 'An IPv4 or IPv6 range, e.g. 10.0.0.0/24 or fd00::/64.',
  peer: 'The addresses of this peer.',
  group: 'The peers of this group.',
  route: 'The destination of the route, saved as a CIDR rule.',
};

/**
 * The rules of a policy, in order, and an inline form to add one. Rules are
 * staged in the parent form (with the draft, so that it survives tab
 * switches) and saved with it.
 */
export default function RulesEditor({
  rules,
  onRulesChange,
  draft,
  onDraftChange,
  error,
  onErrorChange,
  network,
  peers,
  groups,
  routes,
}: {
  rules: FormRule[];
  onRulesChange: (rules: FormRule[]) => void;
  draft: RuleDraft;
  onDraftChange: (draft: RuleDraft) => void;
  /** Why the draft cannot be added (shown under the target). */
  error: string | null;
  onErrorChange: (error: string | null) => void;
  network?: Network;
  peers: Peer[];
  groups: Group[];
  routes: Route[];
}) {
  const headingId = useId();
  const cidrs = networkCidrs(network);
  const routeOptions = useMemo(() => routeTargets(routes), [routes]);
  const peerById = useMemo(() => new Map(peers.map((p) => [p.id, p])), [peers]);
  const groupById = useMemo(() => new Map(groups.map((g) => [g.id, g])), [groups]);

  const update = (patch: Partial<RuleDraft>) => {
    onDraftChange({ ...draft, ...patch });
    onErrorChange(null);
  };
  // Keeps the direction, action and target type for the next rule.
  const cleared = (d: RuleDraft): RuleDraft => ({ ...d, target: d.kind === 'network' ? (cidrs[0] ?? '') : '', description: '' });
  const clear = () => {
    onDraftChange(cleared(draft));
    onErrorChange(null);
  };

  const add = () => {
    const rule = ruleFromDraft(draft, routeOptions, rules);
    if (typeof rule === 'string') {
      onErrorChange(rule);
      return;
    }
    onRulesChange([...rules, { ...rule, key: newRuleKey() }]);
    clear();
  };

  // Enter adds the rule rather than submitting the whole policy.
  const addOnEnter = (e: KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      add();
    }
  };

  const targetProps = { label: 'Target', hint: hints[draft.kind], error };
  let target: ReactNode;
  switch (draft.kind) {
    case 'network':
      target = (
        <Select
          {...targetProps}
          value={draft.target}
          onChange={(e) => update({ target: e.target.value })}
          options={cidrs.map((c) => ({ value: c, label: `${c} (${family(c)})` }))}
        />
      );
      break;
    case 'cidr':
      target = (
        <TextInput
          {...targetProps}
          mono
          value={draft.target}
          onChange={(e) => update({ target: e.target.value })}
          onKeyDown={addOnEnter}
          placeholder="10.0.0.0/24"
          autoComplete="off"
        />
      );
      break;
    case 'peer':
      target = (
        <Combobox
          {...targetProps}
          value={draft.target}
          onChange={(target) => update({ target })}
          options={peers.map((p) => ({ value: p.id, label: p.name, description: p.address }))}
          placeholder="Search peers…"
        />
      );
      break;
    case 'group':
      target = (
        <Combobox
          {...targetProps}
          value={draft.target}
          onChange={(target) => update({ target })}
          options={groups.map((g) => {
            const n = g.peer_ids?.length ?? 0;
            return { value: g.id, label: g.name, description: `${n} peer${n === 1 ? '' : 's'}` };
          })}
          placeholder="Search groups…"
        />
      );
      break;
    case 'route':
      target = (
        <Combobox
          {...targetProps}
          value={draft.target}
          onChange={(target) => update({ target })}
          options={routeOptions}
          placeholder="Search routes…"
        />
      );
      break;
  }

  return (
    <div className="space-y-5">
      <div className="space-y-3">
        <p className="text-sm text-ink-muted">
          Output rules match traffic from the peers of the policy's groups to the target; input rules, traffic from the
          target to them.
        </p>
        {rules.length === 0 ? (
          <p className="rounded-control border border-dashed border-line-strong px-3 py-4 text-center text-sm text-ink-muted">
            No rule yet: add one below.
          </p>
        ) : (
          <ul aria-label="Rules" className="divide-y divide-line rounded-control border border-line">
            {rules.map((rule, index) => {
              const t = describeTarget(rule, { network, peers: peerById, groups: groupById, routes });
              return (
                <li key={rule.key} className="flex items-center gap-3 px-3 py-2">
                  <span className="w-5 shrink-0 text-center text-xs font-medium text-ink-subtle tabular-nums">{index + 1}</span>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                      <Badge tone={rule.action === 'allow' ? 'success' : 'danger'}>{rule.action === 'allow' ? 'Allow' : 'Deny'}</Badge>
                      <Badge tone="neutral">{rule.direction === 'output' ? 'Output' : 'Input'}</Badge>
                      <span className="text-xs text-ink-muted">{t.type}</span>
                      <span className={cx('truncate text-sm font-medium text-ink', t.mono && 'font-mono text-[13px]')}>{t.value}</span>
                      {t.detail && <span className="truncate text-xs text-ink-muted">{t.detail}</span>}
                    </div>
                    {rule.description && <p className="mt-0.5 truncate text-xs text-ink-muted">{rule.description}</p>}
                  </div>
                  <IconButton
                    label={`Remove rule ${index + 1}`}
                    tone="danger"
                    onClick={() => onRulesChange(rules.filter((r) => r.key !== rule.key))}
                  >
                    <XMarkIcon className="size-4" />
                  </IconButton>
                </li>
              );
            })}
          </ul>
        )}
      </div>

      <div role="group" aria-labelledby={headingId} className="space-y-4 rounded-control border border-line bg-surface-muted/40 p-4">
        <h3 id={headingId} className="text-sm font-semibold text-ink">
          Add a rule
        </h3>
        <div className="grid gap-4 sm:grid-cols-[1fr_8rem_8rem]">
          <Select
            label="Direction"
            value={draft.direction}
            onChange={(e) => update({ direction: e.target.value as RuleDraft['direction'] })}
            options={directions}
          />
          <Select
            label="Action"
            value={draft.action}
            onChange={(e) => update({ action: e.target.value as RuleDraft['action'] })}
            options={actions}
          />
          <Select
            label="Target type"
            value={draft.kind}
            onChange={(e) => {
              const kind = e.target.value as TargetKind;
              update({ kind, target: kind === 'network' ? (cidrs[0] ?? '') : '' });
            }}
            options={kinds}
          />
        </div>
        {target}
        <TextInput
          label="Description"
          value={draft.description}
          onChange={(e) => update({ description: e.target.value })}
          onKeyDown={addOnEnter}
          placeholder="Why this rule exists"
        />
        <div className="flex justify-end gap-2">
          {isPending(draft) && (
            <Button variant="ghost" size="sm" onClick={clear}>
              Clear
            </Button>
          )}
          <Button variant="secondary" size="sm" icon={<PlusIcon className="size-4" />} onClick={add}>
            Add rule
          </Button>
        </div>
      </div>
    </div>
  );
}
