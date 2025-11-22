package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"wirety/internal/domain/auth"

	"github.com/lib/pq"
)

// UserRepository is a Postgres implementation of auth.Repository
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository constructs a UserRepository
func NewUserRepository(db *sql.DB) *UserRepository { return &UserRepository{db: db} }

// scanUser scans a user row into an auth.User
func scanUser(rows scanner) (*auth.User, error) {
	var u auth.User
	var networks []string
	var lastLogin sql.NullTime
	err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, pq.Array(&networks), &u.CreatedAt, &u.UpdatedAt, &lastLogin)
	if err != nil {
		return nil, err
	}
	u.AuthorizedNetworks = networks
	if lastLogin.Valid {
		u.LastLoginAt = lastLogin.Time
	}
	return &u, nil
}

// scanner is implemented by *sql.Row and *sql.Rows
type scanner interface {
	Scan(dest ...interface{}) error
}

func (r *UserRepository) GetUser(userID string) (*auth.User, error) {
	row := r.db.QueryRow(`SELECT id,email,name,role,authorized_networks,created_at,updated_at,last_login_at FROM users WHERE id=$1`, userID)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	return u, nil
}

func (r *UserRepository) GetUserByEmail(email string) (*auth.User, error) {
	row := r.db.QueryRow(`SELECT id,email,name,role,authorized_networks,created_at,updated_at,last_login_at FROM users WHERE email=$1`, email)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	return u, nil
}

func (r *UserRepository) CreateUser(user *auth.User) error {
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	_, err := r.db.Exec(`INSERT INTO users (id,email,name,role,authorized_networks,created_at,updated_at,last_login_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		user.ID, user.Email, user.Name, user.Role, pq.Array(user.AuthorizedNetworks), user.CreatedAt, user.UpdatedAt, nil)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepository) UpdateUser(user *auth.User) error {
	user.UpdatedAt = time.Now()
	_, err := r.db.Exec(`UPDATE users SET email=$2,name=$3,role=$4,authorized_networks=$5,updated_at=$6,last_login_at=$7 WHERE id=$1`,
		user.ID, user.Email, user.Name, user.Role, pq.Array(user.AuthorizedNetworks), user.UpdatedAt, nullTimePtr(user.LastLoginAt))
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// nullTimePtr returns interface{} nil if zero time
func nullTimePtr(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}

func (r *UserRepository) DeleteUser(userID string) error {
	res, err := r.db.Exec(`DELETE FROM users WHERE id=$1`, userID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) ListUsers() ([]*auth.User, error) {
	rows, err := r.db.Query(`SELECT id,email,name,role,authorized_networks,created_at,updated_at,last_login_at FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	out := make([]*auth.User, 0)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *UserRepository) GetFirstUser() (*auth.User, error) {
	row := r.db.QueryRow(`SELECT id,email,name,role,authorized_networks,created_at,updated_at,last_login_at FROM users ORDER BY created_at ASC LIMIT 1`)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no users found")
		}
		return nil, err
	}
	return u, nil
}

func (r *UserRepository) GetDefaultPermissions() (*auth.DefaultNetworkPermissions, error) {
	var role auth.Role
	var networks []string
	err := r.db.QueryRow(`SELECT default_role, default_authorized_networks FROM default_permissions WHERE singleton=TRUE`).Scan(&role, pq.Array(&networks))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("default permissions not set")
		}
		return nil, fmt.Errorf("get default permissions: %w", err)
	}
	return &auth.DefaultNetworkPermissions{DefaultRole: role, DefaultAuthorizedNetworks: networks}, nil
}

func (r *UserRepository) SetDefaultPermissions(perms *auth.DefaultNetworkPermissions) error {
	_, err := r.db.Exec(`INSERT INTO default_permissions (singleton, default_role, default_authorized_networks) VALUES (TRUE,$1,$2)
        ON CONFLICT (singleton) DO UPDATE SET default_role=EXCLUDED.default_role, default_authorized_networks=EXCLUDED.default_authorized_networks`, perms.DefaultRole, pq.Array(perms.DefaultAuthorizedNetworks))
	if err != nil {
		return fmt.Errorf("set default permissions: %w", err)
	}
	return nil
}

// Session management methods

func (r *UserRepository) CreateSession(session *auth.Session) error {
	now := time.Now()
	session.CreatedAt = now
	session.LastUsedAt = now
	_, err := r.db.Exec(`INSERT INTO user_sessions (session_hash, user_id, access_token, refresh_token, access_token_expires_at, refresh_token_expires_at, created_at, last_used_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		session.SessionHash, session.UserID, session.AccessToken, session.RefreshToken, session.AccessTokenExpiresAt, session.RefreshTokenExpiresAt, session.CreatedAt, session.LastUsedAt)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *UserRepository) GetSession(sessionHash string) (*auth.Session, error) {
	var s auth.Session
	err := r.db.QueryRow(`SELECT session_hash, user_id, access_token, refresh_token, access_token_expires_at, refresh_token_expires_at, created_at, last_used_at FROM user_sessions WHERE session_hash=$1`, sessionHash).
		Scan(&s.SessionHash, &s.UserID, &s.AccessToken, &s.RefreshToken, &s.AccessTokenExpiresAt, &s.RefreshTokenExpiresAt, &s.CreatedAt, &s.LastUsedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("get session: %w", err)
	}
	return &s, nil
}

func (r *UserRepository) UpdateSession(session *auth.Session) error {
	session.LastUsedAt = time.Now()
	_, err := r.db.Exec(`UPDATE user_sessions SET access_token=$2, refresh_token=$3, access_token_expires_at=$4, refresh_token_expires_at=$5, last_used_at=$6 WHERE session_hash=$1`,
		session.SessionHash, session.AccessToken, session.RefreshToken, session.AccessTokenExpiresAt, session.RefreshTokenExpiresAt, session.LastUsedAt)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}
	return nil
}

func (r *UserRepository) DeleteSession(sessionHash string) error {
	res, err := r.db.Exec(`DELETE FROM user_sessions WHERE session_hash=$1`, sessionHash)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("session not found")
	}
	return nil
}

func (r *UserRepository) DeleteUserSessions(userID string) error {
	_, err := r.db.Exec(`DELETE FROM user_sessions WHERE user_id=$1`, userID)
	if err != nil {
		return fmt.Errorf("delete user sessions: %w", err)
	}
	return nil
}

func (r *UserRepository) CleanupExpiredSessions() error {
	_, err := r.db.Exec(`DELETE FROM user_sessions WHERE refresh_token_expires_at < NOW()`)
	if err != nil {
		return fmt.Errorf("cleanup expired sessions: %w", err)
	}
	return nil
}
