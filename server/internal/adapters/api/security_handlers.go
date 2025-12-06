package api

import (
	"net/http"
	"time"

	"wirety/internal/adapters/api/middleware"
	"wirety/internal/domain/network"

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
	user := middleware.GetUserFromContext(c)
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

	// Filter incidents for non-admin users - only show incidents for their own peers
	if user != nil && !user.IsAdministrator() {
		var filteredIncidents []*network.SecurityIncident
		for _, incident := range incidents {
			// Get the peer to check ownership
			peer, err := h.service.GetPeer(c.Request.Context(), incident.NetworkID, incident.PeerID)
			if err == nil && peer.OwnerID == user.ID {
				filteredIncidents = append(filteredIncidents, incident)
			}
		}
		c.JSON(http.StatusOK, filteredIncidents)
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
	user := middleware.GetUserFromContext(c)

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

	// Filter incidents for non-admin users - only show incidents for their own peers
	if user != nil && !user.IsAdministrator() {
		var filteredIncidents []*network.SecurityIncident
		for _, incident := range incidents {
			peer, err := h.service.GetPeer(c.Request.Context(), networkID, incident.PeerID)
			if err == nil && peer.OwnerID == user.ID {
				filteredIncidents = append(filteredIncidents, incident)
			}
		}
		c.JSON(http.StatusOK, filteredIncidents)
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
	user := middleware.GetUserFromContext(c)

	incident, err := h.service.GetSecurityIncident(c.Request.Context(), incidentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Check permission for non-admin users
	if user != nil && !user.IsAdministrator() {
		peer, err := h.service.GetPeer(c.Request.Context(), incident.NetworkID, incident.PeerID)
		if err != nil || peer.OwnerID != user.ID {
			c.JSON(http.StatusForbidden, gin.H{"error": "you can only view incidents for your own peers"})
			return
		}
	}

	c.JSON(http.StatusOK, incident)
}

// ResolveSecurityIncident godoc
// @Summary Resolve a security incident
// @Description Mark a security incident as resolved (admin only)
// @Tags security
// @Accept json
// @Produce json
// @Param incidentId path string true "Incident ID"
// @Success 200 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /security/incidents/{incidentId}/resolve [post]
func (h *Handler) ResolveSecurityIncident(c *gin.Context) {
	incidentID := c.Param("incidentId")

	// Get user from auth context
	user := middleware.GetUserFromContext(c)

	// Only administrators can resolve incidents
	if user == nil || !user.IsAdministrator() {
		c.JSON(http.StatusForbidden, gin.H{"error": "only administrators can resolve security incidents"})
		return
	}

	resolvedBy := user.Email

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
// func (h *Handler) ReconnectPeer(c *gin.Context) {
// 	networkID := c.Param("networkId")
// 	peerID := c.Param("peerId")

// 	if err := h.service.ReconnectPeer(c.Request.Context(), networkID, peerID); err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{"message": "Peer reconnected successfully"})
// }
// Security Config API structures

// SecurityConfigResponse represents the API response for security config
type SecurityConfigResponse struct {
	ID                       string `json:"id"`
	NetworkID                string `json:"network_id"`
	Enabled                  bool   `json:"enabled"`
	SessionConflictThreshold int64  `json:"session_conflict_threshold_minutes"` // Convert to minutes for frontend
	EndpointChangeThreshold  int64  `json:"endpoint_change_threshold_minutes"`  // Convert to minutes for frontend
	MaxEndpointChangesPerDay int    `json:"max_endpoint_changes_per_day"`
	CreatedAt                string `json:"created_at"`
	UpdatedAt                string `json:"updated_at"`
}

// SecurityConfigRequest represents the API request for security config
type SecurityConfigRequest struct {
	Enabled                  *bool  `json:"enabled,omitempty"`
	SessionConflictThreshold *int64 `json:"session_conflict_threshold_minutes,omitempty"` // Accept minutes from frontend
	EndpointChangeThreshold  *int64 `json:"endpoint_change_threshold_minutes,omitempty"`  // Accept minutes from frontend
	MaxEndpointChangesPerDay *int   `json:"max_endpoint_changes_per_day,omitempty"`
}

// convertToSecurityConfigResponse converts domain SecurityConfig to API response
func convertToSecurityConfigResponse(config *network.SecurityConfig) *SecurityConfigResponse {
	return &SecurityConfigResponse{
		ID:                       config.ID,
		NetworkID:                config.NetworkID,
		Enabled:                  config.Enabled,
		SessionConflictThreshold: int64(config.SessionConflictThreshold / time.Minute),
		EndpointChangeThreshold:  int64(config.EndpointChangeThreshold / time.Minute),
		MaxEndpointChangesPerDay: config.MaxEndpointChangesPerDay,
		CreatedAt:                config.CreatedAt.Format(time.RFC3339),
		UpdatedAt:                config.UpdatedAt.Format(time.RFC3339),
	}
}

// convertFromSecurityConfigRequest converts API request to domain SecurityConfigUpdateRequest
func convertFromSecurityConfigRequest(req *SecurityConfigRequest) *network.SecurityConfigUpdateRequest {
	updateReq := &network.SecurityConfigUpdateRequest{
		Enabled:                  req.Enabled,
		MaxEndpointChangesPerDay: req.MaxEndpointChangesPerDay,
	}

	if req.SessionConflictThreshold != nil {
		threshold := time.Duration(*req.SessionConflictThreshold) * time.Minute
		updateReq.SessionConflictThreshold = &threshold
	}

	if req.EndpointChangeThreshold != nil {
		threshold := time.Duration(*req.EndpointChangeThreshold) * time.Minute
		updateReq.EndpointChangeThreshold = &threshold
	}

	return updateReq
}

// GetNetworkSecurityConfig godoc
// @Summary Get network security configuration
// @Description Get the security configuration for a specific network (admin only)
// @Tags security
// @Accept json
// @Produce json
// @Param networkId path string true "Network ID"
// @Success 200 {object} SecurityConfigResponse
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /networks/{networkId}/security/config [get]
// @Security BearerAuth
func (h *Handler) GetNetworkSecurityConfig(c *gin.Context) {
	networkID := c.Param("networkId")

	config, err := h.service.GetSecurityConfig(c.Request.Context(), networkID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	response := convertToSecurityConfigResponse(config)
	c.JSON(http.StatusOK, response)
}

// UpdateNetworkSecurityConfig godoc
// @Summary Update network security configuration
// @Description Update the security configuration for a specific network (admin only)
// @Tags security
// @Accept json
// @Produce json
// @Param networkId path string true "Network ID"
// @Param config body SecurityConfigRequest true "Security configuration"
// @Success 200 {object} SecurityConfigResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /networks/{networkId}/security/config [put]
// @Security BearerAuth
func (h *Handler) UpdateNetworkSecurityConfig(c *gin.Context) {
	networkID := c.Param("networkId")

	var req SecurityConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate request
	if req.SessionConflictThreshold != nil && *req.SessionConflictThreshold < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session conflict threshold must be at least 1 minute"})
		return
	}
	if req.EndpointChangeThreshold != nil && *req.EndpointChangeThreshold < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Endpoint change threshold must be at least 1 minute"})
		return
	}
	if req.MaxEndpointChangesPerDay != nil && (*req.MaxEndpointChangesPerDay < 1 || *req.MaxEndpointChangesPerDay > 1000) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Max endpoint changes per day must be between 1 and 1000"})
		return
	}

	updateReq := convertFromSecurityConfigRequest(&req)
	config, err := h.service.UpdateSecurityConfig(c.Request.Context(), networkID, updateReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response := convertToSecurityConfigResponse(config)
	c.JSON(http.StatusOK, response)
}
