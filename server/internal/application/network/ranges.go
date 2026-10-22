package network

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"wirety/internal/domain/ipam"
	"wirety/internal/domain/network"

	"github.com/rs/zerolog/log"
)

// usableAddresses is how many peer addresses IPAM can hand out in prefix: all
// of them but the network address, and the broadcast address in IPv4. Capped
// at 2^62 for IPv6.
func usableAddresses(prefix netip.Prefix) uint64 {
	hostBits := prefix.Addr().BitLen() - prefix.Bits()
	if hostBits >= 62 {
		return 1 << 62
	}
	n := uint64(1) << hostBits
	if prefix.Addr().Is4() {
		if n <= 2 {
			return 0
		}
		return n - 2
	}
	return n - 1
}

// validateNetworkRange checks a network's IPv4 (v6 false) or IPv6 range: a
// network CIDR of that family with room for a jump peer and a client. field
// names the request field in errors.
func validateNetworkRange(field, cidr string, v6 bool) error {
	if err := network.ValidateNetworkCIDR(cidr); err != nil {
		return fmt.Errorf("%s: %w", field, err)
	}
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return fmt.Errorf("%s: %w: %q", field, network.ErrInvalidCIDR, cidr)
	}
	switch {
	case v6 && !prefix.Addr().Is6():
		return fmt.Errorf("%s: %w: %s is not an IPv6 range", field, network.ErrInvalidCIDR, cidr)
	case !v6 && !prefix.Addr().Is4():
		return fmt.Errorf("%s: %w: %s is not an IPv4 range", field, network.ErrInvalidCIDR, cidr)
	case usableAddresses(prefix) < 2:
		return fmt.Errorf("%s: %w: %s is too small, a network needs at least 2 addresses (a jump peer and a client)", field, network.ErrInvalidCIDR, cidr)
	}
	return nil
}

// checkRangeFree refuses cidr when it overlaps the range of another network
// than exceptNetworkID: IPAM has a single address space, so two networks
// sharing addresses would hand out the same IPs (and a device on both could
// not route them).
func (s *Service) checkRangeFree(ctx context.Context, cidr, exceptNetworkID string) error {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return fmt.Errorf("%w: %q", network.ErrInvalidCIDR, cidr)
	}
	networks, err := s.repo.ListNetworks(ctx)
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}
	for _, n := range networks {
		if n.ID == exceptNetworkID {
			continue
		}
		for _, other := range []string{n.CIDR, n.CIDRv6} {
			if other == "" {
				continue
			}
			otherPrefix, err := netip.ParsePrefix(other)
			if err != nil || !prefix.Overlaps(otherPrefix) {
				continue
			}
			return fmt.Errorf("%w: %s overlaps %s, the range of network “%s”", network.ErrCIDROverlap, cidr, other, n.Name)
		}
	}
	return nil
}

// checkRoom refuses a new range too small for the network's peers.
func checkRoom(field, cidr string, peers int) error {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return fmt.Errorf("%s: %w: %q", field, network.ErrInvalidCIDR, cidr)
	}
	if usable := usableAddresses(prefix); usable < uint64(peers) { // #nosec G115 - peers is a count
		return fmt.Errorf("%s: %w: %s has room for %d peers, the network has %d", field, network.ErrInvalidCIDR, cidr, usable, peers)
	}
	return nil
}

// moveRange moves the network's IPv4 (v6 false) or IPv6 range from oldCIDR
// (empty when the network had none) to newCIDR in IPAM: it frees the old range,
// reserves the new one, keeps the peers' addresses the new range still contains
// (growing a network renumbers no one) and gives the other peers a new one.
// The new range was checked to be free and large enough.
func (s *Service) moveRange(ctx context.Context, networkID string, peers []*network.Peer, oldCIDR, newCIDR string, v6 bool) error {
	address := func(p *network.Peer) *string {
		if v6 {
			return &p.AddressV6
		}
		return &p.Address
	}

	if oldCIDR != "" {
		for _, p := range peers {
			if a := *address(p); a != "" {
				if err := s.repo.ReleaseIP(ctx, oldCIDR, bareIP(a)); err != nil {
					log.Warn().Err(err).Str("ip", a).Str("cidr", oldCIDR).Msg("failed to release address while changing the network range")
				}
			}
		}
		s.freeRanges(ctx, oldCIDR)
	}
	if _, err := s.repo.EnsureRootPrefix(ctx, newCIDR); err != nil {
		return fmt.Errorf("failed to reserve %s in IPAM: %w", newCIDR, err)
	}

	prefix, err := netip.ParsePrefix(newCIDR)
	if err != nil {
		return fmt.Errorf("%w: %q", network.ErrInvalidCIDR, newCIDR)
	}
	// Keep addresses first, so that a new address never takes one a later peer
	// keeps.
	var renumbered []*network.Peer
	for _, p := range peers {
		addr, err := netip.ParseAddr(bareIP(*address(p)))
		if err == nil && prefix.Contains(addr) && s.repo.AcquireSpecificIP(ctx, newCIDR, addr.String()) == nil {
			continue
		}
		renumbered = append(renumbered, p)
	}
	for _, p := range renumbered {
		ip, err := s.repo.AcquireIP(ctx, newCIDR)
		if err != nil {
			return fmt.Errorf("failed to allocate an address in %s for peer %s: %w", newCIDR, p.Name, err)
		}
		*address(p) = ip
		p.UpdatedAt = time.Now()
		if err := s.repo.UpdatePeer(ctx, networkID, p); err != nil {
			return fmt.Errorf("failed to update peer %s with its new address: %w", p.Name, err)
		}
	}
	return nil
}

// bareIP strips the prefix length some stored addresses carry (10.0.0.2/32).
func bareIP(addr string) string {
	if i := strings.IndexByte(addr, '/'); i != -1 {
		return addr[:i]
	}
	return addr
}

// acquireError explains why a new peer got no address in the network's range.
func acquireError(n *network.Network, cidr, family string, err error) error {
	if errors.Is(err, ipam.ErrNoAddressAvailable) {
		return fmt.Errorf("%w: “%s” has no free %s address left in %s, widen its range", network.ErrNetworkFull, n.Name, family, cidr)
	}
	return fmt.Errorf("failed to acquire %s address from IPAM: %w", family, err)
}
