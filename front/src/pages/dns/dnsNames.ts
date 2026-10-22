// Names and addresses of DNS records, as the server and the agents see them.

// sanitizeDNSLabel mirrors server-side sanitizeDNSLabel:
// lowercase + replace any non-alphanumeric / non-hyphen char with '-'.
// Used so the FQDN we display in the UI matches exactly what the agent's
// DNS server actually serves.
function sanitizeDNSLabel(s: string): string {
  if (!s) return 'peer';
  const out = s.toLowerCase().replace(/[^a-z0-9-]/g, '-');
  return out || 'peer';
}

/**
 * The full DNS name (FQDN) a peer queries to resolve a record:
 * <name>.<network-name>.<network-domain-suffix or "internal">. The route of
 * the record intentionally does NOT appear — routes are an internal grouping
 * concept and renaming a route must not change the resolved name. Wildcard
 * names keep their "*." prefix intact so the preview matches what the agent
 * serves.
 *
 *   name="nas"     → "nas.mynet.internal"
 *   name="*"       → "*.mynet.internal"
 *   name="*.api"   → "*.api.mynet.internal"
 */
export function recordFQDN(name: string, networkName: string, domainSuffix?: string): string {
  const network = sanitizeDNSLabel(networkName);
  const suffix = domainSuffix?.trim() || 'internal';
  if (name === '*') {
    return network ? `*.${network}.${suffix}` : `*.${suffix}`;
  }
  if (name.startsWith('*.')) {
    const sub = name.slice(2).toLowerCase();
    return network ? `*.${sub}.${network}.${suffix}` : `*.${sub}.${suffix}`;
  }
  const label = sanitizeDNSLabel(name);
  return network ? `${label}.${network}.${suffix}` : `${label}.${suffix}`;
}

const LABEL = /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/i;

/**
 * Mirrors the server's validateDNSName: a single hostname label, or a
 * wildcard ("*" or "*.label[.label…]").
 */
export function isValidRecordName(name: string): boolean {
  if (name === '*') return true;
  if (name.startsWith('*.')) return name.slice(2).split('.').every((label) => LABEL.test(label));
  return LABEL.test(name);
}
