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
