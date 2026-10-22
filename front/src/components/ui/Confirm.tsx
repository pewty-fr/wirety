import { createContext, useCallback, useContext, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { ExclamationTriangleIcon } from '@heroicons/react/24/outline';
import { Button } from './Button';
import { Modal, ModalBody, ModalFooter } from './Modal';

export interface ConfirmOptions {
  title: ReactNode;
  message?: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  /** Red confirm button and warning icon (deletions, revocations). */
  destructive?: boolean;
}

type ConfirmFn = (options: ConfirmOptions) => Promise<boolean>;

const ConfirmContext = createContext<ConfirmFn | null>(null);

/** Provides useConfirm(): an in-app replacement for window.confirm. */
export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [options, setOptions] = useState<ConfirmOptions | null>(null);
  const [open, setOpen] = useState(false);
  const resolver = useRef<((value: boolean) => void) | null>(null);

  const confirm = useCallback<ConfirmFn>((opts) => {
    resolver.current?.(false);
    setOptions(opts);
    setOpen(true);
    return new Promise<boolean>((resolve) => {
      resolver.current = resolve;
    });
  }, []);

  const settle = (value: boolean) => {
    resolver.current?.(value);
    resolver.current = null;
    setOpen(false);
  };

  return (
    <ConfirmContext.Provider value={confirm}>
      {children}
      {options && (
        <Modal isOpen={open} onClose={() => settle(false)} title={options.title} size="sm">
          <ModalBody className="flex gap-3">
            {options.destructive && (
              <div className="flex size-9 shrink-0 items-center justify-center rounded-full bg-red-50 dark:bg-red-500/10">
                <ExclamationTriangleIcon className="size-5 text-red-600 dark:text-red-400" aria-hidden="true" />
              </div>
            )}
            {options.message && <div className="text-sm text-ink-secondary">{options.message}</div>}
          </ModalBody>
          <ModalFooter>
            <Button variant="secondary" onClick={() => settle(false)}>
              {options.cancelLabel ?? 'Cancel'}
            </Button>
            <Button variant={options.destructive ? 'danger' : 'primary'} onClick={() => settle(true)}>
              {options.confirmLabel ?? 'Confirm'}
            </Button>
          </ModalFooter>
        </Modal>
      )}
    </ConfirmContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useConfirm(): ConfirmFn {
  const confirm = useContext(ConfirmContext);
  if (!confirm) throw new Error('useConfirm must be used inside <ConfirmProvider>');
  return confirm;
}
