package auth

// Repository defines the interface for user persistence
type Repository interface {
	// GetUser retrieves a user by their OIDC subject ID
	GetUser(userID string) (*User, error)

	// GetUserByEmail retrieves a user by their email
	GetUserByEmail(email string) (*User, error)

	// CreateUser creates a new user
	CreateUser(user *User) error

	// UpdateUser updates an existing user
	UpdateUser(user *User) error

	// DeleteUser deletes a user
	DeleteUser(userID string) error

	// ListUsers retrieves all users
	ListUsers() ([]*User, error)

	// GetFirstUser returns the first user (for initial admin setup)
	GetFirstUser() (*User, error)

	// GetDefaultPermissions retrieves default permissions for new users
	GetDefaultPermissions() (*DefaultNetworkPermissions, error)

	// SetDefaultPermissions sets default permissions for new users
	SetDefaultPermissions(perms *DefaultNetworkPermissions) error
}
