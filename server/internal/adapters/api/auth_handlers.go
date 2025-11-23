package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"wirety/internal/config"
	"wirety/internal/domain/auth"
	"wirety/internal/infrastructure/oidc"

	"github.com/gin-gonic/gin"
)

// Uses shared OIDC discovery adapter (internal/infrastructure/oidc)

// AuthConfigResponse contains public authentication configuration
type AuthConfigResponse struct {
	Enabled   bool   `json:"enabled"`
	IssuerURL string `json:"issuer_url"`
	ClientID  string `json:"client_id"`
}

// GetAuthConfig godoc
// @Summary      Get authentication configuration
// @Description  Get public authentication configuration (no auth required)
// @Tags         auth
// @Produce      json
// @Success      200 {object} AuthConfigResponse
// @Router       /auth/config [get]
func (h *Handler) GetAuthConfig(c *gin.Context) {
	cfg := config.LoadConfig()

	c.JSON(http.StatusOK, AuthConfigResponse{
		Enabled:   cfg.Auth.Enabled,
		IssuerURL: cfg.Auth.IssuerURL,
		ClientID:  cfg.Auth.ClientID,
	})
}

// TokenRequest contains the authorization code exchange request
type TokenRequest struct {
	Code        string `json:"code" binding:"required"`
	RedirectURI string `json:"redirect_uri" binding:"required"`
}

// TokenResponse contains the session hash response
type TokenResponse struct {
	SessionHash string `json:"session_hash"`
	ExpiresIn   int    `json:"expires_in"`
}

// ExchangeToken godoc
// @Summary      Exchange authorization code for session
// @Description  Exchange OIDC authorization code for access token and create server-side session
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body TokenRequest true "Token exchange request"
// @Success      200 {object} TokenResponse
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /auth/token [post]
func (h *Handler) ExchangeToken(c *gin.Context) {
	cfg := config.LoadConfig()

	if !cfg.Auth.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authentication is not enabled"})
		return
	}

	var req TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Discover OIDC endpoints via shared adapter
	discovery, err := oidc.Discover(c.Request.Context(), cfg.Auth.IssuerURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to discover OIDC endpoints: %v", err)})
		return
	}

	// Prepare token exchange request with offline_access scope to get refresh token
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", req.Code)
	data.Set("redirect_uri", req.RedirectURI)
	data.Set("client_id", cfg.Auth.ClientID)
	data.Set("client_secret", cfg.Auth.ClientSecret)

	// Make request to token endpoint from discovery
	resp, err := http.DefaultClient.Post(discovery.TokenEndpoint, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to exchange token: %v", err)})
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read response: %v", err)})
		return
	}

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Token exchange failed: %s", string(body))})
		return
	}

	var oidcTokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &oidcTokenResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse token response: %v", err)})
		return
	}

	if oidcTokenResp.RefreshToken == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No refresh token received. Ensure offline_access scope is requested."})
		return
	}

	// Validate the access token and get user claims
	claims, err := h.authService.ValidateToken(c.Request.Context(), oidcTokenResp.AccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to validate token: %v", err)})
		return
	}

	// Get or create user
	user, err := h.authService.GetOrCreateUser(c.Request.Context(), claims)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get user: %v", err)})
		return
	}

	// Create session with tokens
	session, err := h.createSession(user.ID, oidcTokenResp.AccessToken, oidcTokenResp.RefreshToken, oidcTokenResp.ExpiresIn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create session: %v", err)})
		return
	}

	c.JSON(http.StatusOK, TokenResponse{
		SessionHash: session.SessionHash,
		ExpiresIn:   oidcTokenResp.ExpiresIn,
	})
}

// createSession creates a new session with a secure hash
func (h *Handler) createSession(userID, accessToken, refreshToken string, expiresIn int) (*auth.Session, error) {
	// Generate a secure random session hash
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate session hash: %w", err)
	}
	hash := sha256.Sum256(randomBytes)
	sessionHash := hex.EncodeToString(hash[:])

	// Calculate expiration times
	accessTokenExpiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
	// Refresh tokens typically last much longer (e.g., 30 days)
	refreshTokenExpiresAt := time.Now().Add(30 * 24 * time.Hour)

	session := &auth.Session{
		SessionHash:           sessionHash,
		UserID:                userID,
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		AccessTokenExpiresAt:  accessTokenExpiresAt,
		RefreshTokenExpiresAt: refreshTokenExpiresAt,
	}

	if err := h.userRepo.CreateSession(session); err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	return session, nil
}

// LogoutRequest contains the session hash to logout
type LogoutRequest struct {
	SessionHash string `json:"session_hash" binding:"required"`
}

// Logout godoc
// @Summary      Logout and invalidate session
// @Description  Logout user and delete server-side session
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body LogoutRequest true "Logout request"
// @Success      200 {object} map[string]string
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.userRepo.DeleteSession(req.SessionHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// LogoutRequest contains the session hash to logout
type LogoutRequest struct {
	SessionHash string `json:"session_hash" binding:"required"`
}

// Logout godoc
// @Summary      Logout and invalidate session
// @Description  Logout user and delete server-side session
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body LogoutRequest true "Logout request"
// @Success      200 {object} map[string]string
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.userRepo.DeleteSession(req.SessionHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}
