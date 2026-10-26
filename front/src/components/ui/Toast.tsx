import { createContext, useCallback, useContext, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { CheckCircleIcon, XCircleIcon, XMarkIcon } from '@heroicons/react/20/solid';
import { cx } from './cx';

type ToastTone = 'success' | 'error';

interface ToastItem {
  id: number;
  tone: ToastTone;
  message: ReactNode;
}

interface ToastApi {
  success: (message: ReactNode) => void;
  error: (message: ReactNode) => void;
}

const ToastContext = createContext<ToastApi | null>(null);

const DURATION_MS = 4500;

/** Provides useToast(): short success/error notices, announced to screen readers. */
export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const dismiss = useCallback((id: number) => setToasts((all) => all.filter((t) => t.id !== id)), []);

  const push = useCallback(
    (tone: ToastTone, message: ReactNode) => {
      const id = Date.now() + Math.random();
      setToasts((all) => [...all.slice(-3), { id, tone, message }]);
      setTimeout(() => dismiss(id), DURATION_MS);
    },
    [dismiss],
  );

  const api = useMemo<ToastApi>(
    () => ({ success: (m) => push('success', m), error: (m) => push('error', m) }),
    [push],
  );

  return (
    <ToastContext.Provider value={api}>
      {children}
      <div aria-live="polite" className="pointer-events-none fixed inset-x-0 bottom-4 z-[60] flex flex-col items-center gap-2 px-4 sm:items-end sm:px-6">
        {toasts.map((t) => (
          <div
            key={t.id}
            role={t.tone === 'error' ? 'alert' : 'status'}
            className={cx(
              'pointer-events-auto flex w-full max-w-sm animate-slide-in items-start gap-2.5 rounded-panel border border-line bg-surface px-3.5 py-3 text-sm text-ink shadow-overlay',
            )}
          >
            {t.tone === 'success' ? (
              <CheckCircleIcon className="mt-0.5 size-4 shrink-0 text-emerald-500" aria-hidden="true" />
            ) : (
              <XCircleIcon className="mt-0.5 size-4 shrink-0 text-red-500" aria-hidden="true" />
            )}
            <div className="min-w-0 flex-1">{t.message}</div>
            <button
              type="button"
              onClick={() => dismiss(t.id)}
              aria-label="Dismiss"
              className="-m-1 rounded p-1 text-ink-subtle hover:text-ink"
            >
              <XMarkIcon className="size-4" />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useToast(): ToastApi {
  const toast = useContext(ToastContext);
  if (!toast) throw new Error('useToast must be used inside <ToastProvider>');
  return toast;
}
