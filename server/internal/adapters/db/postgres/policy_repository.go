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

// PolicyRepository is a PostgreSQL implementation of network.PolicyRepository
type PolicyRepository struct {
	db *sql.DB
}

// NewPolicyRepository constructs a new PolicyRepository
func NewPolicyRepository(db *sql.DB) *PolicyRepository {
	return &PolicyRepository{db: db}
}

// CreatePolicy creates a new policy in the database
func (r *PolicyRepository) CreatePolicy(ctx context.Context, networkID string, policy *network.Policy) error {
	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	// Ensure rules slice is never nil
	if policy.Rules == nil {
		policy.Rules = []network.PolicyRule{}
	}

	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Insert policy
	_, err = tx.ExecContext(ctx, `
		INSERT INTO policies (id, network_id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, policy.ID, networkID, policy.Name, policy.Description, policy.CreatedAt, policy.UpdatedAt)
	if err != nil {
		// Check for unique constraint violation
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("policy name already exists in network")
		}
		return fmt.Errorf("create policy: %w", err)
	}

	// Insert rules if any
	for i, rule := range policy.Rules {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO policy_rules (id, policy_id, direction, action, target, target_type, description, rule_order, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, rule.ID, policy.ID, rule.Direction, rule.Action, rule.Target, rule.TargetType, rule.Description, i, now)
		if err != nil {
			return fmt.Errorf("create policy rule: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetPolicy retrieves a policy by ID
func (r *PolicyRepository) GetPolicy(ctx context.Context, networkID, policyID string) (*network.Policy, error) {
	var p network.Policy
	err := r.db.QueryRowContext(ctx, `
		SELECT id, network_id, name, description, created_at, updated_at
		FROM policies
		WHERE id = $1 AND network_id = $2
	`, policyID, networkID).Scan(&p.ID, &p.NetworkID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("policy not found")
		}
		return nil, fmt.Errorf("get policy: %w", err)
	}

	// Load policy rules
	rules, err := r.loadPolicyRules(ctx, policyID)
	if err != nil {
		return nil, err
	}
	p.Rules = rules

	return &p, nil
}

// UpdatePolicy updates an existing policy
func (r *PolicyRepository) UpdatePolicy(ctx context.Context, networkID string, policy *network.Policy) error {
	policy.UpdatedAt = time.Now()

	res, err := r.db.ExecContext(ctx, `
		UPDATE policies
		SET name = $3, description = $4, updated_at = $5
		WHERE id = $1 AND network_id = $2
	`, policy.ID, networkID, policy.Name, policy.Description, policy.UpdatedAt)
	if err != nil {
		// Check for unique constraint violation
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("policy name already exists in network")
		}
		return fmt.Errorf("update policy: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("policy not found")
	}

	return nil
}

// DeletePolicy deletes a policy
func (r *PolicyRepository) DeletePolicy(ctx context.Context, networkID, policyID string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM policies
		WHERE id = $1 AND network_id = $2
	`, policyID, networkID)
	if err != nil {
		return fmt.Errorf("delete policy: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("policy not found")
	}

	return nil
}

// ListPolicies lists all policies in a network
func (r *PolicyRepository) ListPolicies(ctx context.Context, networkID string) ([]*network.Policy, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, network_id, name, description, created_at, updated_at
		FROM policies
		WHERE network_id = $1
		ORDER BY created_at ASC
	`, networkID)
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
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

// AddRuleToPolicy adds a rule to a policy
func (r *PolicyRepository) AddRuleToPolicy(ctx context.Context, networkID, policyID string, rule *network.PolicyRule) error {
	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Verify policy exists and belongs to network
	var exists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM policies WHERE id = $1 AND network_id = $2)
	`, policyID, networkID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check policy exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("policy not found")
	}

	// Get the next rule order
	var maxOrder sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		SELECT MAX(rule_order) FROM policy_rules WHERE policy_id = $1
	`, policyID).Scan(&maxOrder)
	if err != nil {
		return fmt.Errorf("get max rule order: %w", err)
	}

	nextOrder := 0
	if maxOrder.Valid {
		nextOrder = int(maxOrder.Int64) + 1
	}

	// Insert rule
	_, err = tx.ExecContext(ctx, `
		INSERT INTO policy_rules (id, policy_id, direction, action, target, target_type, description, rule_order, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, rule.ID, policyID, rule.Direction, rule.Action, rule.Target, rule.TargetType, rule.Description, nextOrder, time.Now())
	if err != nil {
		return fmt.Errorf("add rule to policy: %w", err)
	}

	// Update policy updated_at timestamp
	_, err = tx.ExecContext(ctx, `
		UPDATE policies SET updated_at = $2 WHERE id = $1
	`, policyID, time.Now())
	if err != nil {
		return fmt.Errorf("update policy timestamp: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// RemoveRuleFromPolicy removes a rule from a policy
func (r *PolicyRepository) RemoveRuleFromPolicy(ctx context.Context, networkID, policyID, ruleID string) error {
	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Verify policy exists and belongs to network
	var exists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM policies WHERE id = $1 AND network_id = $2)
	`, policyID, networkID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check policy exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("policy not found")
	}

	// Remove rule
	res, err := tx.ExecContext(ctx, `
		DELETE FROM policy_rules
		WHERE id = $1 AND policy_id = $2
	`, ruleID, policyID)
	if err != nil {
		return fmt.Errorf("remove rule from policy: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("rule not found")
	}

	// Update policy updated_at timestamp
	_, err = tx.ExecContext(ctx, `
		UPDATE policies SET updated_at = $2 WHERE id = $1
	`, policyID, time.Now())
	if err != nil {
		return fmt.Errorf("update policy timestamp: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// UpdateRule updates an existing rule
func (r *PolicyRepository) UpdateRule(ctx context.Context, networkID, policyID string, rule *network.PolicyRule) error {
	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Verify policy exists and belongs to network
	var exists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM policies WHERE id = $1 AND network_id = $2)
	`, policyID, networkID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check policy exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("policy not found")
	}

	// Update rule
	res, err := tx.ExecContext(ctx, `
		UPDATE policy_rules
		SET direction = $3, action = $4, target = $5, target_type = $6, description = $7
		WHERE id = $1 AND policy_id = $2
	`, rule.ID, policyID, rule.Direction, rule.Action, rule.Target, rule.TargetType, rule.Description)
	if err != nil {
		return fmt.Errorf("update rule: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("rule not found")
	}

	// Update policy updated_at timestamp
	_, err = tx.ExecContext(ctx, `
		UPDATE policies SET updated_at = $2 WHERE id = $1
	`, policyID, time.Now())
	if err != nil {
		return fmt.Errorf("update policy timestamp: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetPoliciesForGroup retrieves all policies attached to a group
func (r *PolicyRepository) GetPoliciesForGroup(ctx context.Context, networkID, groupID string) ([]*network.Policy, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.network_id, p.name, p.description, p.created_at, p.updated_at
		FROM policies p
		INNER JOIN group_policies gp ON p.id = gp.policy_id
		WHERE gp.group_id = $1 AND p.network_id = $2
		ORDER BY gp.policy_order ASC
	`, groupID, networkID)
	if err != nil {
		return nil, fmt.Errorf("get policies for group: %w", err)
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

// Helper functions

func (r *PolicyRepository) loadPolicyRules(ctx context.Context, policyID string) ([]network.PolicyRule, error) {
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
