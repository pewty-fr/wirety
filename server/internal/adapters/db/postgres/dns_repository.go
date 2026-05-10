package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"wirety/internal/domain/network"

	"github.com/lib/pq"
)

// DNSRepository is a PostgreSQL implementation of network.DNSRepository
type DNSRepository struct {
	db *sql.DB
}

// NewDNSRepository constructs a new DNSRepository
func NewDNSRepository(db *sql.DB) *DNSRepository {
	return &DNSRepository{db: db}
}

// dnsMappingColumns is the column list every SELECT for dns_mappings must use,
// in the order scanDNSMapping expects.  Keeping it centralised stops drift
// between LIST and GET when adding new columns (like the v6 work in
// migration 027).
const dnsMappingColumns = "id, route_id, name, ip_address, ip_address_v6, created_at, updated_at"

// scanDNSMapping pulls a row out of a Scanner.  Both ip columns are NULLABLE
// since migration 027 — at least one is always set, but we don't assume which.
func scanDNSMapping(s interface{ Scan(...interface{}) error }, m *network.DNSMapping) error {
	var ip4, ip6 sql.NullString
	if err := s.Scan(&m.ID, &m.RouteID, &m.Name, &ip4, &ip6, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return err
	}
	m.IPAddress = strFromNull(ip4)
	m.IPv6Address = strFromNull(ip6)
	return nil
}

// validateAgainstRoute checks that the mapping's IPv4/IPv6 addresses sit inside
// the corresponding family of the route's destination CIDR(s).  An IPv4 mapping
// requires the route to have a v4 CIDR; an IPv6 mapping requires a v6 CIDR.
func validateAgainstRoute(tx *sql.Tx, ctx context.Context, routeID string, mapping *network.DNSMapping) error {
	var cidr, cidrV6 sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT destination_cidr, destination_cidr_v6 FROM routes WHERE id = $1
	`, routeID).Scan(&cidr, &cidrV6)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("route not found")
		}
		return fmt.Errorf("check route: %w", err)
	}
	if mapping.IPAddress != "" {
		if !cidr.Valid {
			return fmt.Errorf("ip_address: route has no IPv4 destination CIDR")
		}
		if err := network.ValidateIPInCIDR(mapping.IPAddress, cidr.String); err != nil {
			return fmt.Errorf("ip_address validation failed: %w", err)
		}
	}
	if mapping.IPv6Address != "" {
		if !cidrV6.Valid {
			return fmt.Errorf("ip_address_v6: route has no IPv6 destination CIDR")
		}
		if err := network.ValidateIPInCIDR(mapping.IPv6Address, cidrV6.String); err != nil {
			return fmt.Errorf("ip_address_v6 validation failed: %w", err)
		}
	}
	return nil
}

// CreateDNSMapping creates a new DNS mapping in the database
func (r *DNSRepository) CreateDNSMapping(ctx context.Context, routeID string, mapping *network.DNSMapping) error {
	now := time.Now()
	mapping.CreatedAt = now
	mapping.UpdatedAt = now

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := validateAgainstRoute(tx, ctx, routeID, mapping); err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO dns_mappings (id, route_id, name, ip_address, ip_address_v6, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		mapping.ID, routeID, mapping.Name,
		nullStr(mapping.IPAddress), nullStr(mapping.IPv6Address),
		mapping.CreatedAt, mapping.UpdatedAt)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("DNS name already exists for route")
		}
		return fmt.Errorf("create DNS mapping: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetDNSMapping retrieves a DNS mapping by ID
func (r *DNSRepository) GetDNSMapping(ctx context.Context, routeID, mappingID string) (*network.DNSMapping, error) {
	var mapping network.DNSMapping
	row := r.db.QueryRowContext(ctx, `
		SELECT `+dnsMappingColumns+`
		FROM dns_mappings
		WHERE id = $1 AND route_id = $2
	`, mappingID, routeID)
	if err := scanDNSMapping(row, &mapping); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("DNS mapping not found")
		}
		return nil, fmt.Errorf("get DNS mapping: %w", err)
	}
	return &mapping, nil
}

// UpdateDNSMapping updates an existing DNS mapping
func (r *DNSRepository) UpdateDNSMapping(ctx context.Context, routeID string, mapping *network.DNSMapping) error {
	mapping.UpdatedAt = time.Now()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := validateAgainstRoute(tx, ctx, routeID, mapping); err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE dns_mappings
		SET name = $3, ip_address = $4, ip_address_v6 = $5, updated_at = $6
		WHERE id = $1 AND route_id = $2
	`,
		mapping.ID, routeID, mapping.Name,
		nullStr(mapping.IPAddress), nullStr(mapping.IPv6Address),
		mapping.UpdatedAt)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("DNS name already exists for route")
		}
		return fmt.Errorf("update DNS mapping: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("DNS mapping not found")
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// DeleteDNSMapping deletes a DNS mapping
func (r *DNSRepository) DeleteDNSMapping(ctx context.Context, routeID, mappingID string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM dns_mappings
		WHERE id = $1 AND route_id = $2
	`, mappingID, routeID)
	if err != nil {
		return fmt.Errorf("delete DNS mapping: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("DNS mapping not found")
	}

	return nil
}

// ListDNSMappings lists all DNS mappings for a route
func (r *DNSRepository) ListDNSMappings(ctx context.Context, routeID string) ([]*network.DNSMapping, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+dnsMappingColumns+`
		FROM dns_mappings
		WHERE route_id = $1
		ORDER BY created_at ASC
	`, routeID)
	if err != nil {
		return nil, fmt.Errorf("list DNS mappings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	mappings := make([]*network.DNSMapping, 0)
	for rows.Next() {
		var mapping network.DNSMapping
		if err := scanDNSMapping(rows, &mapping); err != nil {
			return nil, fmt.Errorf("scan DNS mapping: %w", err)
		}
		mappings = append(mappings, &mapping)
	}

	return mappings, rows.Err()
}

// GetNetworkDNSMappings retrieves all DNS mappings for a network (for DNS server configuration)
func (r *DNSRepository) GetNetworkDNSMappings(ctx context.Context, networkID string) ([]*network.DNSMapping, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT dm.id, dm.route_id, dm.name, dm.ip_address, dm.ip_address_v6, dm.created_at, dm.updated_at
		FROM dns_mappings dm
		INNER JOIN routes r ON dm.route_id = r.id
		WHERE r.network_id = $1
		ORDER BY dm.created_at ASC
	`, networkID)
	if err != nil {
		return nil, fmt.Errorf("get network DNS mappings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	mappings := make([]*network.DNSMapping, 0)
	for rows.Next() {
		var mapping network.DNSMapping
		if err := scanDNSMapping(rows, &mapping); err != nil {
			return nil, fmt.Errorf("scan DNS mapping: %w", err)
		}
		mappings = append(mappings, &mapping)
	}

	return mappings, rows.Err()
}
