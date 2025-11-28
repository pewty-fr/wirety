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

	// Insert route
	_, err = tx.ExecContext(ctx, `
		INSERT INTO routes (id, network_id, name, description, destination_cidr, jump_peer_id, domain_suffix, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, route.ID, networkID, route.Name, route.Description, route.DestinationCIDR, route.JumpPeerID, route.DomainSuffix, route.CreatedAt, route.UpdatedAt)
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

// GetRoute retrieves a route by ID
func (r *RouteRepository) GetRoute(ctx context.Context, networkID, routeID string) (*network.Route, error) {
	var route network.Route
	err := r.db.QueryRowContext(ctx, `
		SELECT id, network_id, name, description, destination_cidr, jump_peer_id, domain_suffix, created_at, updated_at
		FROM routes
		WHERE id = $1 AND network_id = $2
	`, routeID, networkID).Scan(&route.ID, &route.NetworkID, &route.Name, &route.Description, &route.DestinationCIDR, &route.JumpPeerID, &route.DomainSuffix, &route.CreatedAt, &route.UpdatedAt)
	if err != nil {
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
		SET name = $3, description = $4, destination_cidr = $5, jump_peer_id = $6, domain_suffix = $7, updated_at = $8
		WHERE id = $1 AND network_id = $2
	`, route.ID, networkID, route.Name, route.Description, route.DestinationCIDR, route.JumpPeerID, route.DomainSuffix, route.UpdatedAt)
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
		SELECT id, network_id, name, description, destination_cidr, jump_peer_id, domain_suffix, created_at, updated_at
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
		err = rows.Scan(&route.ID, &route.NetworkID, &route.Name, &route.Description, &route.DestinationCIDR, &route.JumpPeerID, &route.DomainSuffix, &route.CreatedAt, &route.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		routes = append(routes, &route)
	}

	return routes, rows.Err()
}

// GetRoutesForGroup retrieves all routes attached to a group
func (r *RouteRepository) GetRoutesForGroup(ctx context.Context, networkID, groupID string) ([]*network.Route, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.id, r.network_id, r.name, r.description, r.destination_cidr, r.jump_peer_id, r.domain_suffix, r.created_at, r.updated_at
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
		err = rows.Scan(&route.ID, &route.NetworkID, &route.Name, &route.Description, &route.DestinationCIDR, &route.JumpPeerID, &route.DomainSuffix, &route.CreatedAt, &route.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		routes = append(routes, &route)
	}

	return routes, rows.Err()
}

// GetRoutesByJumpPeer retrieves all routes that use a specific jump peer
func (r *RouteRepository) GetRoutesByJumpPeer(ctx context.Context, networkID, jumpPeerID string) ([]*network.Route, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, network_id, name, description, destination_cidr, jump_peer_id, domain_suffix, created_at, updated_at
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
		err = rows.Scan(&route.ID, &route.NetworkID, &route.Name, &route.Description, &route.DestinationCIDR, &route.JumpPeerID, &route.DomainSuffix, &route.CreatedAt, &route.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		routes = append(routes, &route)
	}

	return routes, rows.Err()
}
