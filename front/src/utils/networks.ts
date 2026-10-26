import type { ComboboxOption } from '../components/ui';
import type { Network } from '../types';

/** A network as a picker option: its name, and its CIDRs as description. */
export function networkOption(n: Network): ComboboxOption {
  return { value: n.id, label: n.name, description: [n.cidr, n.cidr_v6].filter(Boolean).join(' · ') };
}
