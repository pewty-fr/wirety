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

// GroupRepository is a PostgreSQL implementation of network.GroupRepository
type GroupRepository struct {
	db *sql.DB
}

// NewGroupRepository constructs a new GroupRepository
func NewGroupRepository(db *sql.DB) *GroupRepository {
	return &GroupRepository{db: db}
}

// CreateGroup creates a new group in the database
func (r *GroupRepository) CreateGroup(ctx context.Context, networkID string, group *network.Group) error {
	now := time.Now()
	group.CreatedAt = now
	group.UpdatedAt = now

	// Ensure slices are never nil to avoid database constraint violations
	if group.PeerIDs == nil {
		group.PeerIDs = []string{}
	}
	if group.PolicyIDs == nil {
		group.PolicyIDs = []string{}
	}
	if group.RouteIDs == nil {
		group.RouteIDs = []string{}
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO groups (id, network_id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, group.ID, networkID, group.Name, group.Description, group.CreatedAt, group.UpdatedAt)
	if err != nil {
		// Check for unique constraint violation
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("group name already exists in network")
		}
		return fmt.Errorf("create group: %w", err)
	}

	return nil
}

// GetGroup retrieves a group by ID
func (r *GroupRepository) GetGroup(ctx context.Context, networkID, groupID string) (*network.Group, error) {
	var g network.Group
	err := r.db.QueryRowContext(ctx, `
		SELECT id, network_id, name, description, created_at, updated_at
		FROM groups
		WHERE id = $1 AND network_id = $2
	`, groupID, networkID).Scan(&g.ID, &g.NetworkID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("group not found")
		}
		return nil, fmt.Errorf("get group: %w", err)
	}

	// Load peer IDs
	peerIDs, err := r.loadGroupPeerIDs(ctx, groupID)
	if err != nil {
		return nil, err
	}
	g.PeerIDs = peerIDs

	// Load policy IDs
	policyIDs, err := r.loadGroupPolicyIDs(ctx, groupID)
	if err != nil {
		return nil, err
	}
	g.PolicyIDs = policyIDs

	// Load route IDs
	routeIDs, err := r.loadGroupRouteIDs(ctx, groupID)
	if err != nil {
		return nil, err
	}
	g.RouteIDs = routeIDs

	return &g, nil
}

// UpdateGroup updates an existing group
func (r *GroupRepository) UpdateGroup(ctx context.Context, networkID string, group *network.Group) error {
	group.UpdatedAt = time.Now()

	res, err := r.db.ExecContext(ctx, `
		UPDATE groups
		SET name = $3, description = $4, updated_at = $5
		WHERE id = $1 AND network_id = $2
	`, group.ID, networkID, group.Name, group.Description, group.UpdatedAt)
	if err != nil {
		// Check for unique constraint violation
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("group name already exists in network")
		}
		return fmt.Errorf("update group: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("group not found")
	}

	return nil
}

// DeleteGroup deletes a group
func (r *GroupRepository) DeleteGroup(ctx context.Context, networkID, groupID string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM groups
		WHERE id = $1 AND network_id = $2
	`, groupID, networkID)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("group not found")
	}

	return nil
}

// ListGroups lists all groups in a network
func (r *GroupRepository) ListGroups(ctx context.Context, networkID string) ([]*network.Group, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT g.id, g.network_id, g.name, g.description, g.created_at, g.updated_at,
		       COALESCE(p.peer_count, 0) AS peert
		FROM groups g
		LEFT JOIN (
			SELECT group_id, COUNT(*) AS peer_count
			FROM group_peers
			GROUP BY group_id
		) p ON p.group_id = g.id
		WHERE g.network_id = $1
		ORDER BY g.created_at ASC
	`, networkID)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer func() { _ = rows.Close() }()

	groups := make([]*network.Group, 0)
	for rows.Next() {
		var g network.Group
		var peerCount int
		err = rows.Scan(&g.ID, &g.NetworkID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt, &peerCount)
		if err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}

		// Load peer IDs
		peerIDs, err := r.loadGroupPeerIDs(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		g.PeerIDs = peerIDs

		// Load policy IDs
		policyIDs, err := r.loadGroupPolicyIDs(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		g.PolicyIDs = policyIDs

		// Load route IDs
		routeIDs, err := r.loadGroupRouteIDs(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		g.RouteIDs = routeIDs

		groups = append(groups, &g)
	}

	return groups, rows.Err()
}

// AddPeerToGroup adds a peer to a group
func (r *GroupRepository) AddPeerToGroup(ctx context.Context, networkID, groupID, peerID string) error {
	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Verify group exists and belongs to network
	var exists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM groups WHERE id = $1 AND network_id = $2)
	`, groupID, networkID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check group exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("group not found")
	}

	// Verify peer exists and belongs to network
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM peers WHERE id = $1 AND network_id = $2)
	`, peerID, networkID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check peer exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("peer not found")
	}

	// Add peer to group (ignore if already exists)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO group_peers (group_id, peer_id, added_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (group_id, peer_id) DO NOTHING
	`, groupID, peerID, time.Now())
	if err != nil {
		return fmt.Errorf("add peer to group: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// RemovePeerFromGroup removes a peer from a group
func (r *GroupRepository) RemovePeerFromGroup(ctx context.Context, networkID, groupID, peerID string) error {
	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Verify group exists and belongs to network
	var exists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM groups WHERE id = $1 AND network_id = $2)
	`, groupID, networkID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check group exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("group not found")
	}

	// Remove peer from group
	res, err := tx.ExecContext(ctx, `
		DELETE FROM group_peers
		WHERE group_id = $1 AND peer_id = $2
	`, groupID, peerID)
	if err != nil {
		return fmt.Errorf("remove peer from group: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("peer not in group")
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetPeerGroups retrieves all groups a peer belongs to
func (r *GroupRepository) GetPeerGroups(ctx context.Context, networkID, peerID string) ([]*network.Group, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT g.id, g.network_id, g.name, g.description, g.created_at, g.updated_at
		FROM groups g
		INNER JOIN group_peers gp ON g.id = gp.group_id
		WHERE gp.peer_id = $1 AND g.network_id = $2
		ORDER BY g.created_at ASC
	`, peerID, networkID)
	if err != nil {
		return nil, fmt.Errorf("get peer groups: %w", err)
	}
	defer func() { _ = rows.Close() }()

	groups := make([]*network.Group, 0)
	for rows.Next() {
		var g network.Group
		err = rows.Scan(&g.ID, &g.NetworkID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}

		// Load peer IDs
		peerIDs, err := r.loadGroupPeerIDs(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		g.PeerIDs = peerIDs

		// Load policy IDs
		policyIDs, err := r.loadGroupPolicyIDs(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		g.PolicyIDs = policyIDs

		// Load route IDs
		routeIDs, err := r.loadGroupRouteIDs(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		g.RouteIDs = routeIDs

		groups = append(groups, &g)
	}

	return groups, rows.Err()
}

// AttachPolicyToGroup attaches a policy to a group
func (r *GroupRepository) AttachPolicyToGroup(ctx context.Context, networkID, groupID, policyID string) error {
	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Verify group exists and belongs to network
	var exists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM groups WHERE id = $1 AND network_id = $2)
	`, groupID, networkID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check group exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("group not found")
	}

	// Verify policy exists and belongs to network
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM policies WHERE id = $1 AND network_id = $2)
	`, policyID, networkID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check policy exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("policy not found")
	}

	// Get the next policy order
	var maxOrder sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		SELECT MAX(policy_order) FROM group_policies WHERE group_id = $1
	`, groupID).Scan(&maxOrder)
	if err != nil {
		return fmt.Errorf("get max policy order: %w", err)
	}

	nextOrder := 0
	if maxOrder.Valid {
		nextOrder = int(maxOrder.Int64) + 1
	}

	// Attach policy to group (ignore if already attached)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO group_policies (group_id, policy_id, attached_at, policy_order)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (group_id, policy_id) DO NOTHING
	`, groupID, policyID, time.Now(), nextOrder)
	if err != nil {
		return fmt.Errorf("attach policy to group: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// DetachPolicyFromGroup detaches a policy from a group
func (r *GroupRepository) DetachPolicyFromGroup(ctx context.Context, networkID, groupID, policyID string) error {
	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Verify group exists and belongs to network
	var exists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM groups WHERE id = $1 AND network_id = $2)
	`, groupID, networkID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check group exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("group not found")
	}

	// Detach policy from group
	res, err := tx.ExecContext(ctx, `
		DELETE FROM group_policies
		WHERE group_id = $1 AND policy_id = $2
	`, groupID, policyID)
	if err != nil {
		return fmt.Errorf("detach policy from group: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("policy not attached to group")
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetGroupPolicies retrieves all policies attached to a group
func (r *GroupRepository) GetGroupPolicies(ctx context.Context, networkID, groupID string) ([]*network.Policy, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.network_id, p.name, p.description, p.created_at, p.updated_at
		FROM policies p
		INNER JOIN group_policies gp ON p.id = gp.policy_id
		WHERE gp.group_id = $1 AND p.network_id = $2
		ORDER BY gp.policy_order ASC
	`, groupID, networkID)
	if err != nil {
		return nil, fmt.Errorf("get group policies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	policies := make([]*network.Policy, 0)
	for rows.Next() {
		var p network.Policy
		err = rows.Scan(&p.ID, &p.NetworkID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan policy: %w", err)
		}

		// Load policy rules
		rules, err := r.loadPolicyRules(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		p.Rules = rules

		policies = append(policies, &p)
	}

	return policies, rows.Err()
}

// AttachRouteToGroup attaches a route to a group
func (r *GroupRepository) AttachRouteToGroup(ctx context.Context, networkID, groupID, routeID string) error {
	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Verify group exists and belongs to network
	var exists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM groups WHERE id = $1 AND network_id = $2)
	`, groupID, networkID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check group exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("group not found")
	}

	// Verify route exists and belongs to network
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM routes WHERE id = $1 AND network_id = $2)
	`, routeID, networkID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check route exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("route not found")
	}

	// Attach route to group (ignore if already attached)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO group_routes (group_id, route_id, attached_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (group_id, route_id) DO NOTHING
	`, groupID, routeID, time.Now())
	if err != nil {
		return fmt.Errorf("attach route to group: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// DetachRouteFromGroup detaches a route from a group
func (r *GroupRepository) DetachRouteFromGroup(ctx context.Context, networkID, groupID, routeID string) error {
	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Verify group exists and belongs to network
	var exists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM groups WHERE id = $1 AND network_id = $2)
	`, groupID, networkID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check group exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("group not found")
	}

	// Detach route from group
	res, err := tx.ExecContext(ctx, `
		DELETE FROM group_routes
		WHERE group_id = $1 AND route_id = $2
	`, groupID, routeID)
	if err != nil {
		return fmt.Errorf("detach route from group: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("route not attached to group")
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetGroupRoutes retrieves all routes attached to a group
func (r *GroupRepository) GetGroupRoutes(ctx context.Context, networkID, groupID string) ([]*network.Route, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.id, r.network_id, r.name, r.description, r.destination_cidr, r.jump_peer_id, r.domain_suffix, r.created_at, r.updated_at
		FROM routes r
		INNER JOIN group_routes gr ON r.id = gr.route_id
		WHERE gr.group_id = $1 AND r.network_id = $2
		ORDER BY r.created_at ASC
	`, groupID, networkID)
	if err != nil {
		return nil, fmt.Errorf("get group routes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	routes := make([]*network.Route, 0)
	for rows.Next() {
		var r network.Route
		err = rows.Scan(&r.ID, &r.NetworkID, &r.Name, &r.Description, &r.DestinationCIDR, &r.JumpPeerID, &r.DomainSuffix, &r.CreatedAt, &r.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		routes = append(routes, &r)
	}

	return routes, rows.Err()
}

// Helper functions

func (r *GroupRepository) loadGroupPeerIDs(ctx context.Context, groupID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT peer_id FROM group_peers WHERE group_id = $1 ORDER BY added_at ASC
	`, groupID)
	if err != nil {
		return nil, fmt.Errorf("load group peer IDs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	peerIDs := make([]string, 0)
	for rows.Next() {
		var peerID string
		if err = rows.Scan(&peerID); err != nil {
			return nil, fmt.Errorf("scan peer ID: %w", err)
		}
		peerIDs = append(peerIDs, peerID)
	}

	return peerIDs, rows.Err()
}

func (r *GroupRepository) loadGroupPolicyIDs(ctx context.Context, groupID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT policy_id FROM group_policies WHERE group_id = $1 ORDER BY policy_order ASC
	`, groupID)
	if err != nil {
		return nil, fmt.Errorf("load group policy IDs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	policyIDs := make([]string, 0)
	for rows.Next() {
		var policyID string
		if err = rows.Scan(&policyID); err != nil {
			return nil, fmt.Errorf("scan policy ID: %w", err)
		}
		policyIDs = append(policyIDs, policyID)
	}

	return policyIDs, rows.Err()
}

func (r *GroupRepository) loadGroupRouteIDs(ctx context.Context, groupID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT route_id FROM group_routes WHERE group_id = $1 ORDER BY attached_at ASC
	`, groupID)
	if err != nil {
		return nil, fmt.Errorf("load group route IDs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	routeIDs := make([]string, 0)
	for rows.Next() {
		var routeID string
		if err = rows.Scan(&routeID); err != nil {
			return nil, fmt.Errorf("scan route ID: %w", err)
		}
		routeIDs = append(routeIDs, routeID)
	}

	return routeIDs, rows.Err()
}

func (r *GroupRepository) loadPolicyRules(ctx context.Context, policyID string) ([]network.PolicyRule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, direction, action, target, target_type, description
		FROM policy_rules
		WHERE policy_id = $1
		ORDER BY rule_order ASC
	`, policyID)
	if err != nil {
		return nil, fmt.Errorf("load policy rules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	rules := make([]network.PolicyRule, 0)
	for rows.Next() {
		var rule network.PolicyRule
		err = rows.Scan(&rule.ID, &rule.Direction, &rule.Action, &rule.Target, &rule.TargetType, &rule.Description)
		if err != nil {
			return nil, fmt.Errorf("scan policy rule: %w", err)
		}
		rules = append(rules, rule)
	}

	return rules, rows.Err()
}
