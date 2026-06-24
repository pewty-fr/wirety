package api

import (
	"net/http"

	"wirety/internal/adapters/api/middleware"
	domain "wirety/internal/domain/network"

	"github.com/gin-gonic/gin"
)

// GetPeerConnectivityStatus godoc
// @Summary      Get peer connectivity status
// @Description  Get the live connectivity status of a peer (whether it has an active agent and the last heartbeat).
// @Tags         peers
// @Produce      json
// @Param        networkId path string true "Network ID"
// @Param        peerId    path string true "Peer ID"
// @Success      200 {object} domain.PeerConnectivityStatus
// @Failure      404 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /networks/{networkId}/peers/{peerId}/session [get]
// @Security     BearerAuth
func (h *Handler) GetPeerConnectivityStatus(c *gin.Context) {
	networkID := c.Param("networkId")
	peerID := c.Param("peerId")
	user := middleware.GetUserFromContext(c)

	// Object-level authz: a non-admin may only read connectivity for their own
	// peers (jump peers are shared infrastructure and stay visible, mirroring
	// GetPeer — the dashboard polls every listed peer's status, jump included).
	peer, err := h.service.GetPeer(c.Request.Context(), networkID, peerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "peer not found"})
		return
	}
	if user != nil && !user.IsAdministrator() && !peer.IsJump && peer.OwnerID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "you can only view your own peers"})
		return
	}

	status, err := h.service.GetPeerConnectivityStatus(c.Request.Context(), networkID, peerID)
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
