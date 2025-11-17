package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"wirety/internal/config"

	"github.com/gin-gonic/gin"
)

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

// TokenResponse contains the access token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// ExchangeToken godoc
// @Summary      Exchange authorization code for access token
// @Description  Exchange OIDC authorization code for access token
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

	// Build token endpoint URL
	tokenURL := fmt.Sprintf("%s/protocol/openid-connect/token", cfg.Auth.IssuerURL)

	// Prepare token exchange request
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", req.Code)
	data.Set("redirect_uri", req.RedirectURI)
	data.Set("client_id", cfg.Auth.ClientID)
	data.Set("client_secret", cfg.Auth.ClientSecret)

	// Make request to token endpoint
	resp, err := http.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to exchange token: %v", err)})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read response: %v", err)})
		return
	}

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Token exchange failed: %s", string(body))})
		return
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse token response: %v", err)})
		return
	}

	c.JSON(http.StatusOK, tokenResp)
}
