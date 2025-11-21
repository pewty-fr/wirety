package api

import (
	"net/http"

	domain "wirety/internal/domain/network"

	"github.com/gin-gonic/gin"
)

// GetPeerSessionStatus godoc
// @Summary      Get peer session status
// @Description  Get the security status of a peer's active sessions
// @Tags         peers
// @Produce      json
// @Param        networkId path string true "Network ID"
// @Param        peerId    path string true "Peer ID"
// @Success      200 {object} domain.PeerSessionStatus
// @Failure      404 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /networks/{networkId}/peers/{peerId}/session [get]
// @Security     BearerAuth
func (h *Handler) GetPeerSessionStatus(c *gin.Context) {
	networkID := c.Param("networkId")
	peerID := c.Param("peerId")

	var status *domain.PeerSessionStatus
	status, err := h.service.GetPeerSessionStatus(c.Request.Context(), networkID, peerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// ListNetworkSessions godoc
// @Summary      List network sessions
// @Description  Get all active agent sessions in a network (admin only)
// @Tags         networks
// @Produce      json
// @Param        networkId path string true "Network ID"
// @Success      200 {array} domain.AgentSession
// @Failure      403 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /networks/{networkId}/sessions [get]
// @Security     BearerAuth
func (h *Handler) ListNetworkSessions(c *gin.Context) {
	networkID := c.Param("networkId")

	var sessions []*domain.AgentSession
	sessions, err := h.service.ListSessions(c.Request.Context(), networkID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, sessions)
}
