package api

import (
	"net/http"
	"strconv"

	"wirety/internal/adapters/api/middleware"
	"wirety/internal/audit"
	domain "wirety/internal/domain/network"

	"github.com/gin-gonic/gin"
)

// PaginatedPeers represents a paginated list of peers
type PaginatedPeers struct {
	Data     []*domain.Peer `json:"data"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

// CreatePeer godoc
//
//	@Summary		Create a new peer
//	@Description	Add a new peer to the network
//	@Tags			peers
//	@Accept			json
//	@Produce		json
//	@Param			networkId	path		string						true	"Network ID"
//	@Param			peer		body		domain.PeerCreateRequest	true	"Peer creation request"
//	@Success		201			{object}	domain.Peer
//	@Failure		400			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Router			/networks/{networkId}/peers [post]
//	@Security		BearerAuth
func (h *Handler) CreatePeer(c *gin.Context) {
	networkID := c.Param("networkId")
	user := middleware.GetUserFromContext(c)

	var req domain.PeerCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var ownerID string
	if user != nil && !user.IsAdministrator() {
		// Non-admins always own their own peers; they cannot set arbitrary owners.
		ownerID = user.ID
	} else {
		// Admins may assign any owner (or none) via owner_id in the request body.
		ownerID = req.OwnerID
	}

	peer, err := h.service.AddPeer(c.Request.Context(), networkID, &req, ownerID)
	if err != nil {
		if isValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	go h.wsManager.NotifyNetworkPeers(networkID)

	id, email := actor(c)
	audit.Server(id, email, c.ClientIP()).
		Str("action", "peer.create").
		Str("network_id", networkID).
		Str("peer_id", peer.ID).
		Str("peer_name", peer.Name).
		Msg("audit")

	c.JSON(http.StatusCreated, peer)
}

// GetPeer godoc
//
//	@Summary		Get a peer
//	@Description	Get a peer by ID
//	@Tags			peers
//	@Produce		json
//	@Param			networkId	path		string	true	"Network ID"
//	@Param			peerId		path		string	true	"Peer ID"
//	@Success		200			{object}	domain.Peer
//	@Failure		404			{object}	map[string]string
//	@Router			/networks/{networkId}/peers/{peerId} [get]
//
// @Security     BearerAuth
func (h *Handler) GetPeer(c *gin.Context) {
	networkID := c.Param("networkId")
	peerID := c.Param("peerId")
	user := middleware.GetUserFromContext(c)

	peer, err := h.service.GetPeer(c.Request.Context(), networkID, peerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if user != nil && !user.IsAdministrator() && peer.OwnerID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "you can only view your own peers"})
		return
	}

	c.JSON(http.StatusOK, peer)
}

// ListPeers godoc
//
// @Summary      List peers (paginated)
// @Description  Get a paginated list of peers in a network. Supports optional filtering by name, address (IP), or ID substring.
// @Tags         peers
// @Produce      json
// @Param        networkId path string true "Network ID"
// @Param        page      query int    false "Page number" default(1)
// @Param        page_size query int    false "Page size" default(20)
// @Param        filter    query string false "Filter by peer name, IP address or ID"
// @Success      200 {object} PaginatedPeers
// @Failure      500 {object} map[string]string
// @Router       /networks/{networkId}/peers [get]
// @Security     BearerAuth
func (h *Handler) ListPeers(c *gin.Context) {
	networkID := c.Param("networkId")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	filter := c.Query("filter")
	user := middleware.GetUserFromContext(c)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 20
	}

	peers, err := h.service.ListPeers(c.Request.Context(), networkID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var accessiblePeers []*domain.Peer
	for _, p := range peers {
		if user != nil && !user.IsAdministrator() && p.OwnerID != user.ID {
			continue
		}
		accessiblePeers = append(accessiblePeers, p)
	}

	var filtered []*domain.Peer
	if filter != "" {
		for _, p := range accessiblePeers {
			if containsIgnoreCase(p.Name, filter) || containsIgnoreCase(p.Address, filter) || containsIgnoreCase(p.ID, filter) {
				filtered = append(filtered, p)
			}
		}
	} else {
		filtered = accessiblePeers
	}

	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	c.JSON(http.StatusOK, PaginatedPeers{
		Data:     filtered[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// UpdatePeer godoc
//
//	@Summary		Update a peer
//	@Description	Update a peer's configuration
//	@Tags			peers
//	@Accept			json
//	@Produce		json
//	@Param			networkId	path		string						true	"Network ID"
//	@Param			peerId		path		string						true	"Peer ID"
//	@Param			peer		body		domain.PeerUpdateRequest	true	"Peer update request"
//	@Success		200			{object}	domain.Peer
//	@Failure		400			{object}	map[string]string
//	@Failure		404			{object}	map[string]string
//	@Router			/networks/{networkId}/peers/{peerId} [put]
//	@Security		BearerAuth
func (h *Handler) UpdatePeer(c *gin.Context) {
	networkID := c.Param("networkId")
	peerID := c.Param("peerId")
	user := middleware.GetUserFromContext(c)

	peer, err := h.service.GetPeer(c.Request.Context(), networkID, peerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "peer not found"})
		return
	}

	if user != nil && !user.CanManagePeer(networkID, peer.OwnerID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "you can only manage your own peers"})
		return
	}

	var req domain.PeerUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.OwnerID != "" && user != nil && !user.IsAdministrator() {
		c.JSON(http.StatusForbidden, gin.H{"error": "only administrators can change peer ownership"})
		return
	}

	peer, err = h.service.UpdatePeer(c.Request.Context(), networkID, peerID, &req)
	if err != nil {
		if isValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		}
		return
	}

	go h.wsManager.NotifyNetworkPeers(networkID)

	id, email := actor(c)
	audit.Server(id, email, c.ClientIP()).
		Str("action", "peer.update").
		Str("network_id", networkID).
		Str("peer_id", peerID).
		Str("peer_name", peer.Name).
		Msg("audit")

	c.JSON(http.StatusOK, peer)
}

// DeletePeer godoc
//
//	@Summary		Delete a peer
//	@Description	Delete a peer by ID
//	@Tags			peers
//	@Param			networkId	path	string	true	"Network ID"
//	@Param			peerId		path	string	true	"Peer ID"
//	@Success		204
//	@Failure		404	{object}	map[string]string
//	@Router			/networks/{networkId}/peers/{peerId} [delete]
//	@Security		BearerAuth
func (h *Handler) DeletePeer(c *gin.Context) {
	networkID := c.Param("networkId")
	peerID := c.Param("peerId")
	user := middleware.GetUserFromContext(c)

	peer, err := h.service.GetPeer(c.Request.Context(), networkID, peerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "peer not found"})
		return
	}

	if user != nil && !user.CanManagePeer(networkID, peer.OwnerID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "you can only manage your own peers"})
		return
	}

	if err := h.service.DeletePeer(c.Request.Context(), networkID, peerID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	go h.wsManager.NotifyNetworkPeers(networkID)

	id, email := actor(c)
	audit.Server(id, email, c.ClientIP()).
		Str("action", "peer.delete").
		Str("network_id", networkID).
		Str("peer_id", peerID).
		Msg("audit")

	c.Status(http.StatusNoContent)
}

// RevokePeerAuthentication godoc
//
//	@Summary		Revoke a peer's captive-portal authentication
//	@Description	Removes the peer from the captive-portal whitelist across all jump peers in the network. The next request from the peer will hit the captive portal and be redirected to SSO. Useful when a peer's session is suspected of being shared/stolen, or when rotating credentials.
//	@Tags			peers
//	@Param			networkId	path	string	true	"Network ID"
//	@Param			peerId		path	string	true	"Peer ID"
//	@Success		204
//	@Failure		403	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Router			/networks/{networkId}/peers/{peerId}/revoke-auth [post]
//	@Security		BearerAuth
func (h *Handler) RevokePeerAuthentication(c *gin.Context) {
	networkID := c.Param("networkId")
	peerID := c.Param("peerId")
	user := middleware.GetUserFromContext(c)

	peer, err := h.service.GetPeer(c.Request.Context(), networkID, peerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "peer not found"})
		return
	}

	// Same authorisation as peer management: the peer's owner OR an admin.
	if user != nil && !user.CanManagePeer(networkID, peer.OwnerID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "you can only manage your own peers"})
		return
	}

	if err := h.service.RevokePeerAuthentication(c.Request.Context(), networkID, peerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	id, email := actor(c)
	audit.Server(id, email, c.ClientIP()).
		Str("action", "peer.revoke_auth").
		Str("network_id", networkID).
		Str("peer_id", peerID).
		Msg("audit")

	c.Status(http.StatusNoContent)
}

// GetPeerConfig godoc
//
// @Summary      Get peer configuration
// @Description  Get WireGuard configuration for a specific peer returned as JSON object
// @Tags         peers
// @Produce      json
// @Param        networkId path string true "Network ID"
// @Param        peerId    path string true "Peer ID"
// @Success      200 {object} map[string]string "JSON object containing config key"
// @Failure      404 {object} map[string]string
// @Router       /networks/{networkId}/peers/{peerId}/config [get]
// @Security     BearerAuth
func (h *Handler) GetPeerConfig(c *gin.Context) {
	networkID := c.Param("networkId")
	peerID := c.Param("peerId")
	user := middleware.GetUserFromContext(c)

	peer, err := h.service.GetPeer(c.Request.Context(), networkID, peerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "peer not found"})
		return
	}

	if user != nil && !user.IsAdministrator() && peer.OwnerID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "you can only view your own peer configuration"})
		return
	}

	config, err := h.service.GeneratePeerConfig(c.Request.Context(), networkID, peerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"config": config})
}
