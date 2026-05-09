package middleware

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"wirety/internal/application/auth"
	"wirety/internal/config"
	domainAuth "wirety/internal/domain/auth"
	"wirety/internal/infrastructure/github"

	"github.com/gin-gonic/gin"
)

// sessionRefreshMu prevents multiple concurrent requests from all trying to
// refresh the same rotating refresh token simultaneously.  Without this, when
// an access token expires every goroutine that sees the stale session calls the
// provider's token endpoint with the same refresh token.  The first call
// succeeds and stores a new rotating refresh token; the others get
// invalid_grant, delete the session, and the user is logged out.
//
// The map is keyed by session hash; values are *sync.Mutex.
var sessionRefreshMu sync.Map

func getSessionRefreshLock(sessionHash string) *sync.Mutex {
	v, _ := sessionRefreshMu.LoadOrStore(sessionHash, &sync.Mutex{})
	return v.(*sync.Mutex)
}

const apiTokenPrefix = "wirety_"

const (
	// UserContextKey is the key used to store user in gin context
	UserContextKey = "user"
)

// AuthMiddleware creates a middleware for authentication
func AuthMiddleware(authService *auth.Service, userRepo domainAuth.Repository, cfg *config.AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// API tokens (wirety_*) are accepted in both auth modes via Authorization: Bearer
		if authHeader := c.GetHeader("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
			raw := strings.TrimPrefix(authHeader, "Bearer ")
			if strings.HasPrefix(raw, apiTokenPrefix) {
				user, err := handleAPITokenAuth(userRepo, raw)
				if err != nil {
					c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
					c.Abort()
					return
				}
				c.Set(UserContextKey, user)
				c.Next()
				return
			}
		}

		// If OIDC auth is disabled, use simple session-based auth
		if !cfg.Enabled {
			// Try cookie first, then Authorization: Session header
			var sessionHash string
			if cookie, err := c.Cookie("wirety_session"); err == nil && cookie != "" {
				sessionHash = cookie
			} else {
				authHeader := c.GetHeader("Authorization")
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.ToLower(parts[0]) == "session" {
					sessionHash = parts[1]
				}
			}
			if sessionHash == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "missing session cookie or authorization header"})
				c.Abort()
				return
			}

			user, err := handleSimpleSessionAuth(userRepo, sessionHash)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				c.Abort()
				return
			}

			c.Set(UserContextKey, user)
			c.Next()
			return
		}

		// Resolve session: cookie takes priority, then Authorization header
		var user *domainAuth.User
		var err error

		if cookie, cookieErr := c.Cookie("wirety_session"); cookieErr == nil && cookie != "" {
			user, err = handleSessionAuth(c, authService, userRepo, cookie)
		} else {
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
				c.Abort()
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
				c.Abort()
				return
			}

			authType := strings.ToLower(parts[0])
			authValue := parts[1]

			switch authType {
			case "session":
				user, err = handleSessionAuth(c, authService, userRepo, authValue)
			case "bearer":
				user, err = handleTokenAuth(c, authService, authValue)
			default:
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization type. Use 'Session' or 'Bearer'"})
				c.Abort()
				return
			}
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

// handleSimpleSessionAuth handles session-based auth for simple (non-OIDC) mode
func handleSimpleSessionAuth(userRepo domainAuth.Repository, sessionHash string) (*domainAuth.User, error) {
	session, err := userRepo.GetSession(sessionHash)
	if err != nil {
		return nil, fmt.Errorf("invalid session")
	}

	if !session.IsValid() {
		_ = userRepo.DeleteSession(sessionHash)
		return nil, fmt.Errorf("session expired")
	}

	user, err := userRepo.GetUser(session.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	session.LastUsedAt = time.Now()
	_ = userRepo.UpdateSession(session)

	return user, nil
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

	// applyRefreshedTokens updates the session fields from a successful refresh response.
	// It prefers the JWT exp claim for AccessTokenExpiresAt because some providers
	// (e.g. Slack) return expires_in for the OAuth access token which can be much
	// longer than the id_token JWT's own lifetime.
	applyRefreshedTokens := func(newAccessToken, newRefreshToken string, expiresIn int) {
		session.AccessToken = newAccessToken
		if exp, ok := authService.ParseTokenExpiry(newAccessToken); ok {
			session.AccessTokenExpiresAt = exp
		} else {
			session.AccessTokenExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
		}
		// Only update refresh token when the provider returned a new one (rotating tokens).
		if newRefreshToken != "" {
			session.RefreshToken = newRefreshToken
		}
	}

	// refreshOnce performs a single token refresh and updates the session in the DB.
	// It captures the new refresh token from the provider (important for rotating-token
	// providers such as Azure Entra ID — discarding it would burn the token and cause
	// the next refresh to fail with invalid_grant).
	refreshOnce := func() error {
		newAccessToken, newRefreshToken, expiresIn, err := authService.RefreshAccessToken(c.Request.Context(), session.RefreshToken)
		if err != nil {
			return err
		}
		applyRefreshedTokens(newAccessToken, newRefreshToken, expiresIn)
		return userRepo.UpdateSession(session)
	}

	// Check if access token needs refresh.
	// Use a per-session mutex so that when multiple concurrent requests all see
	// the same expired access token, only one of them calls the provider's token
	// endpoint.  After acquiring the lock the session is re-read from the DB so
	// that the waiting goroutines transparently pick up the fresh tokens written
	// by the winner and skip the refresh altogether.
	didRefresh := false
	if session.IsAccessTokenExpired() {
		// No refresh token: cannot renew, expire the session immediately.
		// Note: Slack issues refresh tokens (xoxe-1-...) when token rotation is
		// enabled in the Slack app settings. With rotation enabled this branch is
		// never hit for Slack sessions.
		if session.RefreshToken == "" {
			_ = userRepo.DeleteSession(sessionHash)
			return nil, fmt.Errorf("session expired")
		}

		mu := getSessionRefreshLock(sessionHash)
		mu.Lock()
		// Re-read: the goroutine that won the lock may have already refreshed.
		if fresh, readErr := userRepo.GetSession(sessionHash); readErr == nil {
			session = fresh
			// Re-bind refreshOnce to the updated session so it uses the new refresh token.
			refreshOnce = func() error {
				newAccessToken, newRefreshToken, expiresIn, err := authService.RefreshAccessToken(c.Request.Context(), session.RefreshToken)
				if err != nil {
					return err
				}
				applyRefreshedTokens(newAccessToken, newRefreshToken, expiresIn)
				return userRepo.UpdateSession(session)
			}
		}
		if session.IsAccessTokenExpired() {
			if err := refreshOnce(); err != nil {
				mu.Unlock()
				// Do NOT delete the session: the failure may be a lost race
				// (another goroutine already consumed the rotating refresh token
				// and this call got invalid_grant).  The session is still valid;
				// the next request will re-read the updated tokens and succeed.
				return nil, fmt.Errorf("failed to refresh token: %w", err)
			}
			didRefresh = true
		}
		mu.Unlock()
	}

	// GitHub sessions store an opaque access token — skip JWT validation entirely.
	// Session validity (30-day RefreshTokenExpiresAt) is the only expiry mechanism.
	if strings.HasPrefix(session.AccessToken, github.TokenPrefix) {
		user, err := userRepo.GetUser(session.UserID)
		if err != nil {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		session.LastUsedAt = time.Now()
		_ = userRepo.UpdateSession(session)
		return user, nil
	}

	// Validate the access token; capture claims for role-sync on refresh.
	claims, err := authService.ValidateToken(c.Request.Context(), session.AccessToken)
	if err != nil {
		if didRefresh {
			// We just refreshed but the new token is already invalid — something is
			// fundamentally wrong (e.g. clock skew, revoked key). Don't burn another
			// refresh token; just fail.
			_ = userRepo.DeleteSession(sessionHash)
			return nil, fmt.Errorf("token invalid after refresh: %w", err)
		}

		// No refresh token available — expire the session immediately.
		if session.RefreshToken == "" {
			_ = userRepo.DeleteSession(sessionHash)
			return nil, fmt.Errorf("session expired")
		}

		// Token invalid but we haven't refreshed yet — try one refresh under lock.
		// Re-read the session first: a concurrent request may have already refreshed
		// the one-time-use refresh token (e.g. Slack's xoxe-1-... rotating token).
		// Without this re-read, two concurrent requests would both try to burn the same
		// refresh token, the second getting invalid_grant → session deleted.
		mu := getSessionRefreshLock(sessionHash)
		mu.Lock()
		if fresh, readErr := userRepo.GetSession(sessionHash); readErr == nil {
			session = fresh
			// Re-bind applyRefreshedTokens and refreshOnce to the updated session.
			refreshOnce = func() error {
				newAccessToken, newRefreshToken, expiresIn, err := authService.RefreshAccessToken(c.Request.Context(), session.RefreshToken)
				if err != nil {
					return err
				}
				applyRefreshedTokens(newAccessToken, newRefreshToken, expiresIn)
				return userRepo.UpdateSession(session)
			}
		}
		// If another goroutine already refreshed, the token may be valid now — skip.
		var refreshErr error
		if _, validErr := authService.ValidateToken(c.Request.Context(), session.AccessToken); validErr != nil {
			refreshErr = refreshOnce()
		}
		mu.Unlock()
		if refreshErr != nil {
			_ = userRepo.DeleteSession(sessionHash)
			return nil, fmt.Errorf("invalid token and refresh failed: %w", refreshErr)
		}
		didRefresh = true

		// Validate the freshly obtained token.
		claims, err = authService.ValidateToken(c.Request.Context(), session.AccessToken)
		if err != nil {
			_ = userRepo.DeleteSession(sessionHash)
			return nil, fmt.Errorf("token validation failed after refresh: %w", err)
		}
	}

	// Get user from DB.
	user, err := userRepo.GetUser(session.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// When the access token was refreshed this request, re-derive the user's
	// role from the live claims.  This keeps group-based roles in sync at the
	// same cadence as token expiry (typically hourly).
	if didRefresh && claims != nil {
		if synced, syncErr := authService.GetOrCreateUser(c.Request.Context(), claims); syncErr == nil {
			user = synced
		} else if errors.Is(syncErr, auth.ErrNotInAuthorizedGroup) {
			// User was removed from the required group since their last login —
			// revoke the session immediately.
			_ = userRepo.DeleteSession(sessionHash)
			return nil, fmt.Errorf("not authorized")
		}
		// Other sync errors: proceed with the DB user (non-fatal).
	}

	// Update last used timestamp.
	session.LastUsedAt = time.Now()
	_ = userRepo.UpdateSession(session)

	return user, nil
}

// handleAPITokenAuth handles API token authentication (wirety_* tokens)
func handleAPITokenAuth(userRepo domainAuth.Repository, rawToken string) (*domainAuth.User, error) {
	h := sha256.Sum256([]byte(rawToken))
	hash := fmt.Sprintf("%x", h)
	token, err := userRepo.GetAPITokenByHash(hash)
	if err != nil {
		return nil, fmt.Errorf("invalid API token")
	}
	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("API token expired")
	}
	user, err := userRepo.GetUser(token.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	_ = userRepo.TouchAPIToken(token.ID)
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
