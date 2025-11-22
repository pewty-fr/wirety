package memory

import (
	"fmt"
	"sync"

	"wirety/internal/domain/auth"
)

// UserRepository is an in-memory implementation of the auth repository
type UserRepository struct {
	mu           sync.RWMutex
	users        map[string]*auth.User // userID -> User
	usersByEmail map[string]*auth.User // email -> User
	sessions     map[string]*auth.Session // sessionHash -> Session
	defaultPerms *auth.DefaultNetworkPermissions
}

// NewUserRepository creates a new in-memory user repository
func NewUserRepository() *UserRepository {
	return &UserRepository{
		users:        make(map[string]*auth.User),
		usersByEmail: make(map[string]*auth.User),
		defaultPerms: &auth.DefaultNetworkPermissions{
			DefaultRole:               auth.RoleUser,
			DefaultAuthorizedNetworks: []string{},
		},
	}
}

// GetUser retrieves a user by their OIDC subject ID
func (r *UserRepository) GetUser(userID string) (*auth.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

// GetUserByEmail retrieves a user by their email
func (r *UserRepository) GetUserByEmail(email string) (*auth.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.usersByEmail[email]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

// CreateUser creates a new user
func (r *UserRepository) CreateUser(user *auth.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.users[user.ID]; exists {
		return fmt.Errorf("user already exists")
	}

	r.users[user.ID] = user
	r.usersByEmail[user.Email] = user
	return nil
}

// UpdateUser updates an existing user
func (r *UserRepository) UpdateUser(user *auth.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.users[user.ID]; !exists {
		return fmt.Errorf("user not found")
	}

	// Update email index if email changed
	if oldUser := r.users[user.ID]; oldUser.Email != user.Email {
		delete(r.usersByEmail, oldUser.Email)
		r.usersByEmail[user.Email] = user
	}

	r.users[user.ID] = user
	return nil
}

// DeleteUser deletes a user
func (r *UserRepository) DeleteUser(userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, exists := r.users[userID]
	if !exists {
		return fmt.Errorf("user not found")
	}

	delete(r.users, userID)
	delete(r.usersByEmail, user.Email)
	return nil
}

// ListUsers retrieves all users
func (r *UserRepository) ListUsers() ([]*auth.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make([]*auth.User, 0, len(r.users))
	for _, user := range r.users {
		users = append(users, user)
	}
	return users, nil
}

// GetFirstUser returns the first user (for initial admin setup)
func (r *UserRepository) GetFirstUser() (*auth.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.users) == 0 {
		return nil, fmt.Errorf("no users found")
	}

	// Return any user (map iteration is random but that's okay)
	for _, user := range r.users {
		return user, nil
	}
	return nil, fmt.Errorf("no users found")
}

// GetDefaultPermissions retrieves default permissions for new users
func (r *UserRepository) GetDefaultPermissions() (*auth.DefaultNetworkPermissions, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.defaultPerms == nil {
		return nil, fmt.Errorf("default permissions not set")
	}
	return r.defaultPerms, nil
}

// SetDefaultPermissions sets default permissions for new users
func (r *UserRepository) SetDefaultPermissions(perms *auth.DefaultNetworkPermissions) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.defaultPerms = perms
	return nil
}

// Session management methods

// CreateSession creates a new session
func (r *UserRepository) CreateSession(session *auth.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sessions == nil {
		r.sessions = make(map[string]*auth.Session)
	}

	if _, exists := r.sessions[session.SessionHash]; exists {
		return fmt.Errorf("session already exists")
	}

	r.sessions[session.SessionHash] = session
	return nil
}

// GetSession retrieves a session by hash
func (r *UserRepository) GetSession(sessionHash string) (*auth.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.sessions == nil {
		return nil, fmt.Errorf("session not found")
	}

	session, exists := r.sessions[sessionHash]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}
	return session, nil
}

// UpdateSession updates an existing session
func (r *UserRepository) UpdateSession(session *auth.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sessions == nil {
		return fmt.Errorf("session not found")
	}

	if _, exists := r.sessions[session.SessionHash]; !exists {
		return fmt.Errorf("session not found")
	}

	r.sessions[session.SessionHash] = session
	return nil
}

// DeleteSession deletes a session
func (r *UserRepository) DeleteSession(sessionHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sessions == nil {
		return fmt.Errorf("session not found")
	}

	if _, exists := r.sessions[sessionHash]; !exists {
		return fmt.Errorf("session not found")
	}

	delete(r.sessions, sessionHash)
	return nil
}

// DeleteUserSessions deletes all sessions for a user
func (r *UserRepository) DeleteUserSessions(userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sessions == nil {
		return nil
	}

	for hash, session := range r.sessions {
		if session.UserID == userID {
			delete(r.sessions, hash)
		}
	}
	return nil
}

// CleanupExpiredSessions removes sessions with expired refresh tokens
func (r *UserRepository) CleanupExpiredSessions() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sessions == nil {
		return nil
	}

	for hash, session := range r.sessions {
		if session.IsRefreshTokenExpired() {
			delete(r.sessions, hash)
		}
	}
	return nil
}
