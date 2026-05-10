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

// RouteRepository is a PostgreSQL implementation of network.RouteRepository
type RouteRepository struct {
	db *sql.DB
}

// NewRouteRepository constructs a new RouteRepository
func NewRouteRepository(db *sql.DB) *RouteRepository {
	return &RouteRepository{db: db}
}

// nullStr converts a Go string to a sql.NullString.  Empty string maps to NULL
// rather than the empty value — this is how we represent "no IPv4 part" or
// "no IPv6 part" for dual-stack-enabled tables (routes, dns_mappings) since
// migration 027.
func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// strFromNull converts a sql.NullString back to a plain Go string, mapping
// NULL → "".
func strFromNull(n sql.NullString) string {
	if !n.Valid {
		return ""
	}
	return n.String
}

// CreateRoute creates a new route in the database
func (r *RouteRepository) CreateRoute(ctx context.Context, networkID string, route *network.Route) error {
	now := time.Now()
	route.CreatedAt = now
	route.UpdatedAt = now

	// Set default domain suffix if not provided
	if route.DomainSuffix == "" {
		route.DomainSuffix = "internal"
	}

	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Verify jump peer exists and belongs to network
	var isJump bool
	err = tx.QueryRowContext(ctx, `
		SELECT is_jump FROM peers WHERE id = $1 AND network_id = $2
	`, route.JumpPeerID, networkID).Scan(&isJump)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("jump peer not found")
		}
		return fmt.Errorf("check jump peer: %w", err)
	}
	if !isJump {
		return fmt.Errorf("peer is not a jump peer")
	}

	// Insert route — both destination_cidr columns are NULLABLE since the
	// dual-stack migration (027), and the DB-level CHECK constraint ensures
	// at least one is set, but we trust the service layer to have validated
	// before reaching here.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO routes (id, network_id, name, description, destination_cidr, destination_cidr_v6, jump_peer_id, domain_suffix, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		route.ID, networkID, route.Name, route.Description,
		nullStr(route.DestinationCIDR), nullStr(route.DestinationCIDRv6),
		route.JumpPeerID, route.DomainSuffix, route.CreatedAt, route.UpdatedAt)
	if err != nil {
		// Check for unique constraint violation
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("route name already exists in network")
		}
		return fmt.Errorf("create route: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// scanRoute pulls a route row out of a Scanner with the new dual-stack columns.
// Centralised so all SELECTs read the same columns in the same order.
func scanRoute(s interface{ Scan(...interface{}) error }, route *network.Route) error {
	var cidr, cidrV6 sql.NullString
	if err := s.Scan(
		&route.ID, &route.NetworkID, &route.Name, &route.Description,
		&cidr, &cidrV6,
		&route.JumpPeerID, &route.DomainSuffix, &route.CreatedAt, &route.UpdatedAt,
	); err != nil {
		return err
	}
	route.DestinationCIDR = strFromNull(cidr)
	route.DestinationCIDRv6 = strFromNull(cidrV6)
	return nil
}

// routeColumns is the column list every SELECT * for routes must use, in the
// order scanRoute expects.
const routeColumns = "id, network_id, name, description, destination_cidr, destination_cidr_v6, jump_peer_id, domain_suffix, created_at, updated_at"

// GetRoute retrieves a route by ID
func (r *RouteRepository) GetRoute(ctx context.Context, networkID, routeID string) (*network.Route, error) {
	var route network.Route
	row := r.db.QueryRowContext(ctx, `
		SELECT `+routeColumns+`
		FROM routes
		WHERE id = $1 AND network_id = $2
	`, routeID, networkID)
	if err := scanRoute(row, &route); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("route not found")
		}
		return nil, fmt.Errorf("get route: %w", err)
	}
	return &route, nil
}

// UpdateRoute updates an existing route
func (r *RouteRepository) UpdateRoute(ctx context.Context, networkID string, route *network.Route) error {
	route.UpdatedAt = time.Now()

	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// If jump peer is being updated, verify it exists and is a jump peer
	if route.JumpPeerID != "" {
		var isJump bool
		err = tx.QueryRowContext(ctx, `
			SELECT is_jump FROM peers WHERE id = $1 AND network_id = $2
		`, route.JumpPeerID, networkID).Scan(&isJump)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("jump peer not found")
			}
			return fmt.Errorf("check jump peer: %w", err)
		}
		if !isJump {
			return fmt.Errorf("peer is not a jump peer")
		}
	}

	// Update route
	res, err := tx.ExecContext(ctx, `
		UPDATE routes
		SET name = $3, description = $4, destination_cidr = $5, destination_cidr_v6 = $6, jump_peer_id = $7, domain_suffix = $8, updated_at = $9
		WHERE id = $1 AND network_id = $2
	`,
		route.ID, networkID, route.Name, route.Description,
		nullStr(route.DestinationCIDR), nullStr(route.DestinationCIDRv6),
		route.JumpPeerID, route.DomainSuffix, route.UpdatedAt)
	if err != nil {
		// Check for unique constraint violation
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("route name already exists in network")
		}
		return fmt.Errorf("update route: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("route not found")
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// DeleteRoute deletes a route
func (r *RouteRepository) DeleteRoute(ctx context.Context, networkID, routeID string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM routes
		WHERE id = $1 AND network_id = $2
	`, routeID, networkID)
	if err != nil {
		return fmt.Errorf("delete route: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("route not found")
	}

	return nil
}

// ListRoutes lists all routes in a network
func (r *RouteRepository) ListRoutes(ctx context.Context, networkID string) ([]*network.Route, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+routeColumns+`
		FROM routes
		WHERE network_id = $1
		ORDER BY created_at ASC
	`, networkID)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	routes := make([]*network.Route, 0)
	for rows.Next() {
		var route network.Route
		if err := scanRoute(rows, &route); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		routes = append(routes, &route)
	}

	return routes, rows.Err()
}

// GetRoutesForGroup retrieves all routes attached to a group
func (r *RouteRepository) GetRoutesForGroup(ctx context.Context, networkID, groupID string) ([]*network.Route, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.id, r.network_id, r.name, r.description, r.destination_cidr, r.destination_cidr_v6, r.jump_peer_id, r.domain_suffix, r.created_at, r.updated_at
		FROM routes r
		INNER JOIN group_routes gr ON r.id = gr.route_id
		WHERE gr.group_id = $1 AND r.network_id = $2
		ORDER BY r.created_at ASC
	`, groupID, networkID)
	if err != nil {
		return nil, fmt.Errorf("get routes for group: %w", err)
	}
	defer func() { _ = rows.Close() }()

	routes := make([]*network.Route, 0)
	for rows.Next() {
		var route network.Route
		if err := scanRoute(rows, &route); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		routes = append(routes, &route)
	}

	return routes, rows.Err()
}

// GetRoutesByJumpPeer retrieves all routes that use a specific jump peer
func (r *RouteRepository) GetRoutesByJumpPeer(ctx context.Context, networkID, jumpPeerID string) ([]*network.Route, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+routeColumns+`
		FROM routes
		WHERE jump_peer_id = $1 AND network_id = $2
		ORDER BY created_at ASC
	`, jumpPeerID, networkID)
	if err != nil {
		return nil, fmt.Errorf("get routes by jump peer: %w", err)
	}
	defer func() { _ = rows.Close() }()

	routes := make([]*network.Route, 0)
	for rows.Next() {
		var route network.Route
		if err := scanRoute(rows, &route); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		routes = append(routes, &route)
	}

	return routes, rows.Err()
}
