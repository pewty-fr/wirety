package memory

import (
	"context"
	"fmt"
	"net"

	"wirety/internal/domain/ipam"
	"wirety/internal/domain/network"

	goipam "github.com/metal-stack/go-ipam"
)

// IPAMRepository is an in-memory implementation of ipam.Repository backed by go-ipam.
type IPAMRepository struct {
	engine goipam.Ipamer
}

// NewIPAMRepository creates a new in-memory IPAM repository.
func NewIPAMRepository(ctx context.Context) *IPAMRepository {
	return &IPAMRepository{engine: goipam.New(ctx)}
}

// EnsureRootPrefix ensures a root prefix exists (creates if missing).
func (r *IPAMRepository) EnsureRootPrefix(ctx context.Context, cidr string) (*network.IPAMPrefix, error) {
	p, err := r.engine.PrefixFrom(ctx, cidr)
	if err != nil {
		p, err = r.engine.NewPrefix(ctx, cidr)
		if err != nil {
			return nil, fmt.Errorf("ensure root prefix: %w", err)
		}
	}
	usage := p.Usage()
	return &network.IPAMPrefix{CIDR: p.Cidr, ParentCIDR: "", UsableHosts: int(usage.AvailableIPs)}, nil // #nosec G115 - AvailableIPs fits in int
}

func (r *IPAMRepository) AcquireChildPrefix(ctx context.Context, parentCIDR string, prefixLen uint8) (*network.IPAMPrefix, error) {
	child, err := r.engine.AcquireChildPrefix(ctx, parentCIDR, prefixLen)
	if err != nil {
		return nil, err
	}
	usage := child.Usage()
	return &network.IPAMPrefix{CIDR: child.Cidr, ParentCIDR: parentCIDR, UsableHosts: int(usage.AvailableIPs)}, nil // #nosec G115 - AvailableIPs fits in int
}

func (r *IPAMRepository) AcquireSpecificChildPrefix(ctx context.Context, parentCIDR string, cidr string) (*network.IPAMPrefix, error) {
	child, err := r.engine.AcquireSpecificChildPrefix(ctx, parentCIDR, cidr)
	if err != nil {
		return nil, err
	}
	usage := child.Usage()
	return &network.IPAMPrefix{CIDR: child.Cidr, ParentCIDR: parentCIDR, UsableHosts: int(usage.AvailableIPs)}, nil // #nosec G115 - AvailableIPs fits in int
}

func (r *IPAMRepository) ReleaseChildPrefix(ctx context.Context, cidr string) error {
	p, err := r.engine.PrefixFrom(ctx, cidr)
	if err != nil {
		return err
	}
	return r.engine.ReleaseChildPrefix(ctx, p)
}

func (r *IPAMRepository) DeletePrefix(ctx context.Context, cidr string) error {
	_, err := r.engine.DeletePrefix(ctx, cidr)
	return err
}

func (r *IPAMRepository) ListChildPrefixes(ctx context.Context, parentCIDR string) ([]*network.IPAMPrefix, error) {
	parent, err := r.engine.PrefixFrom(ctx, parentCIDR)
	if err != nil {
		return nil, fmt.Errorf("parent prefix not found: %w", err)
	}
	_, parentNet, err := net.ParseCIDR(parent.Cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid parent cidr")
	}
	all, err := r.engine.ReadAllPrefixCidrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("read all prefixes failed: %w", err)
	}
	out := make([]*network.IPAMPrefix, 0)
	for _, cidr := range all {
		if cidr == parent.Cidr {
			continue
		}
		_, childNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if parentNet.Contains(childNet.IP) && childNet.String() != parentNet.String() {
			cp, err := r.engine.PrefixFrom(ctx, cidr)
			if err != nil {
				continue
			}
			usage := cp.Usage()
			out = append(out, &network.IPAMPrefix{CIDR: cidr, ParentCIDR: parentCIDR, UsableHosts: int(usage.AvailableIPs)}) // #nosec G115 - AvailableIPs fits in int
		}
	}
	return out, nil
}

func (r *IPAMRepository) AcquireIP(ctx context.Context, cidr string) (string, error) {
	ipObj, err := r.engine.AcquireIP(ctx, cidr)
	if err != nil {
		return "", err
	}
	return ipObj.IP.String(), nil
}

func (r *IPAMRepository) ReleaseIP(ctx context.Context, cidr string, ip string) error {
	return r.engine.ReleaseIPFromPrefix(ctx, cidr, ip)
}

// Interface compliance assertion
var _ ipam.Repository = (*IPAMRepository)(nil)
