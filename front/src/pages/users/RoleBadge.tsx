import { Badge } from '../../components/ui';
import { roleLabel } from './roles';

/** A user's role: administrators in the primary tone, users neutral. */
export function RoleBadge({ role }: { role: string }) {
  return <Badge tone={role === 'administrator' ? 'primary' : 'neutral'}>{roleLabel(role)}</Badge>;
}
