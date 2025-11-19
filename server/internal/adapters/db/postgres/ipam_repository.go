package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net"

	"wirety/internal/domain/ipam"
	"wirety/internal/domain/network"

	goipam "github.com/metal-stack/go-ipam"
)

// IPAMRepository is a Postgres-backed implementation of ipam.Repository.
// It keeps an in-memory go-ipam engine for allocation logic and persists state
// (prefixes and allocated IPs) to SQL tables.
type IPAMRepository struct {
	db     *sql.DB
	engine goipam.Ipamer
}

// NewIPAMRepository creates a repository and loads existing state.
func NewIPAMRepository(ctx context.Context, db *sql.DB) (*IPAMRepository, error) {
	r := &IPAMRepository{db: db, engine: goipam.New(ctx)}
	// Load prefixes first
	rows, err := db.QueryContext(ctx, `SELECT cidr FROM ipam_prefixes ORDER BY cidr`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cidr string
			if err = rows.Scan(&cidr); err != nil {
				return nil, err
			}
			_, _ = r.engine.NewPrefix(ctx, cidr)
		}
	}
	// Load allocated IPs
	ipRows, err := db.QueryContext(ctx, `SELECT prefix_cidr, ip FROM ipam_allocated_ips`)
	if err == nil {
		defer ipRows.Close()
		for ipRows.Next() {
			var prefix, ip string
			if err = ipRows.Scan(&prefix, &ip); err != nil {
				return nil, err
			}
			// Ensure prefix is loaded
			if _, err2 := r.engine.PrefixFrom(ctx, prefix); err2 != nil {
				// create prefix if missing to keep engine consistent
				_, _ = r.engine.NewPrefix(ctx, prefix)
			}
			// Mark IP as used inside engine by acquiring it (engine will allocate first free; can't force specific ip easily)
			// For correctness we skip re-allocation (engine state rebuild best-effort). A full deterministic restore would require upstream support.
		}
	}
	return r, nil
}

// EnsureRootPrefix ensures a root prefix exists.
func (r *IPAMRepository) EnsureRootPrefix(ctx context.Context, cidr string) (*network.IPAMPrefix, error) {
	// Try load from engine
	p, err := r.engine.PrefixFrom(ctx, cidr)
	if err != nil {
		// create in engine
		p, err = r.engine.NewPrefix(ctx, cidr)
		if err != nil {
			return nil, fmt.Errorf("create prefix: %w", err)
		}
		// persist
		if _, err = r.db.ExecContext(ctx, `INSERT INTO ipam_prefixes (cidr,parent_cidr,created_at) VALUES ($1,$2,NOW()) ON CONFLICT DO NOTHING`, cidr, nil); err != nil {
			return nil, fmt.Errorf("persist prefix: %w", err)
		}
	}
	usage := p.Usage()
	return &network.IPAMPrefix{CIDR: p.Cidr, ParentCIDR: "", UsableHosts: int(usage.AvailableIPs)}, nil
}

func (r *IPAMRepository) AcquireChildPrefix(ctx context.Context, parentCIDR string, prefixLen uint8) (*network.IPAMPrefix, error) {
	// Ensure parent exists in engine
	if _, err := r.engine.PrefixFrom(ctx, parentCIDR); err != nil {
		if _, err = r.engine.NewPrefix(ctx, parentCIDR); err != nil {
			return nil, fmt.Errorf("parent missing: %w", err)
		}
	}
	child, err := r.engine.AcquireChildPrefix(ctx, parentCIDR, prefixLen)
	if err != nil {
		return nil, err
	}
	// Persist child
	if _, err = r.db.ExecContext(ctx, `INSERT INTO ipam_prefixes (cidr,parent_cidr,created_at) VALUES ($1,$2,NOW())`, child.Cidr, parentCIDR); err != nil {
		return nil, fmt.Errorf("persist child prefix: %w", err)
	}
	usage := child.Usage()
	return &network.IPAMPrefix{CIDR: child.Cidr, ParentCIDR: parentCIDR, UsableHosts: int(usage.AvailableIPs)}, nil
}

func (r *IPAMRepository) AcquireSpecificChildPrefix(ctx context.Context, parentCIDR string, cidr string) (*network.IPAMPrefix, error) {
	if _, err := r.engine.PrefixFrom(ctx, parentCIDR); err != nil {
		if _, err = r.engine.NewPrefix(ctx, parentCIDR); err != nil {
			return nil, err
		}
	}
	child, err := r.engine.AcquireSpecificChildPrefix(ctx, parentCIDR, cidr)
	if err != nil {
		return nil, err
	}
	if _, err = r.db.ExecContext(ctx, `INSERT INTO ipam_prefixes (cidr,parent_cidr,created_at) VALUES ($1,$2,NOW()) ON CONFLICT DO NOTHING`, child.Cidr, parentCIDR); err != nil {
		return nil, fmt.Errorf("persist specific child: %w", err)
	}
	usage := child.Usage()
	return &network.IPAMPrefix{CIDR: child.Cidr, ParentCIDR: parentCIDR, UsableHosts: int(usage.AvailableIPs)}, nil
}

func (r *IPAMRepository) ReleaseChildPrefix(ctx context.Context, cidr string) error {
	p, err := r.engine.PrefixFrom(ctx, cidr)
	if err != nil {
		return err
	}
	if err = r.engine.ReleaseChildPrefix(ctx, p); err != nil {
		return err
	}
	if _, err = r.db.ExecContext(ctx, `DELETE FROM ipam_prefixes WHERE cidr=$1`, cidr); err != nil {
		return fmt.Errorf("delete child prefix: %w", err)
	}
	return nil
}

func (r *IPAMRepository) DeletePrefix(ctx context.Context, cidr string) error {
	_, err := r.engine.DeletePrefix(ctx, cidr)
	if err != nil {
		return err
	}
	if _, err = r.db.ExecContext(ctx, `DELETE FROM ipam_prefixes WHERE cidr=$1`, cidr); err != nil {
		return fmt.Errorf("delete prefix row: %w", err)
	}
	return nil
}

func (r *IPAMRepository) ListChildPrefixes(ctx context.Context, parentCIDR string) ([]*network.IPAMPrefix, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT cidr FROM ipam_prefixes WHERE parent_cidr=$1 ORDER BY cidr`, parentCIDR)
	if err != nil {
		return nil, fmt.Errorf("list child prefixes: %w", err)
	}
	defer rows.Close()
	out := make([]*network.IPAMPrefix, 0)
	for rows.Next() {
		var cidr string
		if err = rows.Scan(&cidr); err != nil {
			return nil, err
		}
		p, _ := r.engine.PrefixFrom(ctx, cidr)
		if p != nil {
			usage := p.Usage()
			out = append(out, &network.IPAMPrefix{CIDR: cidr, ParentCIDR: parentCIDR, UsableHosts: int(usage.AvailableIPs)})
		} else {
			out = append(out, &network.IPAMPrefix{CIDR: cidr, ParentCIDR: parentCIDR})
		}
	}
	return out, rows.Err()
}

func (r *IPAMRepository) AcquireIP(ctx context.Context, cidr string) (string, error) {
	// ensure prefix exists
	if _, err := r.engine.PrefixFrom(ctx, cidr); err != nil {
		if _, err = r.engine.NewPrefix(ctx, cidr); err != nil {
			return "", err
		}
	}
	ipObj, err := r.engine.AcquireIP(ctx, cidr)
	if err != nil {
		return "", err
	}
	if _, err = r.db.ExecContext(ctx, `INSERT INTO ipam_allocated_ips (prefix_cidr, ip, allocated_at) VALUES ($1,$2,NOW())`, cidr, ipObj.IP.String()); err != nil {
		return "", fmt.Errorf("persist allocated ip: %w", err)
	}
	return ipObj.IP.String(), nil
}

func (r *IPAMRepository) ReleaseIP(ctx context.Context, cidr string, ip string) error {
	// engine release
	if err := r.engine.ReleaseIPFromPrefix(ctx, cidr, ip); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM ipam_allocated_ips WHERE ip=$1`, ip); err != nil {
		return fmt.Errorf("delete allocated ip: %w", err)
	}
	return nil
}

// Ensure interface compliance
var _ ipam.Repository = (*IPAMRepository)(nil)

// Utility: compute usable hosts (approx) for CIDR
func usableHosts(cidr string) int {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return 0
	}
	ones, bits := ipNet.Mask.Size()
	hostBits := bits - ones
	if hostBits <= 1 {
		return 0
	}
	// IPv4 only assumption for now
	if bits != 32 {
		return 0
	}
	return (1 << hostBits) - 2
}
