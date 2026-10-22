import { forwardRef, useId } from 'react';
import type { InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from 'react';
import { ChevronUpDownIcon } from '@heroicons/react/20/solid';
import { cx } from './cx';
import { controlClass } from './styles';

interface FieldControlProps {
  id: string;
  'aria-describedby'?: string;
  'aria-invalid'?: boolean;
}

export interface FieldProps {
  label: ReactNode;
  /** Help text under the control. */
  hint?: ReactNode;
  /** Error message; marks the control invalid. */
  error?: ReactNode;
  required?: boolean;
  className?: string;
  /** Renders the control, wired to the label, hint and error. */
  children: (control: FieldControlProps) => ReactNode;
}

/** Label + control + hint/error, with the accessibility wiring between them. */
export function Field({ label, hint, error, required, className, children }: FieldProps) {
  const id = useId();
  const hintId = `${id}-hint`;
  const errorId = `${id}-error`;
  const describedBy = [error ? errorId : null, hint ? hintId : null].filter(Boolean).join(' ') || undefined;
  return (
    <div className={cx('space-y-1.5', className)}>
      <label htmlFor={id} className="block text-sm font-medium text-ink">
        {label}
        {required && (
          <span className="ml-0.5 text-primary-600" aria-hidden="true">
            *
          </span>
        )}
      </label>
      {children({ id, 'aria-describedby': describedBy, 'aria-invalid': error ? true : undefined })}
      {error ? (
        <p id={errorId} className="text-xs font-medium text-red-600 dark:text-red-400">
          {error}
        </p>
      ) : (
        hint && (
          <p id={hintId} className="text-xs text-ink-muted">
            {hint}
          </p>
        )
      )}
    </div>
  );
}


type FieldOwnProps = Pick<FieldProps, 'label' | 'hint' | 'error' | 'className'>;

export type TextInputProps = FieldOwnProps & Omit<InputHTMLAttributes<HTMLInputElement>, 'className'> & {
  /** Monospace text, for addresses, CIDRs and keys. */
  mono?: boolean;
  /** An icon inside the field, on the left (e.g. a magnifier). */
  leading?: ReactNode;
  /** Content inside the field, on the right (e.g. a show/hide button); a function receives the input's id. */
  trailing?: ReactNode | ((input: { id: string }) => ReactNode);
};

export const TextInput = forwardRef<HTMLInputElement, TextInputProps>(function TextInput(
  { label, hint, error, className, required, mono, leading, trailing, ...props },
  ref,
) {
  return (
    <Field label={label} hint={hint} error={error} required={required} className={className}>
      {(control) => (
        <div className="relative">
          {leading && (
            <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-2.5 text-ink-subtle [&>svg]:size-4">
              {leading}
            </div>
          )}
          <input
            ref={ref}
            required={required}
            className={cx(controlClass, 'h-9', mono && 'font-mono text-[13px]', leading ? 'pl-8' : null, trailing ? 'pr-10' : null)}
            {...control}
            {...props}
          />
          {trailing && (
            <div className="absolute inset-y-0 right-0.5 flex items-center">
              {typeof trailing === 'function' ? trailing({ id: control.id }) : trailing}
            </div>
          )}
        </div>
      )}
    </Field>
  );
});

export type TextareaProps = FieldOwnProps & Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, 'className'>;

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { label, hint, error, className, required, rows = 3, ...props },
  ref,
) {
  return (
    <Field label={label} hint={hint} error={error} required={required} className={className}>
      {(control) => (
        <textarea
          ref={ref}
          rows={rows}
          required={required}
          className={cx(controlClass, 'py-2')}
          {...control}
          {...props}
        />
      )}
    </Field>
  );
});

export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export type SelectProps = FieldOwnProps &
  Omit<SelectHTMLAttributes<HTMLSelectElement>, 'className'> & {
    options: SelectOption[];
    /** First, empty option (e.g. "Select a network…"). */
    placeholder?: string;
  };

export const Select = forwardRef<HTMLSelectElement, SelectProps>(function Select(
  { label, hint, error, className, required, options, placeholder, ...props },
  ref,
) {
  return (
    <Field label={label} hint={hint} error={error} required={required} className={className}>
      {(control) => (
        <div className="relative">
          <select
            ref={ref}
            required={required}
            className={cx(controlClass, 'h-9 appearance-none pr-9')}
            {...control}
            {...props}
          >
            {placeholder !== undefined && <option value="">{placeholder}</option>}
            {options.map((o) => (
              <option key={o.value} value={o.value} disabled={o.disabled}>
                {o.label}
              </option>
            ))}
          </select>
          <ChevronUpDownIcon
            className="pointer-events-none absolute top-1/2 right-2.5 size-4 -translate-y-1/2 text-ink-subtle"
            aria-hidden="true"
          />
        </div>
      )}
    </Field>
  );
});

export interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label: ReactNode;
  description?: ReactNode;
}

export const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(function Checkbox(
  { label, description, className, ...props },
  ref,
) {
  const id = useId();
  return (
    <div className={cx('flex items-start gap-3', className)}>
      <input
        ref={ref}
        id={id}
        type="checkbox"
        aria-describedby={description ? `${id}-desc` : undefined}
        className="mt-0.5 size-4 shrink-0 rounded border-line-strong accent-primary-600 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500"
        {...props}
      />
      <div className="text-sm">
        <label htmlFor={id} className="font-medium text-ink">
          {label}
        </label>
        {description && (
          <p id={`${id}-desc`} className="text-xs text-ink-muted">
            {description}
          </p>
        )}
      </div>
    </div>
  );
});
