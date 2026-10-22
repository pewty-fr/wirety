import type { SelectOption } from '../../components/ui';
import type { User } from '../../types';

export type Role = User['role'];

export const roleOptions: SelectOption[] = [
  { value: 'user', label: 'User' },
  { value: 'administrator', label: 'Administrator' },
];

/** What a role allows, shown under the role pickers. */
export const roleHint: Record<Role, string> = {
  user: 'Can only use the networks they are authorized on.',
  administrator: 'Manages Wirety and can use every network.',
};

export const roleLabel = (role: string) => (role === 'administrator' ? 'Administrator' : 'User');

/** The bootstrap administrator of password mode (AUTH_PASSWORD): managed by the server's configuration. */
export const BOOTSTRAP_ADMIN_ID = 'admin';

/** A loose check that catches typos; the server has the last word. */
export const isEmail = (value: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);

export const MIN_PASSWORD_LENGTH = 8;
