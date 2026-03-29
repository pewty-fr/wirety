package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"

	"wirety/internal/adapters/api/middleware"
	"wirety/internal/domain/auth"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const apiTokenPrefix = "wirety_"

// CreateAPIToken godoc
//
//	@Summary		Create an API token
//	@Description	Generate a new long-lived API token for the authenticated user
//	@Tags			tokens
//	@Accept			json
//	@Produce		json
//	@Param			request	body		auth.APITokenCreateRequest	true	"Token creation request"
//	@Success		201		{object}	auth.APITokenResponse
//	@Failure		400		{object}	map[string]string
//	@Failure		401		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Router			/users/me/tokens [post]
//	@Security		BearerAuth
func (h *Handler) CreateAPIToken(c *gin.Context) {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found in context"})
		return
	}

	var req auth.APITokenCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate 32 random bytes → 64 hex chars
	raw, err := generateRawToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	h256 := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(h256[:])

	token := &auth.APIToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Name:      req.Name,
		TokenHash: hash,
		ExpiresAt: req.ExpiresAt,
	}

	if err := h.userRepo.CreateAPIToken(token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create token"})
		return
	}

	c.JSON(http.StatusCreated, auth.APITokenResponse{
		ID:         token.ID,
		Name:       token.Name,
		RawToken:   raw, // shown exactly once
		CreatedAt:  token.CreatedAt,
		ExpiresAt:  token.ExpiresAt,
		LastUsedAt: token.LastUsedAt,
	})
}

// ListAPITokens godoc
//
//	@Summary		List API tokens
//	@Description	List all API tokens for the authenticated user
//	@Tags			tokens
//	@Produce		json
//	@Success		200	{array}		auth.APITokenResponse
//	@Failure		401	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Router			/users/me/tokens [get]
//	@Security		BearerAuth
func (h *Handler) ListAPITokens(c *gin.Context) {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found in context"})
		return
	}

	tokens, err := h.userRepo.ListAPITokens(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list tokens"})
		return
	}

	resp := make([]auth.APITokenResponse, 0, len(tokens))
	for _, t := range tokens {
		resp = append(resp, auth.APITokenResponse{
			ID:         t.ID,
			Name:       t.Name,
			CreatedAt:  t.CreatedAt,
			ExpiresAt:  t.ExpiresAt,
			LastUsedAt: t.LastUsedAt,
		})
	}

	c.JSON(http.StatusOK, resp)
}

// DeleteAPIToken godoc
//
//	@Summary		Delete an API token
//	@Description	Revoke an API token by ID
//	@Tags			tokens
//	@Produce		json
//	@Param			tokenId	path		string	true	"Token ID"
//	@Success		204
//	@Failure		401		{object}	map[string]string
//	@Failure		403		{object}	map[string]string
//	@Failure		404		{object}	map[string]string
//	@Router			/users/me/tokens/{tokenId} [delete]
//	@Security		BearerAuth
func (h *Handler) DeleteAPIToken(c *gin.Context) {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found in context"})
		return
	}

	tokenID := c.Param("tokenId")

	// Ensure token belongs to the user by listing their tokens
	tokens, err := h.userRepo.ListAPITokens(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list tokens"})
		return
	}

	owned := false
	for _, t := range tokens {
		if t.ID == tokenID {
			owned = true
			break
		}
	}
	if !owned {
		c.JSON(http.StatusForbidden, gin.H{"error": "token not found or not owned by user"})
		return
	}

	if err := h.userRepo.DeleteAPIToken(tokenID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

func generateRawToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return apiTokenPrefix + hex.EncodeToString(b), nil
}

// createAPITokenForUser hashes rawToken, persists it and returns the response
// (including the raw token, shown once). Shared between the REST handler and the
// embedded MCP server.
func createAPITokenForUser(repo auth.Repository, userID, name, rawToken string) (auth.APITokenResponse, error) {
	h256 := sha256.Sum256([]byte(rawToken))
	hash := hex.EncodeToString(h256[:])

	token := &auth.APIToken{
		ID:        uuid.New().String(),
		UserID:    userID,
		Name:      name,
		TokenHash: hash,
	}
	if err := repo.CreateAPIToken(token); err != nil {
		return auth.APITokenResponse{}, fmt.Errorf("create token: %w", err)
	}
	return auth.APITokenResponse{
		ID:         token.ID,
		Name:       token.Name,
		RawToken:   rawToken,
		CreatedAt:  token.CreatedAt,
		ExpiresAt:  token.ExpiresAt,
		LastUsedAt: token.LastUsedAt,
	}, nil
}
