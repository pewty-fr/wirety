package network

import (
	"fmt"
	"net"
)

// hostBitsError reports a CIDR whose address has host bits set, with the
// network address to use instead.
func hostBitsError(cidr string, network *net.IPNet) error {
	return fmt.Errorf("%w: %s has host bits set, use the network address %s", ErrInvalidCIDR, cidr, network)
}

// ValidateNetworkCIDR checks that cidr names a network: CIDR notation, and an
// address without host bits (10.0.0.0/8, not 10.0.0.1/8 — routes and firewall
// rules would silently use the network anyway, which hides typos).
func ValidateNetworkCIDR(cidr string) error {
	ip, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("%w: %q is not in CIDR notation (address/prefix)", ErrInvalidCIDR, cidr)
	}
	if !ip.Equal(network.IP) {
		return hostBitsError(cidr, network)
	}
	return nil
}

// ValidateAllowedIP checks an entry of a peer's additional allowed IPs: a
// single address, or a network CIDR.
func ValidateAllowedIP(s string) error {
	if net.ParseIP(s) != nil {
		return nil
	}
	return ValidateNetworkCIDR(s)
}
