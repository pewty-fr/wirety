import type { ReactNode, TdHTMLAttributes, ThHTMLAttributes } from 'react';
import { cx } from './cx';

/** A bordered, horizontally scrollable table. */
export function Table({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cx('overflow-x-auto rounded-panel border border-line bg-surface shadow-panel', className)}>
      <table className="min-w-full divide-y divide-line text-sm">{children}</table>
    </div>
  );
}

export function THead({ children }: { children: ReactNode }) {
  return <thead className="bg-surface-muted/60">{children}</thead>;
}

export function TBody({ children }: { children: ReactNode }) {
  return <tbody className="divide-y divide-line">{children}</tbody>;
}

export function TH({ className, children, ...props }: ThHTMLAttributes<HTMLTableCellElement>) {
  return (
    <th
      scope="col"
      className={cx('px-4 py-2.5 text-left text-xs font-medium whitespace-nowrap text-ink-muted', className)}
      {...props}
    >
      {children}
    </th>
  );
}

export function TR({ children, className, onClick }: { children: ReactNode; className?: string; onClick?: () => void }) {
  return (
    <tr
      onClick={onClick}
      className={cx('transition-colors hover:bg-surface-hover/60', onClick && 'cursor-pointer', className)}
    >
      {children}
    </tr>
  );
}

export function TD({ className, children, mono, ...props }: TdHTMLAttributes<HTMLTableCellElement> & { mono?: boolean }) {
  return (
    <td
      className={cx('px-4 py-3 whitespace-nowrap text-ink-secondary', mono && 'font-mono text-[13px]', className)}
      {...props}
    >
      {children}
    </td>
  );
}
