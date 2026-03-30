package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type createCaptiveTokenRequest struct {
	PeerIP string `json:"peer_ip" binding:"required"`
}

// CreateCaptivePortalToken creates a captive portal token for a connecting peer.
// Called by the jump peer agent (authenticated via enrollment token) when a peer
// connects to the WireGuard tunnel and needs to authenticate.
func (h *Handler) CreateCaptivePortalToken(c *gin.Context) {
	token := extractBearerToken(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}

	networkID, peer, err := h.service.ResolveAgentToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	if !peer.IsJump {
		c.JSON(http.StatusForbidden, gin.H{"error": "only jump peers can create captive portal tokens"})
		return
	}

	var req createCaptiveTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cpt, err := h.service.CreateCaptivePortalToken(c.Request.Context(), networkID, peer.ID, req.PeerIP)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create token"})
		return
	}

	c.JSON(http.StatusCreated, cpt)
}

type authenticateCaptivePortalRequest struct {
	CaptiveToken string `json:"captive_token" binding:"required"`
	SessionHash  string `json:"session_hash" binding:"required"`
}

// AuthenticateCaptivePortal validates the captive portal token and session, then whitelists the peer.
// Called by the frontend captive portal page after the user authenticates via OIDC or password.
func (h *Handler) AuthenticateCaptivePortal(c *gin.Context) {
	var req authenticateCaptivePortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cpt, err := h.service.AuthenticateCaptivePortal(c.Request.Context(), req.CaptiveToken, req.SessionHash)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"peer_ip":     cpt.PeerIP,
		"network_id":  cpt.NetworkID,
		"whitelisted": true,
	})
}
