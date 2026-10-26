import type { ComponentType, ReactNode, SVGProps } from 'react';
import { cx } from '../../components/ui';

/**
 * Frame of the screens shown outside the dashboard (sign-in, captive portal):
 * the Wirety brand on top, the content in a centred column on the canvas.
 */
export default function AuthShell({
  children,
  footer,
  wide = false,
}: {
  children: ReactNode;
  /** Small print under the content. */
  footer?: ReactNode;
  /** A wider column, for content with details to read. */
  wide?: boolean;
}) {
  return (
    <div className="flex min-h-screen flex-col items-center bg-canvas px-4 py-10 sm:justify-center sm:py-16">
      <div className={cx('w-full animate-pop-in', wide ? 'max-w-lg' : 'max-w-sm')}>
        <div className="mb-6 flex items-center justify-center gap-2.5">
          <img src="/logo.svg" alt="" className="size-9 rounded-lg shadow-sm" />
          <span className="text-xl font-semibold tracking-tight text-ink">Wirety</span>
        </div>
        {children}
        {footer && <div className="mt-6 text-center text-xs text-ink-subtle">{footer}</div>}
      </div>
    </div>
  );
}

export type StatusTone = 'neutral' | 'success' | 'warning' | 'danger';

const statusTones: Record<StatusTone, string> = {
  neutral: 'bg-surface-muted text-ink-muted',
  success: 'bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-400',
  warning: 'bg-amber-50 text-amber-600 dark:bg-amber-500/10 dark:text-amber-400',
  danger: 'bg-red-50 text-red-600 dark:bg-red-500/10 dark:text-red-400',
};

/** A round, tinted icon heading a status screen. */
export function StatusIcon({ icon: Icon, tone = 'neutral' }: { icon: ComponentType<SVGProps<SVGSVGElement>>; tone?: StatusTone }) {
  return (
    <div className={cx('mx-auto flex size-11 items-center justify-center rounded-full', statusTones[tone])}>
      <Icon className="size-6" aria-hidden="true" />
    </div>
  );
}
