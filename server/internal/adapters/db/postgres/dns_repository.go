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

// CreateDNSMapping creates a new DNS mapping in the database
func (r *DNSRepository) CreateDNSMapping(ctx context.Context, routeID string, mapping *network.DNSMapping) error {
	now := time.Now()
	mapping.CreatedAt = now
	mapping.UpdatedAt = now

	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Verify route exists and get its destination CIDR
	var destinationCIDR string
	err = tx.QueryRowContext(ctx, `
		SELECT destination_cidr FROM routes WHERE id = $1
	`, routeID).Scan(&destinationCIDR)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("route not found")
		}
		return fmt.Errorf("check route: %w", err)
	}

	// Validate IP address is within route's CIDR
	if err := network.ValidateIPInCIDR(mapping.IPAddress, destinationCIDR); err != nil {
		return fmt.Errorf("IP address validation failed: %w", err)
	}

	// Insert DNS mapping
	_, err = tx.ExecContext(ctx, `
		INSERT INTO dns_mappings (id, route_id, name, ip_address, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, mapping.ID, routeID, mapping.Name, mapping.IPAddress, mapping.CreatedAt, mapping.UpdatedAt)
	if err != nil {
		// Check for unique constraint violation
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
	err := r.db.QueryRowContext(ctx, `
		SELECT id, route_id, name, ip_address, created_at, updated_at
		FROM dns_mappings
		WHERE id = $1 AND route_id = $2
	`, mappingID, routeID).Scan(&mapping.ID, &mapping.RouteID, &mapping.Name, &mapping.IPAddress, &mapping.CreatedAt, &mapping.UpdatedAt)
	if err != nil {
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

	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Verify route exists and get its destination CIDR
	var destinationCIDR string
	err = tx.QueryRowContext(ctx, `
		SELECT destination_cidr FROM routes WHERE id = $1
	`, routeID).Scan(&destinationCIDR)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("route not found")
		}
		return fmt.Errorf("check route: %w", err)
	}

	// Validate IP address is within route's CIDR
	if err := network.ValidateIPInCIDR(mapping.IPAddress, destinationCIDR); err != nil {
		return fmt.Errorf("IP address validation failed: %w", err)
	}

	// Update DNS mapping
	res, err := tx.ExecContext(ctx, `
		UPDATE dns_mappings
		SET name = $3, ip_address = $4, updated_at = $5
		WHERE id = $1 AND route_id = $2
	`, mapping.ID, routeID, mapping.Name, mapping.IPAddress, mapping.UpdatedAt)
	if err != nil {
		// Check for unique constraint violation
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
		SELECT id, route_id, name, ip_address, created_at, updated_at
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
		err = rows.Scan(&mapping.ID, &mapping.RouteID, &mapping.Name, &mapping.IPAddress, &mapping.CreatedAt, &mapping.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan DNS mapping: %w", err)
		}
		mappings = append(mappings, &mapping)
	}

	return mappings, rows.Err()
}

// GetNetworkDNSMappings retrieves all DNS mappings for a network (for DNS server configuration)
func (r *DNSRepository) GetNetworkDNSMappings(ctx context.Context, networkID string) ([]*network.DNSMapping, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT dm.id, dm.route_id, dm.name, dm.ip_address, dm.created_at, dm.updated_at
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
		err = rows.Scan(&mapping.ID, &mapping.RouteID, &mapping.Name, &mapping.IPAddress, &mapping.CreatedAt, &mapping.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan DNS mapping: %w", err)
		}
		mappings = append(mappings, &mapping)
	}

	return mappings, rows.Err()
}
