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
	DeletePrefix(ctx context.Context, cidr string) error
	ListChildPrefixes(ctx context.Context, parentCIDR string) ([]*network.IPAMPrefix, error)
	AcquireIP(ctx context.Context, cidr string) (string, error)
	ReleaseIP(ctx context.Context, cidr string, ip string) error
}
