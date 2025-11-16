package ipam

import (
	"context"
	"fmt"

	"wirety/internal/domain/network"
)

// Service provides IPAM helper operations backed by the domain Repository.
// This allows later replacement of the memory repository with a DB-backed one
// without changing application/service logic.
type Service struct {
	repo network.Repository
}

// NewService constructs an IPAM service using the provided repository.
func NewService(repo network.Repository) *Service { return &Service{repo: repo} }

// SuggestCIDRs returns a list of CIDRs sized to hold at least maxPeers peers.
// baseCIDR is the root network we carve from (e.g. 10.0.0.0/8). count is how many suggestions.
func (s *Service) SuggestCIDRs(ctx context.Context, baseCIDR string, maxPeers, count int) (int, []string, error) {
	if maxPeers <= 0 {
		return 0, nil, fmt.Errorf("maxPeers must be > 0")
	}
	if count <= 0 {
		count = 1
	}

	// Determine required prefix length: smallest prefix with usable hosts >= maxPeers.
	// Usable hosts = 2^(32-prefix) - 2
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
	// Bound prefix so we don't propose absurdly small networks
	if prefixLen < 8 { // avoid generating giant /7 etc.
		prefixLen = 8
	}

	// Ensure root prefix persisted
	_, err := s.repo.EnsureRootPrefix(ctx, baseCIDR)
	if err != nil {
		return 0, nil, err
	}

	cidrs := make([]string, 0, count)
	for i := 0; i < count; i++ {
		child, err := s.repo.AcquireChildPrefix(ctx, baseCIDR, uint8(prefixLen))
		if err != nil {
			if len(cidrs) == 0 {
				return 0, nil, fmt.Errorf("failed allocating child prefix: %w", err)
			}
			break
		}
		cidrs = append(cidrs, child.CIDR)
	}

	return prefixLen, cidrs, nil
}
