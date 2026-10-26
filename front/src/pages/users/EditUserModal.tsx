import { useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { FormModal, Select, TextInput, apiErrorMessage, useToast } from '../../components/ui';
import { useAuth } from '../../contexts/AuthContext';
import { useNetworks } from '../../hooks/useQueries';
import type { User } from '../../types';
import { formatDateTime } from '../../utils/dates';
import { NetworkChecklist } from './NetworkChecklist';
import { BOOTSTRAP_ADMIN_ID, roleHint, roleOptions } from './roles';
import type { Role } from './roles';

interface UserForm {
  name: string;
  role: Role;
  networkIds: string[];
}

const formFrom = (user: User | null): UserForm => ({
  name: user?.name ?? '',
  role: user?.role ?? 'user',
  networkIds: user?.authorized_networks ?? [],
});

/**
 * A user's details, and their name, role and authorized networks, saved
 * together.
 */
export default function EditUserModal({
  isOpen,
  onClose,
  user,
}: {
  isOpen: boolean;
  onClose: () => void;
  user: User | null;
}) {
  const { user: me, refreshUser } = useAuth();
  const toast = useToast();
  const queryClient = useQueryClient();
  const { data: networks = [] } = useNetworks();
  const [form, setForm] = useState<UserForm>(() => formFrom(user));
  const [nameError, setNameError] = useState<string | undefined>();
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // Each opening starts from the user as saved.
  useEffect(() => {
    if (!isOpen) return;
    setForm(formFrom(user));
    setNameError(undefined);
    setSubmitError(null);
  }, [isOpen, user]);

  if (!user) return null;

  const set = <K extends keyof UserForm>(key: K, value: UserForm[K]) => setForm((f) => ({ ...f, [key]: value }));
  // Administrators cannot demote themselves (they would lose this page), and
  // the bootstrap admin's role comes from the server's configuration.
  const roleLocked = user.id === me?.id || user.id === BOOTSTRAP_ADMIN_ID;

  const handleSubmit = async () => {
    setSubmitError(null);
    const name = form.name.trim();
    if (!name) {
      setNameError('Give the user a name.');
      return;
    }
    setNameError(undefined);
    setSubmitting(true);
    try {
      await api.updateUser(user.id, {
        name,
        role: form.role,
        authorized_networks: form.role === 'administrator' ? [] : form.networkIds,
      });
    } catch (err) {
      setSubmitError(apiErrorMessage(err));
      setSubmitting(false);
      return;
    }
    setSubmitting(false);
    toast.success(`User “${name}” saved`);
    void queryClient.invalidateQueries({ queryKey: ['users'] });
    // The sidebar and the profile show the signed-in user.
    if (user.id === me?.id) void refreshUser();
    onClose();
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={`Edit user ${user.name}`}
      description={user.email}
      onSubmit={handleSubmit}
      submitLabel="Save changes"
      submitting={submitting}
      error={submitError}
    >
      <dl className="grid grid-cols-2 gap-x-4 gap-y-3 rounded-control border border-line bg-surface-muted/60 px-4 py-3 text-sm">
        <div>
          <dt className="text-xs text-ink-muted">Last sign-in</dt>
          <dd className="mt-0.5 text-ink">{formatDateTime(user.last_login_at) ?? 'Never'}</dd>
        </div>
        <div>
          <dt className="text-xs text-ink-muted">Created</dt>
          <dd className="mt-0.5 text-ink">{formatDateTime(user.created_at) ?? '—'}</dd>
        </div>
        <div className="col-span-2">
          <dt className="text-xs text-ink-muted">User ID</dt>
          <dd className="mt-0.5 font-mono text-xs break-all text-ink-secondary">{user.id}</dd>
        </div>
      </dl>
      <TextInput
        label="Name"
        required
        value={form.name}
        onChange={(e) => set('name', e.target.value)}
        error={nameError}
        autoComplete="off"
      />
      <Select
        label="Role"
        value={form.role}
        onChange={(e) => set('role', e.target.value as Role)}
        options={roleOptions}
        disabled={roleLocked}
        hint={
          roleLocked
            ? user.id === me?.id
              ? 'You cannot change your own role.'
              : 'The role of the built-in administrator cannot change.'
            : roleHint[form.role]
        }
      />
      {form.role !== 'administrator' && (
        <NetworkChecklist
          label="Authorized networks"
          networks={networks}
          value={form.networkIds}
          onChange={(ids) => set('networkIds', ids)}
        />
      )}
    </FormModal>
  );
}
