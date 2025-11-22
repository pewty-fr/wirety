package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"wirety/internal/application/auth"
	"wirety/internal/config"
	domainAuth "wirety/internal/domain/auth"

	"github.com/gin-gonic/gin"
)

const (
	// UserContextKey is the key used to store user in gin context
	UserContextKey = "user"
)

// AuthMiddleware creates a middleware for authentication
func AuthMiddleware(authService *auth.Service, userRepo domainAuth.Repository, cfg *config.AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// If auth is disabled, set a default admin user and continue
		if !cfg.Enabled {
			// Create a virtual admin user for no-auth mode
			adminUser := &domainAuth.User{
				ID:    "admin",
				Email: "admin@wirety.local",
				Name:  "Administrator",
				Role:  domainAuth.RoleAdministrator,
			}
			c.Set(UserContextKey, adminUser)
			c.Next()
			return
		}

		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		// Extract Bearer token or Session hash
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		authType := strings.ToLower(parts[0])
		authValue := parts[1]

		var user *domainAuth.User
		var err error

		switch authType {
		case "session":
			// Session-based authentication (preferred)
			user, err = handleSessionAuth(c, authService, userRepo, authValue)
		case "bearer":
			// Legacy token-based authentication (for backward compatibility)
			user, err = handleTokenAuth(c, authService, authValue)
		default:
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization type. Use 'Session' or 'Bearer'"})
			c.Abort()
			return
		}

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		// Store user in context
		c.Set(UserContextKey, user)
		c.Next()
	}
}

// handleSessionAuth handles session-based authentication
func handleSessionAuth(c *gin.Context, authService *auth.Service, userRepo domainAuth.Repository, sessionHash string) (*domainAuth.User, error) {
	// Get session from database
	session, err := userRepo.GetSession(sessionHash)
	if err != nil {
		return nil, fmt.Errorf("invalid session")
	}

	// Check if session is valid
	if !session.IsValid() {
		_ = userRepo.DeleteSession(sessionHash)
		return nil, fmt.Errorf("session expired")
	}

	// Check if access token needs refresh
	if session.IsAccessTokenExpired() {
		// Refresh the access token
		newAccessToken, expiresIn, err := authService.RefreshAccessToken(c.Request.Context(), session.RefreshToken)
		if err != nil {
			// If refresh fails, delete the session
			_ = userRepo.DeleteSession(sessionHash)
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}

		// Update session with new access token
		session.AccessToken = newAccessToken
		session.AccessTokenExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
		if err := userRepo.UpdateSession(session); err != nil {
			return nil, fmt.Errorf("failed to update session: %w", err)
		}
	}

	// Validate the access token
	_, err = authService.ValidateToken(c.Request.Context(), session.AccessToken)
	if err != nil {
		// If validation fails, try to refresh
		newAccessToken, expiresIn, refreshErr := authService.RefreshAccessToken(c.Request.Context(), session.RefreshToken)
		if refreshErr != nil {
			_ = userRepo.DeleteSession(sessionHash)
			return nil, fmt.Errorf("invalid token and refresh failed")
		}

		// Update session with new access token
		session.AccessToken = newAccessToken
		session.AccessTokenExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
		if err := userRepo.UpdateSession(session); err != nil {
			return nil, fmt.Errorf("failed to update session: %w", err)
		}

		// Validate the new token
		_, err = authService.ValidateToken(c.Request.Context(), session.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("token validation failed after refresh: %w", err)
		}
	}

	// Get user
	user, err := userRepo.GetUser(session.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Update last used timestamp
	session.LastUsedAt = time.Now()
	_ = userRepo.UpdateSession(session)

	return user, nil
}

// handleTokenAuth handles legacy token-based authentication
func handleTokenAuth(c *gin.Context, authService *auth.Service, tokenString string) (*domainAuth.User, error) {
	// Validate token
	claims, err := authService.ValidateToken(c.Request.Context(), tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Get or create user
	user, err := authService.GetOrCreateUser(c.Request.Context(), claims)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// RequireAdmin is a middleware that requires administrator role
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetUserFromContext(c)
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found in context"})
			c.Abort()
			return
		}

		if !user.IsAdministrator() {
			c.JSON(http.StatusForbidden, gin.H{"error": "administrator role required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireNetworkAccess is a middleware that requires access to a specific network
func RequireNetworkAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetUserFromContext(c)
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found in context"})
			c.Abort()
			return
		}

		networkID := c.Param("networkId")
		if networkID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "network ID required"})
			c.Abort()
			return
		}

		if !user.HasNetworkAccess(networkID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access to this network is not authorized"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUserFromContext retrieves the user from the gin context
func GetUserFromContext(c *gin.Context) *domainAuth.User {
	if user, exists := c.Get(UserContextKey); exists {
		if u, ok := user.(*domainAuth.User); ok {
			return u
		}
	}
	return nil
}
