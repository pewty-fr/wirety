/**
 * Parses an API timestamp. The server sends Go's zero time
 * ("0001-01-01T00:00:00Z") for events that never happened (a user who never
 * signed in): those, like missing or invalid values, give null.
 */
function parse(iso?: string | null): Date | null {
  if (!iso) return null;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime()) || d.getUTCFullYear() < 1971) return null;
  return d;
}

/** "Oct 1, 2026", or null when unset. */
export function formatDate(iso?: string | null): string | null {
  return parse(iso)?.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' }) ?? null;
}

/** "Oct 1, 2026, 10:00 AM", or null when unset. */
export function formatDateTime(iso?: string | null): string | null {
  return (
    parse(iso)?.toLocaleString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    }) ?? null
  );
}

/** "just now", "5m ago", "3h ago", "2d ago", or null when unset. */
export function timeAgo(iso?: string | null): string | null {
  const d = parse(iso);
  if (!d) return null;
  const mins = Math.floor((Date.now() - d.getTime()) / 60000);
  if (mins < 2) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

/** True when the timestamp is set and in the past. */
export function isPast(iso?: string | null): boolean {
  const d = parse(iso);
  return d !== null && d.getTime() < Date.now();
}
