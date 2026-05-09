/**
 * Checks whether the host bits of an IPv4 address are zero, i.e. whether it
 * is the actual network address for the given prefix length.
 * e.g. 10.255.236.0/22 → ok; 10.255.238.0/22 → NOT the network address.
 */
function isIPv4NetworkAddress(ip: string, prefixLen: number): boolean {
  const octets = ip.split('.').map(Number);
  if (octets.length !== 4) return false;
  // Build 32-bit integer
  const ipInt = ((octets[0] << 24) | (octets[1] << 16) | (octets[2] << 8) | octets[3]) >>> 0;
  const mask = prefixLen === 0 ? 0 : (0xffffffff << (32 - prefixLen)) >>> 0;
  return (ipInt & mask) >>> 0 === ipInt;
}

/**
 * For an IPv4 CIDR with host bits set, return the corrected network address.
 * e.g. "10.255.238.0/22" → "10.255.236.0/22"
 */
function correctIPv4NetworkAddress(ip: string, prefixLen: number): string {
  const octets = ip.split('.').map(Number);
  const ipInt = ((octets[0] << 24) | (octets[1] << 16) | (octets[2] << 8) | octets[3]) >>> 0;
  const mask = prefixLen === 0 ? 0 : (0xffffffff << (32 - prefixLen)) >>> 0;
  const netInt = (ipInt & mask) >>> 0;
  return [
    (netInt >>> 24) & 0xff,
    (netInt >>> 16) & 0xff,
    (netInt >>> 8) & 0xff,
    netInt & 0xff,
  ].join('.') + '/' + prefixLen;
}

/**
 * Validates if a string is a valid CIDR notation (e.g., 192.168.1.0/24 or 2001:db8::/32)
 * Also checks that the IP is the actual network address (host bits are zero).
 * @param cidr - The CIDR string to validate
 * @returns true if valid, false otherwise
 */
export function isValidCIDR(cidr: string): boolean {
  if (!cidr || typeof cidr !== 'string') {
    return false;
  }

  const parts = cidr.trim().split('/');
  if (parts.length !== 2) {
    return false;
  }

  const [ip, prefix] = parts;
  const prefixNum = parseInt(prefix, 10);

  // Check if prefix is a valid number
  if (isNaN(prefixNum)) {
    return false;
  }

  // Check for IPv4
  const ipv4Regex = /^(\d{1,3}\.){3}\d{1,3}$/;
  if (ipv4Regex.test(ip)) {
    const octets = ip.split('.').map(Number);
    const validOctets = octets.every(octet => octet >= 0 && octet <= 255);
    if (!validOctets || prefixNum < 0 || prefixNum > 32) return false;
    return isIPv4NetworkAddress(ip, prefixNum);
  }

  // Check for IPv6
  const ipv6Regex = /^([\da-f]{0,4}:){2,7}[\da-f]{0,4}$/i;
  if (ipv6Regex.test(ip) || ip === '::') {
    // Validate prefix is 0-128 for IPv6
    return prefixNum >= 0 && prefixNum <= 128;
  }

  return false;
}

/**
 * Get error message for invalid CIDR
 * @param cidr - The CIDR string
 * @returns Error message or null if valid
 */
export function getCIDRError(cidr: string): string | null {
  if (!cidr) {
    return null; // Empty is handled by required field
  }

  if (!cidr.includes('/')) {
    return 'CIDR must include a prefix (e.g., 192.168.1.0/24)';
  }

  const parts = cidr.split('/');
  if (parts.length !== 2) {
    return 'Invalid CIDR format. Use IP/prefix (e.g., 10.0.0.0/8)';
  }

  const [ip, prefix] = parts;
  const prefixNum = parseInt(prefix, 10);

  if (isNaN(prefixNum)) {
    return 'Prefix must be a number';
  }

  const ipv4Regex = /^(\d{1,3}\.){3}\d{1,3}$/;
  if (ipv4Regex.test(ip)) {
    const octets = ip.split('.').map(Number);
    if (!octets.every(octet => octet >= 0 && octet <= 255)) {
      return 'Invalid IPv4 address (octets must be 0-255)';
    }
    if (prefixNum < 0 || prefixNum > 32) {
      return 'IPv4 prefix must be between 0 and 32';
    }
    if (!isIPv4NetworkAddress(ip, prefixNum)) {
      const corrected = correctIPv4NetworkAddress(ip, prefixNum);
      return `${ip} has host bits set — did you mean ${corrected}?`;
    }
    return null;
  }

  const ipv6Regex = /^([\da-f]{0,4}:){2,7}[\da-f]{0,4}$/i;
  if (ipv6Regex.test(ip) || ip === '::') {
    if (prefixNum < 0 || prefixNum > 128) {
      return 'IPv6 prefix must be between 0 and 128';
    }
    return null;
  }

  return 'Invalid IP address format';
}

/**
 * Generates a random IPv6 Unique Local Address (ULA) prefix per RFC 4193.
 *
 * ULAs are the IPv6 equivalent of RFC 1918 private addresses: they are
 * non-routable on the public internet and intended exactly for private
 * networks like a Wirety VPN.  RFC 4193 mandates:
 *   • prefix `fc00::/7` with the L bit set (`fd00::/8` in practice)
 *   • a 40-bit pseudo-random Global ID (NOT sequential or low-bit) to make
 *     two independent ULA networks effectively non-collide-able if they ever
 *     end up bridged
 *   • subnet at /64 (one trillion subnets per Global ID; for a single VPN
 *     network /64 is the natural choice — plenty of room for `/128` peers)
 *
 * Returns CIDRs in the form `fd<10 hex chars>:<4>:<4>::/64`.  Multiple calls
 * produce independent random Global IDs (browser crypto.getRandomValues).
 */
export function suggestIPv6ULACIDRs(count: number = 5): string[] {
  const out: string[] = [];
  // We need 40 random bits for the Global ID + 16 bits for the subnet ID.
  // Build it via crypto.getRandomValues so each suggestion is genuinely random
  // (NOT seeded from time/network state, per RFC 4193 §3.2.2).
  for (let i = 0; i < count; i++) {
    const bytes = new Uint8Array(7); // 5 (Global ID) + 2 (subnet ID) = 56 bits
    crypto.getRandomValues(bytes);
    const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
    // Format as fdXX:XXXX:XXXX:YYYY::/64
    //   GlobalID = first 40 bits  -> hex[0..10] (5 bytes)
    //   SubnetID = next 16 bits   -> hex[10..14] (2 bytes)
    const cidr = `fd${hex.slice(0, 2)}:${hex.slice(2, 6)}:${hex.slice(6, 10)}:${hex.slice(10, 14)}::/64`;
    out.push(cidr);
  }
  return out;
}
