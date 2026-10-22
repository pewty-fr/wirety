import type { Group, Network, Peer, PolicyRule, Route } from '../../types';
import { getCIDRError, isValidCIDR } from '../../utils/validation';

/** A rule as the API takes it. */
export type RuleFields = Omit<PolicyRule, 'id'>;

/** A rule of the policy form: saved (it has its server id) or added in the form. */
export interface FormRule extends RuleFields {
  id?: string;
  /** Stable list key. */
  key: string;
}

/**
 * What a rule can target in the editor. 'network' and 'route' are shortcuts
 * that pick a CIDR: they are sent as 'cidr'.
 */
export type TargetKind = 'network' | 'cidr' | 'peer' | 'group' | 'route';

/** The rule being written in the editor, before it is added to the list. */
export interface RuleDraft {
  direction: PolicyRule['direction'];
  action: PolicyRule['action'];
  kind: TargetKind;
  /** A CIDR (network, cidr), a peer or group id, or a route option value (route). */
  target: string;
  description: string;
}

export const blankDraft: RuleDraft = { direction: 'output', action: 'allow', kind: 'cidr', target: '', description: '' };

let seq = 0;
export const newRuleKey = () => `new-rule-${++seq}`;

export const formRule = (rule: PolicyRule): FormRule => ({ ...rule, key: rule.id });

/** The API payload of a rule. */
export const ruleFields = ({ direction, action, target_type, target, description }: RuleFields): RuleFields => ({
  direction,
  action,
  target_type,
  target,
  description,
});

/** The IPv4 and/or IPv6 ranges of a network. */
export const networkCidrs = (n: Network | undefined): string[] =>
  [n?.cidr, n?.cidr_v6].filter((c): c is string => Boolean(c));

export const family = (cidr: string) => (cidr.includes(':') ? 'IPv6' : 'IPv4');

export interface RouteTarget {
  value: string;
  label: string;
  description: string;
  cidr: string;
}

/** The destinations of the routes, one option per route and address family. */
export function routeTargets(routes: Route[]): RouteTarget[] {
  return routes.flatMap((r) => {
    const cidrs = [r.destination_cidr, r.destination_cidr_v6].filter((c): c is string => Boolean(c));
    return cidrs.map((cidr) => ({
      value: `${r.id}:${family(cidr)}`,
      label: cidrs.length > 1 ? `${r.name} (${family(cidr)})` : r.name,
      description: cidr,
      cidr,
    }));
  });
}

/** True when the editor holds a rule the user started but has not added. */
export const isPending = (d: RuleDraft) => (d.kind !== 'network' && d.target.trim() !== '') || d.description.trim() !== '';

const missingTarget: Record<TargetKind, string> = {
  network: 'This network has no address range.',
  cidr: 'Enter a CIDR, e.g. 10.0.0.0/24.',
  peer: 'Choose a peer.',
  group: 'Choose a group.',
  route: 'Choose a route.',
};

/** The rule written in the editor, or the error that prevents adding it. */
export function ruleFromDraft(d: RuleDraft, routes: RouteTarget[], existing: RuleFields[]): RuleFields | string {
  const target = d.kind === 'route' ? (routes.find((r) => r.value === d.target)?.cidr ?? '') : d.target.trim();
  if (!target) return missingTarget[d.kind];
  if (d.kind === 'cidr' && !isValidCIDR(target)) {
    return getCIDRError(target) ?? 'Enter a valid CIDR, e.g. 10.0.0.0/24.';
  }
  const rule: RuleFields = {
    direction: d.direction,
    action: d.action,
    target_type: d.kind === 'peer' || d.kind === 'group' ? d.kind : 'cidr',
    target,
    description: d.description.trim(),
  };
  const duplicate = existing.some(
    (r) => r.direction === rule.direction && r.action === rule.action && r.target_type === rule.target_type && r.target === rule.target,
  );
  return duplicate ? 'This rule is already in the list.' : rule;
}

export interface TargetView {
  type: string;
  value: string;
  /** Show the value in monospace (addresses, unknown ids). */
  mono: boolean;
  detail?: string;
}

/** How to show the target of a rule: names instead of ids, and what a CIDR stands for. */
export function describeTarget(
  rule: RuleFields,
  ctx: { network?: Network; peers: Map<string, Peer>; groups: Map<string, Group>; routes: Route[] },
): TargetView {
  switch (rule.target_type) {
    case 'peer': {
      const p = ctx.peers.get(rule.target);
      return {
        type: 'Peer',
        value: p?.name ?? rule.target,
        mono: !p,
        detail: p && [p.address, p.address_v6].filter(Boolean).join(' · '),
      };
    }
    case 'group': {
      const g = ctx.groups.get(rule.target);
      return { type: 'Group', value: g?.name ?? rule.target, mono: !g };
    }
    case 'cidr': {
      const route = ctx.routes.find((r) => r.destination_cidr === rule.target || r.destination_cidr_v6 === rule.target);
      const detail = networkCidrs(ctx.network).includes(rule.target)
        ? `network ${ctx.network!.name}`
        : route && `route ${route.name}`;
      return { type: 'CIDR', value: rule.target, mono: true, detail };
    }
    default:
      return { type: rule.target_type, value: rule.target, mono: true };
  }
}
