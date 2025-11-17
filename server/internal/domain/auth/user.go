package auth

import "time"

// Role represents user roles in the system
type Role string

const (
	RoleAdministrator Role = "administrator"
	RoleUser          Role = "user"
)

// User represents a user in the system
type User struct {
	ID                 string    `json:"id"`                  // OIDC subject ID
	Email              string    `json:"email"`               // User email from OIDC
	Name               string    `json:"name"`                // Display name from OIDC
	Role               Role      `json:"role"`                // User role (administrator or user)
	AuthorizedNetworks []string  `json:"authorized_networks"` // Network IDs the user can access
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	LastLoginAt        time.Time `json:"last_login_at"`
}

// UserCreateRequest represents a request to create a new user
type UserCreateRequest struct {
	Email              string   `json:"email" binding:"required"`
	Name               string   `json:"name" binding:"required"`
	Role               Role     `json:"role" binding:"required"`
	AuthorizedNetworks []string `json:"authorized_networks"`
}

// UserUpdateRequest represents a request to update user settings
type UserUpdateRequest struct {
	Name               string   `json:"name,omitempty"`
	Role               Role     `json:"role,omitempty"`
	AuthorizedNetworks []string `json:"authorized_networks,omitempty"`
}

// IsAdministrator checks if the user has administrator role
func (u *User) IsAdministrator() bool {
	return u.Role == RoleAdministrator
}

// HasNetworkAccess checks if the user has access to a specific network
func (u *User) HasNetworkAccess(networkID string) bool {
	if u.IsAdministrator() {
		return true
	}
	for _, id := range u.AuthorizedNetworks {
		if id == networkID {
			return true
		}
	}
	return false
}

// CanManagePeer checks if the user can manage a peer in a network
// Users can only manage their own peers in networks they have access to
func (u *User) CanManagePeer(networkID string, peerOwnerID string) bool {
	if u.IsAdministrator() {
		return true
	}
	// Regular users can only manage their own peers in authorized networks
	return u.HasNetworkAccess(networkID) && peerOwnerID == u.ID
}

// DefaultNetworkPermissions represents default settings for new users
type DefaultNetworkPermissions struct {
	DefaultRole               Role     `json:"default_role"`
	DefaultAuthorizedNetworks []string `json:"default_authorized_networks"`
}
