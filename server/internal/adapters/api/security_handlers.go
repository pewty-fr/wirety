package api

import (
	"net/http"

	"wirety/internal/adapters/api/middleware"

	"github.com/gin-gonic/gin"
)

// ListSecurityIncidents godoc
// @Summary List all security incidents
// @Description Get a list of all security incidents, optionally filtered by resolved status
// @Tags security
// @Accept json
// @Produce json
// @Param resolved query bool false "Filter by resolved status"
// @Success 200 {array} network.SecurityIncident
// @Failure 500 {object} map[string]string
// @Router /security/incidents [get]
// @Security     BearerAuth
func (h *Handler) ListSecurityIncidents(c *gin.Context) {
	var resolved *bool
	if c.Query("resolved") != "" {
		val := c.Query("resolved") == "true"
		resolved = &val
	}

	incidents, err := h.service.ListSecurityIncidents(c.Request.Context(), resolved)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, incidents)
}

// ListNetworkSecurityIncidents godoc
// @Summary List security incidents for a network
// @Description Get a list of security incidents for a specific network
// @Tags security
// @Accept json
// @Produce json
// @Param networkId path string true "Network ID"
// @Param resolved query bool false "Filter by resolved status"
// @Success 200 {array} network.SecurityIncident
// @Failure 500 {object} map[string]string
// @Router /networks/{networkId}/security/incidents [get]
// @Security     BearerAuth
func (h *Handler) ListNetworkSecurityIncidents(c *gin.Context) {
	networkID := c.Param("networkId")

	var resolved *bool
	if c.Query("resolved") != "" {
		val := c.Query("resolved") == "true"
		resolved = &val
	}

	incidents, err := h.service.ListSecurityIncidentsByNetwork(c.Request.Context(), networkID, resolved)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, incidents)
}

// GetSecurityIncident godoc
// @Summary Get a security incident
// @Description Get details of a specific security incident
// @Tags security
// @Accept json
// @Produce json
// @Param incidentId path string true "Incident ID"
// @Success 200 {object} network.SecurityIncident
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /security/incidents/{incidentId} [get]
func (h *Handler) GetSecurityIncident(c *gin.Context) {
	incidentID := c.Param("incidentId")

	incident, err := h.service.GetSecurityIncident(c.Request.Context(), incidentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, incident)
}

// ResolveSecurityIncident godoc
// @Summary Resolve a security incident
// @Description Mark a security incident as resolved
// @Tags security
// @Accept json
// @Produce json
// @Param incidentId path string true "Incident ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /security/incidents/{incidentId}/resolve [post]
func (h *Handler) ResolveSecurityIncident(c *gin.Context) {
	incidentID := c.Param("incidentId")

	// Get user from auth context
	user := middleware.GetUserFromContext(c)
	resolvedBy := "system"
	if user != nil {
		resolvedBy = user.Email
	}

	if err := h.service.ResolveSecurityIncident(c.Request.Context(), incidentID, resolvedBy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Incident resolved successfully"})
}

// ReconnectPeer godoc
// @Summary Reconnect a peer to jump servers
// @Description Re-establish connections between a peer and all jump servers in the network
// @Tags peers
// @Accept json
// @Produce json
// @Param networkId path string true "Network ID"
// @Param peerId path string true "Peer ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /networks/{networkId}/peers/{peerId}/reconnect [post]
// @Security     BearerAuth
func (h *Handler) ReconnectPeer(c *gin.Context) {
	networkID := c.Param("networkId")
	peerID := c.Param("peerId")

	if err := h.service.ReconnectPeer(c.Request.Context(), networkID, peerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Peer reconnected successfully"})
}
