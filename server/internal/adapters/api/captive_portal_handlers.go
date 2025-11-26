package api

import (
	"net/http"

	"wirety/internal/config"

	"github.com/gin-gonic/gin"
)

// AuthenticateCaptivePortalRequest contains the authentication request from captive portal
type AuthenticateCaptivePortalRequest struct {
	CaptiveToken string `json:"captive_token" binding:"required"` // Temporary captive portal token
	SessionHash  string `json:"session_hash" binding:"required"`  // User session hash
	PeerIP       string `json:"peer_ip" binding:"required"`
}

// AuthenticateCaptivePortal godoc
// @Summary      Authenticate non-agent peer for internet access
// @Description  Validates agent token and user access token, then whitelists the peer IP
// @Tags         captive-portal
// @Accept       json
// @Produce      json
// @Param        request body AuthenticateCaptivePortalRequest true "Authentication request"
// @Success      200 {object} map[string]string
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /captive-portal/authenticate [post]
func (h *Handler) AuthenticateCaptivePortal(c *gin.Context) {
	cfg := config.LoadConfig()

	var req AuthenticateCaptivePortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate captive portal token
	networkID, jumpPeerID, err := h.service.ValidateCaptivePortalToken(c.Request.Context(), req.CaptiveToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired captive portal token"})
		return
	}

	// Delete the token after use (one-time use for security)
	_ = h.service.DeleteCaptivePortalToken(c.Request.Context(), req.CaptiveToken)

	// Validate user session
	if cfg.Auth.Enabled {
		// Get session from session hash
		session, err := h.userRepo.GetSession(req.SessionHash)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired session"})
			return
		}

		// Get user from session
		user, err := h.userRepo.GetUser(session.UserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
			return
		}

		// Check if user has access to this network
		if !user.HasNetworkAccess(networkID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "User does not have access to this network"})
			return
		}
	}

	// Add peer IP to whitelist for this network
	if err := h.service.AddCaptivePortalWhitelist(c.Request.Context(), networkID, jumpPeerID, req.PeerIP); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to whitelist peer"})
		return
	}

	// Notify jump peer via WebSocket to update firewall rules
	// This is handled automatically by the WebSocket manager when whitelist is updated

	c.JSON(http.StatusOK, gin.H{
		"message":    "Authentication successful",
		"network_id": networkID,
		"peer_ip":    req.PeerIP,
	})
}
