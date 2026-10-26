import { useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { FormModal, PasswordInput, Select, TextInput, apiErrorMessage, useToast } from '../../components/ui';
import { useNetworks } from '../../hooks/useQueries';
import { NetworkChecklist } from './NetworkChecklist';
import { MIN_PASSWORD_LENGTH, isEmail, roleHint, roleOptions } from './roles';
import type { Role } from './roles';

interface UserForm {
  email: string;
  name: string;
  password: string;
  role: Role;
  networkIds: string[];
}

const emptyForm: UserForm = { email: '', name: '', password: '', role: 'user', networkIds: [] };

type Errors = Partial<Record<'email' | 'name' | 'password', string>>;

/** Creates a local user, who signs in with their email and password (password mode only). */
export default function CreateUserModal({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) {
  const toast = useToast();
  const queryClient = useQueryClient();
  const { data: networks = [] } = useNetworks();
  const [form, setForm] = useState<UserForm>(emptyForm);
  const [errors, setErrors] = useState<Errors>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!isOpen) return;
    setForm(emptyForm);
    setErrors({});
    setSubmitError(null);
  }, [isOpen]);

  const set = <K extends keyof UserForm>(key: K, value: UserForm[K]) => setForm((f) => ({ ...f, [key]: value }));

  const validate = () => {
    const e: Errors = {};
    const email = form.email.trim();
    if (!email) e.email = 'Enter the email address the user signs in with.';
    else if (!isEmail(email)) e.email = 'Enter a valid email address.';
    if (!form.name.trim()) e.name = 'Give the user a name.';
    if (form.password.length < MIN_PASSWORD_LENGTH) e.password = `Use at least ${MIN_PASSWORD_LENGTH} characters.`;
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const handleSubmit = async () => {
    setSubmitError(null);
    if (!validate()) return;
    setSubmitting(true);
    const name = form.name.trim();
    try {
      await api.createUser({
        email: form.email.trim(),
        name,
        role: form.role,
        password: form.password,
        // Administrators use every network.
        authorized_networks: form.role === 'administrator' ? [] : form.networkIds,
      });
    } catch (err) {
      setSubmitError(apiErrorMessage(err));
      setSubmitting(false);
      return;
    }
    setSubmitting(false);
    toast.success(`User “${name}” created`);
    void queryClient.invalidateQueries({ queryKey: ['users'] });
    onClose();
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title="New user"
      description="They sign in with their email address and this password."
      onSubmit={handleSubmit}
      submitLabel="Create user"
      submitting={submitting}
      error={submitError}
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <TextInput
          label="Email"
          type="email"
          required
          value={form.email}
          onChange={(e) => set('email', e.target.value)}
          placeholder="alice@example.com"
          autoComplete="off"
          error={errors.email}
          autoFocus
        />
        <TextInput
          label="Name"
          required
          value={form.name}
          onChange={(e) => set('name', e.target.value)}
          placeholder="Alice Martin"
          autoComplete="off"
          error={errors.name}
        />
      </div>
      <PasswordInput
        label="Password"
        required
        value={form.password}
        onChange={(e) => set('password', e.target.value)}
        error={errors.password}
        hint={`At least ${MIN_PASSWORD_LENGTH} characters.`}
      />
      <Select
        label="Role"
        value={form.role}
        onChange={(e) => set('role', e.target.value as Role)}
        options={roleOptions}
        hint={roleHint[form.role]}
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
