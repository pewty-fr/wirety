// Utility functions for computing network capacity and slot usage from CIDR
// Assumes IPv4 CIDR in form a.b.c.d/p

export function computeCapacityFromCIDR(cidr: string): number | null {
  const parts = cidr.split('/');
  if (parts.length !== 2) return null;
  const prefix = parseInt(parts[1], 10);
  if (isNaN(prefix) || prefix < 0 || prefix > 32) return null;
  // Total hosts = 2^(32 - prefix); usable hosts typically exclude network and broadcast (size - 2) if prefix <=30
  const size = 2 ** (32 - prefix);
  if (prefix >= 31) {
    // /31 or /32 special cases: treat capacity as size (point-to-point) minus 0 or 0
    return size;
  }
  return size - 2;
}

export function formatCapacity(capacity: number | null): string {
  if (capacity == null) return 'N/A';
  if (capacity > 100000) return '>100k';
  return capacity.toLocaleString();
}
