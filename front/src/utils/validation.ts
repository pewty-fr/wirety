export const validateCIDR = (cidr: string): boolean => {
  const cidrRegex = /^(\d{1,3}\.){3}\d{1,3}\/\d{1,2}$/;
  if (!cidrRegex.test(cidr)) return false;

  const [ip, prefix] = cidr.split('/');
  const prefixNum = parseInt(prefix, 10);
  if (prefixNum < 0 || prefixNum > 32) return false;

  const octets = ip.split('.').map(Number);
  return octets.every(octet => octet >= 0 && octet <= 255);
};

export const validateIPv4 = (ip: string): boolean => {
  const ipRegex = /^(\d{1,3}\.){3}\d{1,3}$/;
  if (!ipRegex.test(ip)) return false;

  const octets = ip.split('.').map(Number);
  return octets.every(octet => octet >= 0 && octet <= 255);
};

export const validatePort = (port: number): boolean => {
  return port > 0 && port <= 65535;
};

export const validateEndpoint = (endpoint: string): boolean => {
  const parts = endpoint.split(':');
  if (parts.length !== 2) return false;

  const [ip, portStr] = parts;
  const port = parseInt(portStr, 10);
  return validateIPv4(ip) && validatePort(port);
};

export const validateDomain = (domain: string): boolean => {
  const domainRegex = /^[a-z0-9]+([\-\.]{1}[a-z0-9]+)*\.[a-z]{2,}$/i;
  return domainRegex.test(domain);
};

export const suggestCIDRs = (): string[] => {
  return [
    '10.0.0.0/8',
    '10.0.0.0/16',
    '10.0.0.0/24',
    '172.16.0.0/12',
    '172.16.0.0/16',
    '172.16.0.0/24',
    '192.168.0.0/16',
    '192.168.0.0/24',
    '192.168.1.0/24',
  ];
};

export const formatDate = (dateString: string): string => {
  const date = new Date(dateString);
  return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
};

export const truncate = (str: string, maxLen: number = 20): string => {
  if (str.length <= maxLen) return str;
  return str.substring(0, maxLen - 3) + '...';
};
