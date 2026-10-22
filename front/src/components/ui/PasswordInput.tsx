import { forwardRef, useState } from 'react';
import { EyeIcon, EyeSlashIcon } from '@heroicons/react/20/solid';
import { IconButton } from './Button';
import { TextInput } from './Field';
import type { TextInputProps } from './Field';

/** A password field with a show/hide toggle. */
export const PasswordInput = forwardRef<HTMLInputElement, Omit<TextInputProps, 'type' | 'trailing'>>(function PasswordInput(
  { autoComplete = 'new-password', ...props },
  ref,
) {
  const [visible, setVisible] = useState(false);
  return (
    <TextInput
      ref={ref}
      type={visible ? 'text' : 'password'}
      autoComplete={autoComplete}
      trailing={({ id }) => (
        <IconButton label={visible ? 'Hide password' : 'Show password'} aria-controls={id} onClick={() => setVisible((v) => !v)}>
          {visible ? <EyeSlashIcon className="size-4" /> : <EyeIcon className="size-4" />}
        </IconButton>
      )}
      {...props}
    />
  );
});
