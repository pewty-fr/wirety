package api

import (
	"net/http"
	"strconv"

	"wirety/internal/application/ipam"
	"wirety/internal/application/network"
	domain "wirety/internal/domain/network"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"     // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger" // gin-swagger middleware

	_ "wirety/docs" // swagger docs
)

// Handler handles HTTP requests for the network API
type Handler struct {
	service     *network.Service
	ipamService *ipam.Service
	wsManager   *WebSocketManager
}

// NewHandler creates a new API handler
func NewHandler(service *network.Service, ipamService *ipam.Service) *Handler {
	return &Handler{
		service:     service,
		ipamService: ipamService,
		wsManager:   NewWebSocketManager(service),
	}
}

// RegisterRoutes registers all API routes
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/v1")
	{
		// Agent enrollment / token resolution
		api.GET("/agent/resolve", h.ResolveAgent)
		// Network routes
		networks := api.Group("/networks")
		{
			networks.POST("", h.CreateNetwork)
			networks.GET("", h.ListNetworks)
			networks.GET("/:networkId", h.GetNetwork)
			networks.PUT("/:networkId", h.UpdateNetwork)
			networks.DELETE("/:networkId", h.DeleteNetwork)

			// Peer routes
			peers := networks.Group("/:networkId/peers")
			{
				peers.POST("", h.CreatePeer)
				peers.GET("", h.ListPeers)
				peers.GET("/:peerId", h.GetPeer)
				peers.PUT("/:peerId", h.UpdatePeer)
				peers.DELETE("/:peerId", h.DeletePeer)
				peers.GET("/:peerId/config", h.GetPeerConfig)
			}

			// ACL routes
			acl := networks.Group("/:networkId/acl")
			{
				acl.GET("", h.GetACL)
				acl.PUT("", h.UpdateACL)
			}
		}

		// WebSocket endpoints for config updates (legacy by IDs and token-based)
		api.GET("/ws/:networkId/:peerId", h.HandleWebSocket) // legacy
		api.GET("/ws", h.HandleWebSocketToken)               // token-based (?token=...)
		api.GET("/health", h.Health)
		api.GET("/ipam/available-cidrs", h.GetAvailableCIDRs)
	}
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

// GetAvailableCIDRs godoc
//
// @Summary      Suggest available CIDRs
// @Description  Returns a list of CIDRs sized to hold at least max_peers peers carved from base_cidr
// @Tags         ipam
// @Produce      json
// @Param        max_peers  query int true  "Maximum number of peers to fit in each CIDR"
// @Param        count      query int false "How many CIDRs to return" default(1)
// @Param        base_cidr  query string false "Root CIDR to carve from" default(10.0.0.0/8)
// @Success      200 {object} map[string]any
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /ipam/available-cidrs [get]
func (h *Handler) GetAvailableCIDRs(c *gin.Context) {
	maxPeersStr := c.Query("max_peers")
	if maxPeersStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max_peers query parameter is required"})
		return
	}
	maxPeers, err := strconv.Atoi(maxPeersStr)
	if err != nil || maxPeers <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max_peers must be a positive integer"})
		return
	}
	countStr := c.DefaultQuery("count", "1")
	count, err := strconv.Atoi(countStr)
	if err != nil || count <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "count must be a positive integer"})
		return
	}
	baseCIDR := c.DefaultQuery("base_cidr", "10.0.0.0/8")

	prefixLen, cidrs, err := h.ipamService.SuggestCIDRs(c.Request.Context(), baseCIDR, maxPeers, count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	usable := (1 << (32 - prefixLen)) - 2
	c.JSON(http.StatusOK, gin.H{
		"base_cidr":           baseCIDR,
		"requested_max_peers": maxPeers,
		"suggested_prefix":    prefixLen,
		"usable_hosts":        usable,
		"cidrs":               cidrs,
	})
}

// CreateNetwork godoc
//
//	@Summary		Create a new network
//	@Description	Create a new WireGuard network
//	@Tags			networks
//	@Accept			json
//	@Produce		json
//	@Param			network	body		domain.NetworkCreateRequest	true	"Network creation request"
//	@Success		201		{object}	domain.Network
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Router			/networks [post]
func (h *Handler) CreateNetwork(c *gin.Context) {
	var req domain.NetworkCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	net, err := h.service.CreateNetwork(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, net)
}

// GetNetwork godoc
//
//	@Summary		Get a network
//	@Description	Get a network by ID
//	@Tags			networks
//	@Produce		json
//	@Param			networkId	path		string	true	"Network ID"
//	@Success		200			{object}	domain.Network
//	@Failure		404			{object}	map[string]string
//	@Router			/networks/{networkId} [get]
func (h *Handler) GetNetwork(c *gin.Context) {
	networkID := c.Param("networkId")

	net, err := h.service.GetNetwork(c.Request.Context(), networkID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, net)
}

// ListNetworks godoc
//
//	@Summary		List all networks
//	@Description	Get a list of all networks
//	@Tags			networks
//	@Produce		json
//	@Success		200	{array}		domain.Network
//	@Failure		500	{object}	map[string]string
//	@Router			/networks [get]
func (h *Handler) ListNetworks(c *gin.Context) {
	networks, err := h.service.ListNetworks(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, networks)
}

// UpdateNetwork godoc
//
//	@Summary		Update a network
//	@Description	Update a network's configuration
//	@Tags			networks
//	@Accept			json
//	@Produce		json
//	@Param			networkId	path		string						true	"Network ID"
//	@Param			network		body		domain.NetworkUpdateRequest	true	"Network update request"
//	@Success		200			{object}	domain.Network
//	@Failure		400			{object}	map[string]string
//	@Failure		404			{object}	map[string]string
//	@Router			/networks/{networkId} [put]
func (h *Handler) UpdateNetwork(c *gin.Context) {
	networkID := c.Param("networkId")

	var req domain.NetworkUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	net, err := h.service.UpdateNetwork(c.Request.Context(), networkID, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, net)
}

// DeleteNetwork godoc
//
//	@Summary		Delete a network
//	@Description	Delete a network by ID
//	@Tags			networks
//	@Param			networkId	path	string	true	"Network ID"
//	@Success		204
//	@Failure		404	{object}	map[string]string
//	@Router			/networks/{networkId} [delete]
//	@Summary		Delete a network
//	@Description	Delete a network by ID
//	@Tags			networks
//	@Param			networkId	path	string	true	"Network ID"
//	@Success		204
//	@Failure		404	{object}	map[string]string
//	@Router			/networks/{networkId} [delete]
func (h *Handler) DeleteNetwork(c *gin.Context) {
	networkID := c.Param("networkId")

	if err := h.service.DeleteNetwork(c.Request.Context(), networkID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
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
func (h *Handler) CreatePeer(c *gin.Context) {
	networkID := c.Param("networkId")

	var req domain.PeerCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	peer, err := h.service.AddPeer(c.Request.Context(), networkID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Notify all connected peers in the network about the topology change
	go h.wsManager.NotifyNetworkPeers(networkID)

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
func (h *Handler) GetPeer(c *gin.Context) {
	networkID := c.Param("networkId")
	peerID := c.Param("peerId")

	peer, err := h.service.GetPeer(c.Request.Context(), networkID, peerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, peer)
}

// ListPeers godoc
//
//	@Summary		List all peers
//	@Description	Get a list of all peers in a network
//	@Tags			peers
//	@Produce		json
//	@Param			networkId	path		string	true	"Network ID"
//	@Success		200			{array}		domain.Peer
//	@Failure		500			{object}	map[string]string
//	@Router			/networks/{networkId}/peers [get]
func (h *Handler) ListPeers(c *gin.Context) {
	networkID := c.Param("networkId")

	peers, err := h.service.ListPeers(c.Request.Context(), networkID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, peers)
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
func (h *Handler) UpdatePeer(c *gin.Context) {
	networkID := c.Param("networkId")
	peerID := c.Param("peerId")

	var req domain.PeerUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	peer, err := h.service.UpdatePeer(c.Request.Context(), networkID, peerID, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Notify all connected peers in the network about the configuration change
	go h.wsManager.NotifyNetworkPeers(networkID)

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
func (h *Handler) DeletePeer(c *gin.Context) {
	networkID := c.Param("networkId")
	peerID := c.Param("peerId")

	if err := h.service.DeletePeer(c.Request.Context(), networkID, peerID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Notify all connected peers in the network about the topology change
	go h.wsManager.NotifyNetworkPeers(networkID)

	c.Status(http.StatusNoContent)
}

// GetPeerConfig godoc
//
//	@Summary		Get peer configuration
//	@Description	Get WireGuard configuration for a specific peer
//	@Tags			peers
//	@Produce		plain
//	@Param			networkId	path		string	true	"Network ID"
//	@Param			peerId		path		string	true	"Peer ID"
//	@Success		200			{string}	string	"WireGuard configuration"
//	@Failure		404			{object}	map[string]string
//	@Router			/networks/{networkId}/peers/{peerId}/config [get]
func (h *Handler) GetPeerConfig(c *gin.Context) {
	networkID := c.Param("networkId")
	peerID := c.Param("peerId")

	config, err := h.service.GeneratePeerConfig(c.Request.Context(), networkID, peerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.String(http.StatusOK, config)
}

// GetACL godoc
//
//	@Summary		Get ACL configuration
//	@Description	Get ACL configuration for a network
//	@Tags			acl
//	@Produce		json
//	@Param			networkId	path		string	true	"Network ID"
//	@Success		200			{object}	domain.ACL
//	@Failure		404			{object}	map[string]string
//	@Router			/networks/{networkId}/acl [get]
func (h *Handler) GetACL(c *gin.Context) {
	networkID := c.Param("networkId")

	acl, err := h.service.GetACL(c.Request.Context(), networkID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, acl)
}

// UpdateACL godoc
//
//	@Summary		Update ACL configuration
//	@Description	Update ACL configuration for a network
//	@Tags			acl
//	@Accept			json
//	@Produce		json
//	@Param			networkId	path		string		true	"Network ID"
//	@Param			acl			body		domain.ACL	true	"ACL configuration"
//	@Success		200			{object}	domain.ACL
//	@Failure		400			{object}	map[string]string
//	@Failure		404			{object}	map[string]string
//	@Router			/networks/{networkId}/acl [put]
func (h *Handler) UpdateACL(c *gin.Context) {
	networkID := c.Param("networkId")

	var acl domain.ACL
	if err := c.ShouldBindJSON(&acl); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateACL(c.Request.Context(), networkID, &acl); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, acl)
}

// Health godoc
//
//	@Summary		Health check
//	@Description	Check if the API is healthy
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	map[string]string
//	@Router			/health [get]
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

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
func (h *Handler) ResolveAgent(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token query parameter is required"})
		return
	}
	networkID, peer, err := h.service.ResolveAgentToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	// Generate initial config
	cfg, err := h.service.GeneratePeerConfig(c.Request.Context(), networkID, peer.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"network_id": networkID,
		"peer_id":    peer.ID,
		"config":     cfg,
	})
}
