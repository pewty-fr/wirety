package ipam

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"wirety/internal/domain/network"

	"github.com/rs/zerolog/log"
)

// ErrNoFreeRange is returned by SuggestCIDRs when the base range has no free
// range of the requested size left.
var ErrNoFreeRange = errors.New("no free range left")

// Service provides IPAM helper operations.
type Service struct{}

// NewService constructs an IPAM service.
func NewService() *Service { return &Service{} }

// SuggestCIDRs returns up to count IPv4 ranges carved from baseCIDR (e.g.
// 10.0.0.0/8), each with room for maxPeers peers and none overlapping inUse,
// the ranges networks already use. It returns fewer when baseCIDR runs out,
// and ErrNoFreeRange when it has none. It only computes: IPAM is not touched.
func (s *Service) SuggestCIDRs(_ context.Context, baseCIDR string, maxPeers, count int, inUse []string) (int, []string, error) {
	if maxPeers <= 0 {
		return 0, nil, fmt.Errorf("maxPeers must be > 0")
	}
	if count <= 0 {
		count = 1
	}
	if err := network.ValidateNetworkCIDR(baseCIDR); err != nil {
		return 0, nil, fmt.Errorf("invalid base_cidr: %w", err)
	}
	base, err := netip.ParsePrefix(baseCIDR)
	if err != nil || !base.Addr().Is4() {
		return 0, nil, fmt.Errorf("invalid base_cidr: %w: %s is not an IPv4 range", network.ErrInvalidCIDR, baseCIDR)
	}

	// Smallest range whose usable hosts (all addresses but the network and
	// broadcast ones) fit maxPeers.
	prefixLen := 32
	for prefixLen >= 0 {
		usable := (1 << (32 - prefixLen)) - 2
		if usable >= maxPeers {
			break
		}
		prefixLen--
	}
	if prefixLen < 0 {
		return 0, nil, fmt.Errorf("cannot satisfy maxPeers=%d", maxPeers)
	}
	// Bound prefix so we don't propose absurdly large networks
	if prefixLen < 8 {
		prefixLen = 8
	}
	if prefixLen < base.Bits() {
		return 0, nil, fmt.Errorf("invalid base_cidr: %w: %s is too small for %d peers (a /%d is needed)", network.ErrInvalidCIDR, baseCIDR, maxPeers, prefixLen)
	}
	log.Info().Str("base_cidr", baseCIDR).Int("max_peers", maxPeers).Int("count", count).Msg("suggesting CIDRs")

	var used []netip.Prefix
	for _, cidr := range inUse {
		if p, err := netip.ParsePrefix(cidr); err == nil && p.Addr().Is4() {
			used = append(used, p.Masked())
		}
	}

	size := uint64(1) << (32 - prefixLen)
	start := uint64(addrToUint32(base.Addr()))
	end := start + uint64(1)<<(32-base.Bits())
	cidrs := make([]string, 0, count)
	for addr := start; addr < end && len(cidrs) < count; {
		candidate := netip.PrefixFrom(uint32ToAddr(uint32(addr)), prefixLen) // #nosec G115 - addr < 2^32
		if u, ok := overlapping(candidate, used); ok {
			// Skip past the range in use, to the next aligned candidate.
			after := uint64(addrToUint32(lastAddr(u))) + 1
			addr = max(addr+size, (after+size-1)/size*size)
			continue
		}
		cidrs = append(cidrs, candidate.String())
		addr += size
	}
	if len(cidrs) == 0 {
		return prefixLen, nil, fmt.Errorf("%w: every /%d of %s overlaps a network", ErrNoFreeRange, prefixLen, baseCIDR)
	}
	return prefixLen, cidrs, nil
}

func overlapping(p netip.Prefix, used []netip.Prefix) (netip.Prefix, bool) {
	for _, u := range used {
		if p.Overlaps(u) {
			return u, true
		}
	}
	return netip.Prefix{}, false
}

func addrToUint32(a netip.Addr) uint32 {
	b := a.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

func uint32ToAddr(v uint32) netip.Addr {
	return netip.AddrFrom4([4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
}

// lastAddr is the last IPv4 address of p.
func lastAddr(p netip.Prefix) netip.Addr {
	return uint32ToAddr(addrToUint32(p.Addr()) | (1<<(32-p.Bits()) - 1))
}
