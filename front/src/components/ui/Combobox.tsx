import { useRef, useState } from 'react';
import type { ReactNode } from 'react';
import {
  Combobox as HeadlessCombobox,
  ComboboxButton,
  ComboboxInput,
  ComboboxOption,
  ComboboxOptions,
} from '@headlessui/react';
import { CheckIcon, ChevronUpDownIcon } from '@heroicons/react/20/solid';
import { Field } from './Field';
import { controlClass } from './styles';
import { cx } from './cx';

export interface ComboboxOption {
  value: string;
  label: string;
  /** Secondary text (address, CIDR…), also searched. */
  description?: string;
  disabled?: boolean;
}

export interface ComboboxProps {
  label: ReactNode;
  value: string;
  onChange: (value: string) => void;
  options: ComboboxOption[];
  placeholder?: string;
  hint?: ReactNode;
  error?: ReactNode;
  required?: boolean;
  disabled?: boolean;
  className?: string;
  /** Text when the search matches nothing. */
  emptyText?: string;
}

/** A searchable single-select, keyboard and screen-reader accessible. */
export function Combobox({
  label,
  value,
  onChange,
  options,
  placeholder = 'Select…',
  hint,
  error,
  required,
  disabled,
  className,
  emptyText = 'No match',
}: ComboboxProps) {
  const [query, setQuery] = useState('');
  const buttonRef = useRef<HTMLButtonElement>(null);
  const q = query.trim().toLowerCase();
  const filtered = q
    ? options.filter((o) => o.label.toLowerCase().includes(q) || o.description?.toLowerCase().includes(q))
    : options;
  const selected = options.find((o) => o.value === value) ?? null;

  return (
    <Field label={label} hint={hint} error={error} required={required} className={className}>
      {(control) => (
        <HeadlessCombobox
          value={selected}
          onChange={(o: ComboboxOption | null) => {
            onChange(o?.value ?? '');
            setQuery('');
          }}
          onClose={() => setQuery('')}
          disabled={disabled}
          immediate
        >
          {({ open }) => (
            <div className="relative">
              <ComboboxInput
                {...control}
                className={cx(controlClass, 'h-9 pr-9')}
                placeholder={placeholder}
                displayValue={(o: ComboboxOption | null) => o?.label ?? ''}
                onChange={(e) => setQuery(e.target.value)}
                // A focused field (e.g. right after a pick) opens on click too.
                onClick={() => !open && buttonRef.current?.click()}
                autoComplete="off"
              />
              <ComboboxButton
                ref={buttonRef}
                className="absolute inset-y-0 right-0 flex items-center px-2.5 text-ink-subtle"
                aria-label="Show options"
              >
                <ChevronUpDownIcon className="size-4" aria-hidden="true" />
              </ComboboxButton>
              <ComboboxOptions
                className={cx(
                  'absolute z-50 mt-1 max-h-64 w-full overflow-auto rounded-control border border-line bg-surface p-1 text-sm shadow-overlay',
                  'empty:invisible focus:outline-none',
                )}
              >
                {filtered.length === 0 && q ? (
                  <div className="px-2.5 py-2 text-ink-muted">{emptyText}</div>
                ) : (
                  filtered.map((o) => (
                    <ComboboxOption
                      key={o.value}
                      value={o}
                      disabled={o.disabled}
                      className="group flex cursor-default items-center gap-2 rounded-md px-2.5 py-1.5 text-ink select-none data-disabled:opacity-50 data-focus:bg-surface-hover"
                    >
                      <CheckIcon className="invisible size-4 shrink-0 text-primary-600 group-data-selected:visible" aria-hidden="true" />
                      <span className="min-w-0 flex-1 truncate">{o.label}</span>
                      {o.description && <span className="shrink-0 font-mono text-xs text-ink-muted">{o.description}</span>}
                    </ComboboxOption>
                  ))
                )}
              </ComboboxOptions>
            </div>
          )}
        </HeadlessCombobox>
      )}
    </Field>
  );
}
