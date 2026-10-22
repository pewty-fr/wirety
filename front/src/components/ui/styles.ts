import { cx } from './cx';

/** Classes of text inputs, selects and comboboxes. */
export const controlClass = cx(
  'block w-full rounded-control border border-line-strong bg-surface px-3 text-sm text-ink shadow-sm',
  'placeholder:text-ink-subtle transition-colors',
  'focus:border-primary-500 focus:outline-none focus:ring-3 focus:ring-primary-500/20',
  'disabled:cursor-not-allowed disabled:bg-surface-muted disabled:text-ink-muted',
  'aria-invalid:border-red-500 aria-invalid:focus:ring-red-500/20',
);

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger';
export type ButtonSize = 'sm' | 'md';

const variants: Record<ButtonVariant, string> = {
  primary:
    'bg-primary-600 text-white shadow-sm hover:bg-primary-700 focus-visible:outline-primary-500 disabled:bg-primary-600/50',
  secondary:
    'bg-surface text-ink border border-line-strong shadow-sm hover:bg-surface-hover focus-visible:outline-primary-500',
  ghost: 'text-ink-secondary hover:bg-surface-hover hover:text-ink focus-visible:outline-primary-500',
  danger: 'bg-red-600 text-white shadow-sm hover:bg-red-700 focus-visible:outline-red-500 disabled:bg-red-600/50',
};

const sizes: Record<ButtonSize, string> = {
  sm: 'h-8 px-3 text-[13px] gap-1.5',
  md: 'h-9 px-3.5 text-sm gap-2',
};

/** Button classes, for links styled as buttons. */
export function buttonClass({
  variant = 'primary',
  size = 'md',
  className,
}: { variant?: ButtonVariant; size?: ButtonSize; className?: string } = {}) {
  return cx(
    'inline-flex shrink-0 items-center justify-center rounded-control font-medium whitespace-nowrap transition-colors',
    'focus-visible:outline-2 focus-visible:outline-offset-2 disabled:cursor-not-allowed disabled:opacity-60',
    variants[variant],
    sizes[size],
    className,
  );
}
