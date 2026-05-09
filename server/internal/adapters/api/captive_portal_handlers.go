package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type createCaptiveTokenRequest struct {
	PeerIP       string `json:"peer_ip" binding:"required"`
	PeerEndpoint string `json:"peer_endpoint"` // full "ip:port" at connect time (optional)
}

// CreateCaptivePortalToken creates a captive portal token for a connecting peer.
// Called by the jump peer agent (authenticated via enrollment token) when a peer
// connects to the WireGuard tunnel and needs to authenticate.
func (h *Handler) CreateCaptivePortalToken(c *gin.Context) {
	// Captive portal requires OIDC so that each user has a distinct identity.
	// Simple auth (AUTH_ENABLED=false) uses a shared admin password — there is no
	// per-user identity to enforce peer ownership, so captive portal is disabled.
	if !h.authConfig.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "captive portal is not available when AUTH_ENABLED=false"})
		return
	}

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

	cpt, err := h.service.CreateCaptivePortalToken(c.Request.Context(), networkID, peer.ID, req.PeerIP, req.PeerEndpoint)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create token"})
		return
	}

	c.JSON(http.StatusCreated, cpt)
}

type authenticateCaptivePortalRequest struct {
	CaptiveToken string `json:"captive_token" binding:"required"`
}

// AuthenticateCaptivePortal validates the captive portal token and session, then whitelists the peer.
// Called by the frontend captive portal page after the user authenticates via OIDC or password.
// The session is read from the wirety_session cookie — no need to send session_hash in the body.
func (h *Handler) AuthenticateCaptivePortal(c *gin.Context) {
	if !h.authConfig.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "captive portal is not available when AUTH_ENABLED=false"})
		return
	}

	var req authenticateCaptivePortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Resolve session hash from cookie (same cookie used by every authenticated page)
	sessionHash, err := c.Cookie(sessionCookieName)
	if err != nil || sessionHash == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session found, please log in first"})
		return
	}

	cpt, err := h.service.AuthenticateCaptivePortal(c.Request.Context(), req.CaptiveToken, sessionHash)
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
