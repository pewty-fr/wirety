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

	// Session management
	// CreateSession creates a new session
	CreateSession(session *Session) error

	// GetSession retrieves a session by hash
	GetSession(sessionHash string) (*Session, error)

	// UpdateSession updates an existing session
	UpdateSession(session *Session) error

	// DeleteSession deletes a session
	DeleteSession(sessionHash string) error

	// DeleteUserSessions deletes all sessions for a user
	DeleteUserSessions(userID string) error

	// CleanupExpiredSessions removes sessions with expired refresh tokens
	CleanupExpiredSessions() error

	// API token management
	// CreateAPIToken persists a new API token (TokenHash must already be set).
	CreateAPIToken(token *APIToken) error

	// GetAPITokenByHash looks up a token by its SHA-256 hash.
	GetAPITokenByHash(hash string) (*APIToken, error)

	// ListAPITokens returns all tokens owned by the given user.
	ListAPITokens(userID string) ([]*APIToken, error)

	// DeleteAPIToken revokes a token by its ID.
	DeleteAPIToken(tokenID string) error

	// TouchAPIToken records that a token was just used (updates last_used_at).
	TouchAPIToken(tokenID string) error
}
