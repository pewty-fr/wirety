package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetCaptivePortalToken godoc
// @Summary      Get captive portal authentication token
// @Description  Generate a temporary token for captive portal authentication (agent only)
// @Tags         agent
// @Produce      json
// @Param        token query string true "Agent enrollment token"
// @Success      200 {object} map[string]string
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /agent/captive-portal-token [get]
func (h *Handler) GetCaptivePortalToken(c *gin.Context) {
	agentToken := c.Query("token")
	if agentToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token query parameter is required"})
		return
	}

	// Resolve agent token to get network and peer info
	networkID, peer, err := h.service.ResolveAgentToken(c.Request.Context(), agentToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid agent token"})
		return
	}

	// Verify this is a jump peer
	if !peer.IsJump {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only jump peers can request captive portal tokens"})
		return
	}

	// Generate captive portal token
	captiveToken, err := h.service.GenerateCaptivePortalToken(c.Request.Context(), networkID, peer.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"captive_portal_token": captiveToken,
		"expires_in":           300, // 5 minutes
	})
}
