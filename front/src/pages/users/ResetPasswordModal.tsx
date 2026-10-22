import { useEffect, useState } from 'react';
import api from '../../api/client';
import { FormModal, PasswordInput, apiErrorMessage, useToast } from '../../components/ui';
import type { User } from '../../types';
import { MIN_PASSWORD_LENGTH } from './roles';

/** Sets a new password for a local user, which signs them out everywhere. */
export default function ResetPasswordModal({
  isOpen,
  onClose,
  user,
}: {
  isOpen: boolean;
  onClose: () => void;
  user: User | null;
}) {
  const toast = useToast();
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | undefined>();
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!isOpen) return;
    setPassword('');
    setError(undefined);
    setSubmitError(null);
  }, [isOpen, user]);

  if (!user) return null;

  const handleSubmit = async () => {
    setSubmitError(null);
    if (password.length < MIN_PASSWORD_LENGTH) {
      setError(`Use at least ${MIN_PASSWORD_LENGTH} characters.`);
      return;
    }
    setError(undefined);
    setSubmitting(true);
    try {
      await api.updateUser(user.id, { password });
    } catch (err) {
      setSubmitError(apiErrorMessage(err));
      setSubmitting(false);
      return;
    }
    setSubmitting(false);
    toast.success(`Password for “${user.name}” reset`);
    onClose();
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      size="sm"
      title={`Reset password for ${user.name}`}
      description={`${user.email} is signed out everywhere and must sign in again with the new password.`}
      onSubmit={handleSubmit}
      submitLabel="Reset password"
      submitting={submitting}
      error={submitError}
    >
      <PasswordInput
        label="New password"
        required
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        error={error}
        hint={`At least ${MIN_PASSWORD_LENGTH} characters.`}
        autoFocus
      />
    </FormModal>
  );
}
