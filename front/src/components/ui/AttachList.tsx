import type { ReactNode } from 'react';
import { ArrowDownIcon, ArrowUpIcon, XMarkIcon } from '@heroicons/react/20/solid';
import { Combobox } from './Combobox';
import type { ComboboxOption } from './Combobox';
import { IconButton } from './Button';

export interface AttachItem {
  id: string;
  label: string;
  /** Secondary text (address, CIDR, description). */
  description?: ReactNode;
  /** Badge or extra content on the right of the label. */
  meta?: ReactNode;
}

/**
 * The items attached to something (peers of a group, policies of a group…),
 * with a picker to add one and a button to remove each. With `onMove`, the
 * order matters and each row can move up or down.
 */
export function AttachList({
  label,
  items,
  available,
  onAdd,
  onRemove,
  onMove,
  addLabel,
  emptyText,
  hint,
}: {
  /** Name of the picker field, e.g. "Add a peer". */
  label: string;
  items: AttachItem[];
  /** What can still be attached. */
  available: ComboboxOption[];
  onAdd: (id: string) => void;
  onRemove: (id: string) => void;
  onMove?: (id: string, direction: -1 | 1) => void;
  /** Placeholder of the picker. */
  addLabel: string;
  emptyText: string;
  hint?: ReactNode;
}) {
  return (
    <div className="space-y-3">
      {items.length === 0 ? (
        <p className="rounded-control border border-dashed border-line-strong px-3 py-4 text-center text-sm text-ink-muted">
          {emptyText}
        </p>
      ) : (
        <ul className="divide-y divide-line rounded-control border border-line" aria-label={label.replace(/^Add an? /i, '')}>
          {items.map((item, index) => (
            <li key={item.id} className="flex items-center gap-3 px-3 py-2">
              {onMove && (
                <span className="w-5 shrink-0 text-center text-xs font-medium text-ink-subtle tabular-nums">{index + 1}</span>
              )}
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="truncate text-sm font-medium text-ink">{item.label}</span>
                  {item.meta}
                </div>
                {item.description && <div className="truncate text-xs text-ink-muted">{item.description}</div>}
              </div>
              {onMove && (
                <>
                  <IconButton label={`Move ${item.label} up`} disabled={index === 0} onClick={() => onMove(item.id, -1)}>
                    <ArrowUpIcon className="size-4" />
                  </IconButton>
                  <IconButton
                    label={`Move ${item.label} down`}
                    disabled={index === items.length - 1}
                    onClick={() => onMove(item.id, 1)}
                  >
                    <ArrowDownIcon className="size-4" />
                  </IconButton>
                </>
              )}
              <IconButton label={`Remove ${item.label}`} tone="danger" onClick={() => onRemove(item.id)}>
                <XMarkIcon className="size-4" />
              </IconButton>
            </li>
          ))}
        </ul>
      )}
      <Combobox
        label={label}
        value=""
        onChange={(id) => id && onAdd(id)}
        options={available}
        placeholder={available.length ? addLabel : 'Nothing left to add'}
        disabled={available.length === 0}
        hint={hint}
      />
    </div>
  );
}
