import type { FormEvent, ReactNode } from 'react';
import { Description, Dialog, DialogBackdrop, DialogPanel, DialogTitle } from '@headlessui/react';
import { XMarkIcon } from '@heroicons/react/20/solid';
import { Alert } from './Alert';
import { Button, IconButton } from './Button';
import { cx } from './cx';

export type ModalSize = 'sm' | 'md' | 'lg' | 'xl';

const sizes: Record<ModalSize, string> = {
  sm: 'max-w-md',
  md: 'max-w-xl',
  lg: 'max-w-3xl',
  xl: 'max-w-5xl',
};

export interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  title: ReactNode;
  description?: ReactNode;
  size?: ModalSize;
  children: ReactNode;
}

/**
 * Dialog shell: header (title, description, close), then the children —
 * usually a ModalBody and a ModalFooter. Focus is trapped inside, Escape and
 * a click on the backdrop close it.
 */
export function Modal({ isOpen, onClose, title, description, size = 'md', children }: ModalProps) {
  return (
    <Dialog open={isOpen} onClose={onClose} className="relative z-50">
      <DialogBackdrop
        transition
        className="fixed inset-0 bg-gray-950/40 backdrop-blur-[2px] transition duration-150 ease-out data-closed:opacity-0 dark:bg-black/60"
      />
      <div className="fixed inset-0 overflow-y-auto">
        <div className="flex min-h-full items-start justify-center p-4 sm:items-center sm:p-6">
          <DialogPanel
            transition
            className={cx(
              'w-full rounded-panel border border-line bg-surface text-left shadow-overlay',
              'transition duration-200 ease-out data-closed:translate-y-2 data-closed:scale-[0.98] data-closed:opacity-0',
              sizes[size],
            )}
          >
            <div className="flex items-start justify-between gap-4 border-b border-line px-5 py-4">
              <div className="min-w-0">
                <DialogTitle className="text-base font-semibold text-ink">{title}</DialogTitle>
                {description && <Description className="mt-1 text-sm text-ink-muted">{description}</Description>}
              </div>
              <IconButton label="Close" onClick={onClose} className="-mt-1 -mr-2">
                <XMarkIcon className="size-5" />
              </IconButton>
            </div>
            {children}
          </DialogPanel>
        </div>
      </div>
    </Dialog>
  );
}

export function ModalBody({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cx('space-y-4 px-5 py-5', className)}>{children}</div>;
}

export function ModalFooter({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={cx(
        'flex flex-col-reverse gap-2 rounded-b-panel border-t border-line bg-surface-muted/60 px-5 py-3.5 sm:flex-row sm:items-center sm:justify-end',
        className,
      )}
    >
      {children}
    </div>
  );
}

export interface FormModalProps extends Omit<ModalProps, 'children'> {
  onSubmit: () => void;
  /** Label of the submit button, e.g. "Create group". */
  submitLabel: string;
  submitting?: boolean;
  submitDisabled?: boolean;
  /** Shown above the fields (e.g. the API error). */
  error?: ReactNode;
  /** Red submit button, for destructive actions. */
  destructive?: boolean;
  cancelLabel?: string;
  /** Extra content on the left of the footer (e.g. a delete button). */
  footerStart?: ReactNode;
  children: ReactNode;
}

/**
 * The standard create/edit dialog: fields, an error banner, and a footer with
 * Cancel and the submit button (disabled with a spinner while submitting).
 */
export function FormModal({
  onSubmit,
  submitLabel,
  submitting = false,
  submitDisabled = false,
  error,
  destructive = false,
  cancelLabel = 'Cancel',
  footerStart,
  children,
  onClose,
  ...modal
}: FormModalProps) {
  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (!submitting) onSubmit();
  };
  return (
    <Modal onClose={onClose} {...modal}>
      <form onSubmit={handleSubmit} noValidate>
        <ModalBody>
          {error && <Alert tone="danger">{error}</Alert>}
          {children}
        </ModalBody>
        <ModalFooter>
          {footerStart && <div className="mr-auto">{footerStart}</div>}
          <Button variant="secondary" onClick={onClose} disabled={submitting}>
            {cancelLabel}
          </Button>
          <Button type="submit" variant={destructive ? 'danger' : 'primary'} loading={submitting} disabled={submitDisabled}>
            {submitLabel}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}
