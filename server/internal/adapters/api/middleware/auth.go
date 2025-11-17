package middleware

import (
	"net/http"
	"strings"

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
func AuthMiddleware(authService *auth.Service, cfg *config.AuthConfig) gin.HandlerFunc {
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

		// Extract Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Validate token
		claims, err := authService.ValidateToken(c.Request.Context(), tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token: " + err.Error()})
			c.Abort()
			return
		}

		// Get or create user
		user, err := authService.GetOrCreateUser(c.Request.Context(), claims)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user: " + err.Error()})
			c.Abort()
			return
		}

		// Store user in context
		c.Set(UserContextKey, user)
		c.Next()
	}
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
