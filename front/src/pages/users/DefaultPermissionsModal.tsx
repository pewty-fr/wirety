import { useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { FormModal, Select, Spinner, apiErrorMessage, useToast } from '../../components/ui';
import { resourceKeys, useDefaultPermissions, useNetworks } from '../../hooks/useQueries';
import { NetworkChecklist } from './NetworkChecklist';
import { roleHint, roleOptions } from './roles';
import type { Role } from './roles';


interface DefaultsForm {
  role: Role;
  networkIds: string[];
}

/** The role and networks a new user gets (also on their first single sign-on). */
export default function DefaultPermissionsModal({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) {
  const toast = useToast();
  const queryClient = useQueryClient();
  const { data: networks = [] } = useNetworks();
  const defaults = useDefaultPermissions(isOpen);
  // Null until the current defaults are loaded: saving before would overwrite them.
  const [form, setForm] = useState<DefaultsForm | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!isOpen) return;
    setSubmitError(null);
    setForm(
      defaults.data
        ? { role: defaults.data.default_role, networkIds: defaults.data.default_authorized_networks ?? [] }
        : null,
    );
  }, [isOpen, defaults.data]);

  const handleSubmit = async () => {
    if (!form) return;
    setSubmitError(null);
    setSubmitting(true);
    try {
      await api.updateDefaultPermissions({
        default_role: form.role,
        default_authorized_networks: form.role === 'administrator' ? [] : form.networkIds,
      });
    } catch (err) {
      setSubmitError(apiErrorMessage(err));
      setSubmitting(false);
      return;
    }
    setSubmitting(false);
    toast.success('Default permissions saved');
    void queryClient.invalidateQueries({ queryKey: resourceKeys.userDefaults });
    onClose();
  };

  const loadError = defaults.isError ? `The current defaults could not be loaded: ${apiErrorMessage(defaults.error)}` : null;

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title="Default permissions for new users"
      description="Applied to the users created from now on, including on their first single sign-on."
      onSubmit={handleSubmit}
      submitLabel="Save"
      submitting={submitting}
      submitDisabled={!form}
      error={submitError ?? loadError}
    >
      {form ? (
        <>
          <Select
            label="Default role"
            value={form.role}
            onChange={(e) => setForm({ ...form, role: e.target.value as Role })}
            options={roleOptions}
            hint={roleHint[form.role]}
          />
          {form.role !== 'administrator' && (
            <NetworkChecklist
              label="Default authorized networks"
              hint="New users can use these networks from their first sign-in."
              networks={networks}
              value={form.networkIds}
              onChange={(ids) => setForm({ ...form, networkIds: ids })}
            />
          )}
        </>
      ) : (
        !loadError && (
          <div className="flex justify-center py-8 text-ink-muted" role="status">
            <Spinner className="size-5" />
            <span className="sr-only">Loading the default permissions…</span>
          </div>
        )
      )}
    </FormModal>
  );
}
