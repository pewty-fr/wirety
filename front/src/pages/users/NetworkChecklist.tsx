import { useId } from 'react';
import type { ReactNode } from 'react';
import { Checkbox } from '../../components/ui';
import type { Network } from '../../types';

/** The networks a user (or new users) may use: one checkbox per network. */
export function NetworkChecklist({
  label,
  hint,
  networks,
  value,
  onChange,
}: {
  label: string;
  hint?: ReactNode;
  networks: Network[];
  /** Ids of the checked networks. */
  value: string[];
  onChange: (ids: string[]) => void;
}) {
  const hintId = useId();
  const toggle = (id: string) => onChange(value.includes(id) ? value.filter((x) => x !== id) : [...value, id]);
  const checked = networks.filter((n) => value.includes(n.id)).length;

  return (
    <fieldset className="min-w-0" aria-describedby={hintId}>
      <legend className="mb-1.5 text-sm font-medium text-ink">{label}</legend>
      {networks.length === 0 ? (
        <p className="rounded-control border border-dashed border-line-strong px-3 py-4 text-center text-sm text-ink-muted">
          No network yet.
        </p>
      ) : (
        <div className="max-h-56 divide-y divide-line overflow-y-auto rounded-control border border-line">
          {networks.map((n) => (
            <Checkbox
              key={n.id}
              className="px-3 py-2"
              label={n.name}
              description={<span className="font-mono">{[n.cidr, n.cidr_v6].filter(Boolean).join(' · ')}</span>}
              checked={value.includes(n.id)}
              onChange={() => toggle(n.id)}
            />
          ))}
        </div>
      )}
      <p id={hintId} className="mt-1.5 text-xs text-ink-muted">
        {networks.length > 0 && `${checked} of ${networks.length} selected. `}
        {hint}
      </p>
    </fieldset>
  );
}
