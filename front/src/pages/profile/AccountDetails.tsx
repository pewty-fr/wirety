import type { ReactNode } from 'react';
import { Badge, cx } from '../../components/ui';
import { useAuth } from '../../contexts/AuthContext';
import { useNetworks } from '../../hooks/useQueries';
import { RoleBadge } from '../users/RoleBadge';

function Item({ label, wide, children }: { label: string; wide?: boolean; children: ReactNode }) {
  return (
    <div className={cx('min-w-0', wide && 'sm:col-span-2')}>
      <dt className="text-xs font-medium text-ink-muted">{label}</dt>
      <dd className="mt-1 text-sm text-ink">{children}</dd>
    </div>
  );
}

/** The signed-in user's account, read-only: administrators manage it on the Users page. */
export function AccountDetails() {
  const { user, authConfig } = useAuth();
  const { data: networks = [] } = useNetworks();
  if (!user) return null;
  const isAdmin = user.role === 'administrator';
  const authorized = user.authorized_networks ?? [];

  return (
    <dl className="grid gap-x-6 gap-y-4 sm:grid-cols-2">
      <Item label="Name">{user.name}</Item>
      <Item label="Email">
        <span className="break-all">{user.email}</span>
      </Item>
      <Item label="Role">
        <RoleBadge role={user.role} />
      </Item>
      <Item label="Sign-in">
        {authConfig?.enabled ? (
          <>
            Single sign-on
            {authConfig.issuer_url && (
              <span className="block truncate font-mono text-xs text-ink-muted">{authConfig.issuer_url}</span>
            )}
          </>
        ) : (
          'Password'
        )}
      </Item>
      <Item label="Authorized networks" wide>
        {isAdmin ? (
          <span className="text-ink-secondary">All networks, as an administrator.</span>
        ) : authorized.length === 0 ? (
          <span className="text-ink-secondary">None yet: ask an administrator for access.</span>
        ) : (
          <ul className="flex flex-wrap gap-1.5" aria-label="Authorized networks">
            {authorized.map((id) => {
              const name = networks.find((n) => n.id === id)?.name;
              return (
                <li key={id}>
                  <Badge className={name ? undefined : 'font-mono'}>{name ?? id}</Badge>
                </li>
              );
            })}
          </ul>
        )}
      </Item>
    </dl>
  );
}
