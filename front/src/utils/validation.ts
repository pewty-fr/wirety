/**
 * Validates if a string is a valid CIDR notation (e.g., 192.168.1.0/24 or 2001:db8::/32)
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
    // Validate each octet is 0-255
    const octets = ip.split('.').map(Number);
    const validOctets = octets.every(octet => octet >= 0 && octet <= 255);
    
    // Validate prefix is 0-32 for IPv4
    return validOctets && prefixNum >= 0 && prefixNum <= 32;
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
