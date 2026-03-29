package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ResolveAgent godoc
// @Summary      Resolve agent enrollment token
// @Description  Exchange a one-time (or long-lived) peer enrollment token for identifiers and initial config
// @Tags         agent
// @Produce      json
// @Param        token  query string true "Enrollment token"
// @Success      200 {object} map[string]any
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /agent/resolve [get]
// @Security     BearerAuth
func (h *Handler) ResolveAgent(c *gin.Context) {
	token := extractBearerToken(c)
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization: Bearer <token> header required"})
		return
	}
	networkID, peer, err := h.service.ResolveAgentToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	cfg, err := h.service.GeneratePeerConfig(c.Request.Context(), networkID, peer.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"network_id": networkID,
		"peer_id":    peer.ID,
		"peer_name":  peer.Name,
		"config":     cfg,
	})
}
