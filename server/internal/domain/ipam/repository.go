package ipam

import (
	"context"
	"wirety/internal/domain/network"
)

// Repository defines persistence operations for IP address management.
// It returns the existing network.IPAMPrefix type to avoid large refactors.
type Repository interface {
	EnsureRootPrefix(ctx context.Context, cidr string) (*network.IPAMPrefix, error)
	AcquireChildPrefix(ctx context.Context, parentCIDR string, prefixLen uint8) (*network.IPAMPrefix, error)
	AcquireSpecificChildPrefix(ctx context.Context, parentCIDR string, cidr string) (*network.IPAMPrefix, error)
	ReleaseChildPrefix(ctx context.Context, cidr string) error
	// DeletePrefix removes a prefix, releasing the addresses still allocated in it.
	DeletePrefix(ctx context.Context, cidr string) error
	ListChildPrefixes(ctx context.Context, parentCIDR string) ([]*network.IPAMPrefix, error)
	// AcquireIP allocates a free address of the prefix; ErrNoAddressAvailable
	// when there is none left.
	AcquireIP(ctx context.Context, cidr string) (string, error)
	// AcquireSpecificIP allocates ip in the prefix; an error when it is taken
	// or reserved (network or broadcast address).
	AcquireSpecificIP(ctx context.Context, cidr string, ip string) error
	ReleaseIP(ctx context.Context, cidr string, ip string) error
}
