package ipam

import (
	"context"
	"fmt"

	"wirety/internal/domain/ipam"

	goipam "github.com/metal-stack/go-ipam"
	"github.com/rs/zerolog/log"
)

// Service provides IPAM helper operations backed by an IPAM repository.
// Hexagonal: application layer depends only on ipam.Repository abstraction.
type Service struct {
	repo ipam.Repository
}

// NewService constructs an IPAM service using the provided repository.
func NewService(repo ipam.Repository) *Service { return &Service{repo: repo} }

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
	log.Info().Str("base_cidr", baseCIDR).Int("max_peers", maxPeers).Int("count", count).Msg("suggesting CIDRs")

	// Ensure root prefix persisted
	// _, err := s.repo.EnsureRootPrefix(ctx, baseCIDR)
	// if err != nil {
	// 	return 0, nil, err
	// }

	cidrs := make([]string, 0, count)
	ipam := goipam.New(context.Background())
	_, _ = ipam.NewPrefix(ctx, baseCIDR)

	for i := 0; i < count; i++ {

		prefix, err := ipam.AcquireChildPrefix(ctx, baseCIDR, uint8(prefixLen)) // #nosec G115 - prefixLen is validated to be 0-32
		if err != nil {
			return 0, nil, fmt.Errorf("failed allocating child prefix: %w", err)
		}
		child, err := s.repo.EnsureRootPrefix(ctx, prefix.Cidr)
		if err != nil {
			// if root prefix can't be ensured, that means the it overlap. we need to try again
			i--
			continue
		}
		err = s.repo.DeletePrefix(ctx, child.CIDR)
		if err != nil {
			i--
			continue
		}
		cidrs = append(cidrs, child.CIDR)
	}

	return prefixLen, cidrs, nil
}
