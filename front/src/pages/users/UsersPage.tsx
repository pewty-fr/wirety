import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { AdjustmentsHorizontalIcon, KeyIcon, PencilSquareIcon, PlusIcon, TrashIcon } from '@heroicons/react/20/solid';
import { UsersIcon } from '@heroicons/react/24/outline';
import api from '../../api/client';
import {
  Alert,
  Button,
  EmptyState,
  IconButton,
  Page,
  PageHeader,
  Pagination,
  Spinner,
  Table,
  TBody,
  TD,
  TH,
  THead,
  TR,
  apiErrorMessage,
  useConfirm,
  useToast,
} from '../../components/ui';
import { useAuth } from '../../contexts/AuthContext';
import { useNetworks, useUsers } from '../../hooks/useQueries';
import type { User } from '../../types';
import CreateUserModal from './CreateUserModal';
import { formatDate } from '../../utils/dates';
import DefaultPermissionsModal from './DefaultPermissionsModal';
import EditUserModal from './EditUserModal';
import ResetPasswordModal from './ResetPasswordModal';
import { RoleBadge } from './RoleBadge';
import { BOOTSTRAP_ADMIN_ID } from './roles';

const PAGE_SIZE = 25;

/** A modal about one user; the user stays set while the modal closes. */
interface UserModalState {
  open: boolean;
  user: User | null;
}
const closed: UserModalState = { open: false, user: null };

export default function UsersPage() {
  const { user: me, authConfig } = useAuth();
  const isAdmin = me?.role === 'administrator';
  // In single sign-on mode, users are created on their first sign-in and have no password.
  const passwordMode = authConfig?.simple_auth === true;
  const { data: users = [], isLoading } = useUsers(isAdmin);
  const { data: networks = [] } = useNetworks();
  const [page, setPage] = useState(1);
  const [creating, setCreating] = useState(false);
  const [defaultsOpen, setDefaultsOpen] = useState(false);
  const [editing, setEditing] = useState<UserModalState>(closed);
  const [resetting, setResetting] = useState<UserModalState>(closed);
  const confirm = useConfirm();
  const toast = useToast();
  const queryClient = useQueryClient();

  const deleteUser = useMutation({
    mutationFn: (u: User) => api.deleteUser(u.id),
    onSuccess: (_, u) => {
      toast.success(`User “${u.name}” deleted`);
      void queryClient.invalidateQueries({ queryKey: ['users'] });
    },
    onError: (err) => toast.error(apiErrorMessage(err)),
  });

  const handleDelete = async (u: User) => {
    const ok = await confirm({
      title: `Delete user “${u.name}”?`,
      message: `${u.email} will no longer be able to sign in. This cannot be undone.`,
      confirmLabel: 'Delete user',
      destructive: true,
    });
    if (ok) deleteUser.mutate(u);
  };

  if (!isAdmin) {
    return (
      <Page>
        <PageHeader title="Users" />
        <Alert tone="warning" title="Administrators only">
          Only administrators can manage users.
        </Alert>
      </Page>
    );
  }

  // The signed-in administrator and the built-in one cannot be deleted.
  const canDelete = (u: User) => u.id !== me?.id && u.id !== BOOTSTRAP_ADMIN_ID;
  // Only local users have a password; the built-in admin's comes from the server's configuration.
  const canResetPassword = (u: User) => passwordMode && u.id !== BOOTSTRAP_ADMIN_ID;

  const networkNames = (u: User) => {
    const names = u.authorized_networks.map((id) => networks.find((n) => n.id === id)?.name ?? id);
    return names.length > 2 ? `${names.slice(0, 2).join(', ')} +${names.length - 2}` : names.join(', ');
  };

  const pages = Math.max(1, Math.ceil(users.length / PAGE_SIZE));
  const currentPage = Math.min(page, pages);
  const shown = users.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE);
  const newUserButton = (
    <Button icon={<PlusIcon className="size-4" />} onClick={() => setCreating(true)}>
      New user
    </Button>
  );

  return (
    <Page>
      <PageHeader
        title="Users"
        description="Who can sign in to Wirety, and which networks they can use."
        actions={
          <>
            <Button
              variant="secondary"
              icon={<AdjustmentsHorizontalIcon className="size-4" />}
              onClick={() => setDefaultsOpen(true)}
            >
              Default permissions
            </Button>
            {passwordMode && newUserButton}
          </>
        }
      />

      {isLoading ? (
        <div className="flex justify-center py-16 text-ink-muted" role="status">
          <Spinner className="size-5" />
          <span className="sr-only">Loading users…</span>
        </div>
      ) : users.length === 0 ? (
        <EmptyState
          icon={UsersIcon}
          title="No user yet"
          description={
            passwordMode
              ? 'Create a user to let someone sign in with their email and a password.'
              : 'Users appear here after their first sign-in.'
          }
          action={passwordMode ? newUserButton : undefined}
        />
      ) : (
        <>
          <Table>
            <THead>
              <tr>
                <TH>User</TH>
                <TH>Role</TH>
                <TH>Networks</TH>
                <TH>Last sign-in</TH>
                <TH>
                  <span className="sr-only">Actions</span>
                </TH>
              </tr>
            </THead>
            <TBody>
              {shown.map((u) => (
                <TR key={u.id} onClick={() => setEditing({ open: true, user: u })}>
                  <TD className="max-w-xs">
                    <div className="flex items-center gap-2 font-medium text-ink">
                      <span className="truncate">{u.name}</span>
                      {u.id === me?.id && <span className="text-xs font-normal text-ink-muted">(you)</span>}
                    </div>
                    <div className="truncate text-xs text-ink-muted">{u.email}</div>
                  </TD>
                  <TD>
                    <RoleBadge role={u.role} />
                  </TD>
                  <TD className="max-w-xs truncate">
                    {u.role === 'administrator' ? (
                      <span className="text-ink-muted">All networks</span>
                    ) : u.authorized_networks.length === 0 ? (
                      <span className="text-ink-muted">No access</span>
                    ) : (
                      networkNames(u)
                    )}
                  </TD>
                  <TD>{formatDate(u.last_login_at) ?? <span className="text-ink-muted">Never</span>}</TD>
                  <TD className="w-0 text-right" onClick={(e) => e.stopPropagation()}>
                    <div className="flex justify-end gap-1">
                      <IconButton label={`Edit ${u.name}`} onClick={() => setEditing({ open: true, user: u })}>
                        <PencilSquareIcon className="size-4" />
                      </IconButton>
                      {canResetPassword(u) && (
                        <IconButton label={`Reset password for ${u.name}`} onClick={() => setResetting({ open: true, user: u })}>
                          <KeyIcon className="size-4" />
                        </IconButton>
                      )}
                      {canDelete(u) && (
                        <IconButton label={`Delete ${u.name}`} tone="danger" onClick={() => handleDelete(u)}>
                          <TrashIcon className="size-4" />
                        </IconButton>
                      )}
                    </div>
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
          <Pagination page={currentPage} pageSize={PAGE_SIZE} total={users.length} onPageChange={setPage} />
        </>
      )}

      <EditUserModal
        isOpen={editing.open}
        onClose={() => setEditing((s) => ({ ...s, open: false }))}
        user={editing.user}
      />
      <ResetPasswordModal
        isOpen={resetting.open}
        onClose={() => setResetting((s) => ({ ...s, open: false }))}
        user={resetting.user}
      />
      <DefaultPermissionsModal isOpen={defaultsOpen} onClose={() => setDefaultsOpen(false)} />
      {passwordMode && <CreateUserModal isOpen={creating} onClose={() => setCreating(false)} />}
    </Page>
  );
}
