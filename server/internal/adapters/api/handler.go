package api

import (
	"context"
	"net/http"
	"strconv"

	"wirety/internal/adapters/api/middleware"
	appauth "wirety/internal/application/auth"
	"wirety/internal/application/ipam"
	"wirety/internal/application/network"
	"wirety/internal/config"
	"wirety/internal/domain/auth"
	domain "wirety/internal/domain/network"
	"wirety/internal/infrastructure/validation"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"     // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger" // gin-swagger middleware

	_ "wirety/docs" // swagger docs
)

// Handler handles HTTP requests for the network API
type Handler struct {
	service       *network.Service
	ipamService   *ipam.Service
	authService   *appauth.Service
	groupService  GroupService
	policyService PolicyService
	routeService  RouteService
	dnsService    DNSService
	wsManager     *WebSocketManager
	userRepo      auth.Repository
	groupRepo     domain.GroupRepository
}

// GroupService defines the interface for group operations
type GroupService interface {
	CreateGroup(ctx context.Context, networkID string, req *domain.GroupCreateRequest) (*domain.Group, error)
	GetGroup(ctx context.Context, networkID, groupID string) (*domain.Group, error)
	UpdateGroup(ctx context.Context, networkID, groupID string, req *domain.GroupUpdateRequest) (*domain.Group, error)
	DeleteGroup(ctx context.Context, networkID, groupID string) error
	ListGroups(ctx context.Context, networkID string) ([]*domain.Group, error)
	AddPeerToGroup(ctx context.Context, networkID, groupID, peerID string) error
	RemovePeerFromGroup(ctx context.Context, networkID, groupID, peerID string) error
	AttachPolicyToGroup(ctx context.Context, networkID, groupID, policyID string) error
	DetachPolicyFromGroup(ctx context.Context, networkID, groupID, policyID string) error
	GetGroupPolicies(ctx context.Context, networkID, groupID string) ([]*domain.Policy, error)
	AttachRouteToGroup(ctx context.Context, networkID, groupID, routeID string) error
	DetachRouteFromGroup(ctx context.Context, networkID, groupID, routeID string) error
}

// PolicyTemplate represents a predefined policy template
type PolicyTemplate struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Rules       []domain.PolicyRule `json:"rules"`
}

// PolicyService defines the interface for policy operations
type PolicyService interface {
	CreatePolicy(ctx context.Context, networkID string, req *domain.PolicyCreateRequest) (*domain.Policy, error)
	GetPolicy(ctx context.Context, networkID, policyID string) (*domain.Policy, error)
	UpdatePolicy(ctx context.Context, networkID, policyID string, req *domain.PolicyUpdateRequest) (*domain.Policy, error)
	DeletePolicy(ctx context.Context, networkID, policyID string) error
	ListPolicies(ctx context.Context, networkID string) ([]*domain.Policy, error)
	AddRuleToPolicy(ctx context.Context, networkID, policyID string, rule *domain.PolicyRule) error
	RemoveRuleFromPolicy(ctx context.Context, networkID, policyID, ruleID string) error
	GetDefaultTemplates() []PolicyTemplate
}

// RouteService defines the interface for route operations
type RouteService interface {
	CreateRoute(ctx context.Context, networkID string, req *domain.RouteCreateRequest) (*domain.Route, error)
	GetRoute(ctx context.Context, networkID, routeID string) (*domain.Route, error)
	UpdateRoute(ctx context.Context, networkID, routeID string, req *domain.RouteUpdateRequest) (*domain.Route, error)
	DeleteRoute(ctx context.Context, networkID, routeID string) error
	ListRoutes(ctx context.Context, networkID string) ([]*domain.Route, error)
	GetPeerRoutes(ctx context.Context, networkID, peerID string) ([]*domain.Route, error)
}

// DNSRecord represents a combined DNS record (peer or route-based)
type DNSRecord struct {
	Name      string `json:"name"`
	IPAddress string `json:"ip_address"`
	FQDN      string `json:"fqdn"`
	Type      string `json:"type"` // "peer" or "route"
}

// DNSService defines the interface for DNS mapping operations
type DNSService interface {
	CreateDNSMapping(ctx context.Context, networkID, routeID string, req *domain.DNSMappingCreateRequest) (*domain.DNSMapping, error)
	GetDNSMapping(ctx context.Context, routeID, mappingID string) (*domain.DNSMapping, error)
	UpdateDNSMapping(ctx context.Context, networkID, routeID, mappingID string, req *domain.DNSMappingUpdateRequest) (*domain.DNSMapping, error)
	DeleteDNSMapping(ctx context.Context, networkID, routeID, mappingID string) error
	ListDNSMappings(ctx context.Context, networkID, routeID string) ([]*domain.DNSMapping, error)
	GetNetworkDNSRecords(ctx context.Context, networkID string) ([]DNSRecord, error)
}

// PaginatedNetworks represents a paginated list of networks
type PaginatedNetworks struct {
	Data     []*domain.Network `json:"data"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

// PaginatedPeers represents a paginated list of peers
type PaginatedPeers struct {
	Data     []*domain.Peer `json:"data"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

// NewHandler creates a new API handler
func NewHandler(service *network.Service, ipamService *ipam.Service, authService *appauth.Service, groupService GroupService, policyService PolicyService, routeService RouteService, dnsService DNSService, groupRepo domain.GroupRepository, userRepo auth.Repository, authConfig *config.AuthConfig) *Handler {
	wsManager := NewWebSocketManager(service, authConfig)

	// Set the WebSocket notifier on the service so it can trigger config updates
	service.SetWebSocketNotifier(wsManager)

	// Set the WebSocket connection checker so the service can check if agents are connected
	service.SetWebSocketConnectionChecker(wsManager)

	return &Handler{
		service:       service,
		ipamService:   ipamService,
		authService:   authService,
		groupService:  groupService,
		policyService: policyService,
		routeService:  routeService,
		dnsService:    dnsService,
		wsManager:     wsManager,
		userRepo:      userRepo,
		groupRepo:     groupRepo,
	}
}

// RegisterRoutes registers all API routes
func (h *Handler) RegisterRoutes(r *gin.Engine, authMiddleware gin.HandlerFunc, requireAdmin gin.HandlerFunc, requireNetworkAccess gin.HandlerFunc) {
	api := r.Group("/api/v1")

	// Public routes (no auth required)
	{
		api.GET("/health", h.Health)
		api.GET("/auth/config", h.GetAuthConfig)
		api.POST("/auth/token", h.ExchangeToken)
		api.POST("/auth/logout", h.Logout)
		// Captive portal authentication
		api.POST("/captive-portal/authenticate", h.AuthenticateCaptivePortal)
		// Agent endpoints (token-based authentication, not OIDC)
		api.GET("/agent/resolve", h.ResolveAgent)
		api.GET("/agent/captive-portal-token", h.GetCaptivePortalToken)
		api.GET("/ws", h.HandleWebSocketToken)               // token-based WebSocket
		api.GET("/ws/:networkId/:peerId", h.HandleWebSocket) // legacy WebSocket
	}

	// Protected routes (auth required)
	protected := api.Group("")
	protected.Use(authMiddleware)
	{
		// User management routes
		users := protected.Group("/users")
		{
			users.GET("/me", h.GetCurrentUser)
			// Admin only routes
			adminUsers := users.Group("")
			adminUsers.Use(requireAdmin)
			{
				adminUsers.GET("", h.ListUsers)
				adminUsers.GET("/defaults", h.GetDefaultPermissions)
				adminUsers.PUT("/defaults", h.UpdateDefaultPermissions)
				adminUsers.GET("/:userId", h.GetUser)
				adminUsers.PUT("/:userId", h.UpdateUser)
				adminUsers.DELETE("/:userId", h.DeleteUser)
			}
		}

		// Network routes (admin or authorized users)
		networks := protected.Group("/networks")
		{
			// List/Create networks - admin only
			networks.GET("", h.ListNetworks)
			networks.POST("", requireAdmin, h.CreateNetwork)

			// Specific network operations - requires network access
			networkOps := networks.Group("/:networkId")
			networkOps.Use(requireNetworkAccess)
			{
				networkOps.GET("", h.GetNetwork)
				networkOps.PUT("", requireAdmin, h.UpdateNetwork)
				networkOps.DELETE("", requireAdmin, h.DeleteNetwork)

				// Peer routes
				peers := networkOps.Group("/peers")
				{
					peers.POST("", h.CreatePeer)
					peers.GET("", h.ListPeers)
					peers.GET("/:peerId", h.GetPeer)
					peers.PUT("/:peerId", h.UpdatePeer)
					peers.DELETE("/:peerId", h.DeletePeer)
					peers.GET("/:peerId/config", h.GetPeerConfig)
					peers.GET("/:peerId/session", h.GetPeerSessionStatus)
					peers.POST("/:peerId/reconnect", h.ReconnectPeer)
				}

				// Network session management (admin only)
				networkOps.GET("/sessions", h.ListNetworkSessions)

				// Network security incidents
				networkOps.GET("/security/incidents", h.ListNetworkSecurityIncidents)

				// ACL routes (admin only)
				acl := networkOps.Group("/acl")
				acl.Use(requireAdmin)
				{
					acl.GET("", h.GetACL)
					acl.PUT("", h.UpdateACL)
				}

				// Group routes (admin only)
				groups := networkOps.Group("/groups")
				groups.Use(requireAdmin)
				{
					groups.POST("", h.CreateGroup)
					groups.GET("", h.ListGroups)
					groups.GET("/:groupId", h.GetGroup)
					groups.PUT("/:groupId", h.UpdateGroup)
					groups.DELETE("/:groupId", h.DeleteGroup)

					// Group membership routes
					groups.POST("/:groupId/peers/:peerId", h.AddPeerToGroup)
					groups.DELETE("/:groupId/peers/:peerId", h.RemovePeerFromGroup)

					// Group policy attachment routes
					groups.POST("/:groupId/policies/:policyId", h.AttachPolicyToGroup)
					groups.DELETE("/:groupId/policies/:policyId", h.DetachPolicyFromGroup)
					groups.GET("/:groupId/policies", h.GetGroupPolicies)
				}

				// Policy routes (admin only)
				policies := networkOps.Group("/policies")
				policies.Use(requireAdmin)
				{
					policies.GET("/templates", h.GetDefaultTemplates)
					policies.POST("", h.CreatePolicy)
					policies.GET("", h.ListPolicies)
					policies.GET("/:policyId", h.GetPolicy)
					policies.PUT("/:policyId", h.UpdatePolicy)
					policies.DELETE("/:policyId", h.DeletePolicy)

					// Policy rule routes
					policies.POST("/:policyId/rules", h.AddRuleToPolicy)
					policies.DELETE("/:policyId/rules/:ruleId", h.RemoveRuleFromPolicy)
				}

				// Route routes (admin only)
				routes := networkOps.Group("/routes")
				routes.Use(requireAdmin)
				{
					routes.POST("", h.CreateRoute)
					routes.GET("", h.ListRoutes)
					routes.GET("/:routeId", h.GetRoute)
					routes.PUT("/:routeId", h.UpdateRoute)
					routes.DELETE("/:routeId", h.DeleteRoute)

					// DNS mapping routes for routes (admin only)
					routes.POST("/:routeId/dns", h.CreateDNSMapping)
					routes.GET("/:routeId/dns", h.ListDNSMappings)
					routes.PUT("/:routeId/dns/:dnsId", h.UpdateDNSMapping)
					routes.DELETE("/:routeId/dns/:dnsId", h.DeleteDNSMapping)
				}

				// Network DNS listing (admin only)
				networkOps.GET("/dns", requireAdmin, h.GetNetworkDNSRecords)

				// Group route attachment routes (admin only)
				groups.POST("/:groupId/routes/:routeId", h.AttachRouteToGroup)
				groups.DELETE("/:groupId/routes/:routeId", h.DetachRouteFromGroup)
				groups.GET("/:groupId/routes", h.GetGroupRoutes)
			}
		}

		// IPAM routes
		ipam := protected.Group("/ipam")
		{
			ipam.GET("/available-cidrs", h.GetAvailableCIDRs)
			ipam.GET("", h.ListIPAMAllocations)
			ipam.GET("/networks/:networkId", requireNetworkAccess, h.GetNetworkIPAM)
		}

		// Security routes
		security := protected.Group("/security")
		{
			security.GET("/incidents", h.ListSecurityIncidents)
			security.GET("/incidents/:incidentId", h.GetSecurityIncident)
			security.POST("/incidents/:incidentId/resolve", h.ResolveSecurityIncident)
		}
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
// @Security     BearerAuth
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

// isValidationError checks if an error is a validation error and returns appropriate status code
func isValidationError(err error) bool {
	return err == validation.ErrInvalidDNSName ||
		err == validation.ErrNameTooLong ||
		err == validation.ErrNameEmpty ||
		err == validation.ErrNameStartsWithHyphen ||
		err == validation.ErrNameEndsWithHyphen
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
//
// @Security     BearerAuth
func (h *Handler) CreateNetwork(c *gin.Context) {
	var req domain.NetworkCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	net, err := h.service.CreateNetwork(c.Request.Context(), &req)
	if err != nil {
		// Check if it's a validation error
		if isValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
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
//
// @Security     BearerAuth
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
// @Summary      List networks (paginated)
// @Description  Get a paginated list of networks. Supports optional filtering by name or CIDR substring.
// @Tags         networks
// @Produce      json
// @Param        page      query int    false "Page number" default(1)
// @Param        page_size query int    false "Page size" default(20)
// @Param        filter    query string false "Filter by network name or CIDR"
// @Success      200 {object} PaginatedNetworks
// @Failure      500 {object} map[string]string
// @Router       /networks [get]
// @Security     BearerAuth
func (h *Handler) ListNetworks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	filter := c.Query("filter")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	networks, err := h.service.ListNetworks(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	user := middleware.GetUserFromContext(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found in context"})
		c.Abort()
		return
	}

	// Ensure user has access to network
	var hasAccess []*domain.Network
	for _, n := range networks {
		if user.HasNetworkAccess(n.ID) {
			hasAccess = append(hasAccess, n)
		}
	}
	networks = hasAccess

	// Apply filtering
	var filtered []*domain.Network
	if filter != "" {
		for _, n := range networks {
			if containsIgnoreCase(n.Name, filter) || containsIgnoreCase(n.CIDR, filter) || containsIgnoreCase(n.ID, filter) {
				filtered = append(filtered, n)
			}
		}
	} else {
		filtered = networks
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

	c.JSON(http.StatusOK, PaginatedNetworks{
		Data:     filtered[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
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
//
// @Security     BearerAuth
func (h *Handler) UpdateNetwork(c *gin.Context) {
	networkID := c.Param("networkId")

	var req domain.NetworkUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	net, err := h.service.UpdateNetwork(c.Request.Context(), networkID, &req)
	if err != nil {
		// Check if it's a validation error
		if isValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		}
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
//	@Description	Delete a network by ID/networks/{networkId} [delete]
//
// @Security     BearerAuth
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
//	@Security		BearerAuth
func (h *Handler) CreatePeer(c *gin.Context) {
	networkID := c.Param("networkId")
	user := middleware.GetUserFromContext(c)

	var req domain.PeerCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set owner ID for non-admin users
	ownerID := ""
	if user != nil && !user.IsAdministrator() {
		ownerID = user.ID
	}

	peer, err := h.service.AddPeer(c.Request.Context(), networkID, &req, ownerID)
	if err != nil {
		// Check if it's a validation error
		if isValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
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

	// Check permission: admins can view any peer, users can only view their own
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
	if pageSize < 1 || pageSize > 500 { // peers per network might be larger
		pageSize = 20
	}

	peers, err := h.service.ListPeers(c.Request.Context(), networkID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Filter by ownership for non-admin users
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

	// Get the peer to check ownership
	peer, err := h.service.GetPeer(c.Request.Context(), networkID, peerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "peer not found"})
		return
	}

	// Check permission: admins can update any peer, users can only update their own
	if user != nil && !user.CanManagePeer(networkID, peer.OwnerID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "you can only manage your own peers"})
		return
	}

	var req domain.PeerUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Only admins can change owner
	if req.OwnerID != "" && user != nil && !user.IsAdministrator() {
		c.JSON(http.StatusForbidden, gin.H{"error": "only administrators can change peer ownership"})
		return
	}

	peer, err = h.service.UpdatePeer(c.Request.Context(), networkID, peerID, &req)
	if err != nil {
		// Check if it's a validation error
		if isValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		}
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
//	@Security		BearerAuth
func (h *Handler) DeletePeer(c *gin.Context) {
	networkID := c.Param("networkId")
	peerID := c.Param("peerId")
	user := middleware.GetUserFromContext(c)

	// Get the peer to check ownership
	peer, err := h.service.GetPeer(c.Request.Context(), networkID, peerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "peer not found"})
		return
	}

	// Check permission: admins can delete any peer, users can only delete their own
	if user != nil && !user.CanManagePeer(networkID, peer.OwnerID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "you can only manage your own peers"})
		return
	}

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

	// Get the peer to check ownership
	peer, err := h.service.GetPeer(c.Request.Context(), networkID, peerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "peer not found"})
		return
	}

	// Check permission: admins can view any config, users can only view their own
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
//
// @Security     BearerAuth
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
//
// @Security     BearerAuth
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

	// Notify all connected peers about the ACL change so they update their configs
	h.wsManager.NotifyNetworkPeers(networkID)

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
//
// @Security     BearerAuth
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
// @Security     BearerAuth
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
		"peer_name":  peer.Name,
		"config":     cfg,
	})
}

// IPAMAllocation represents an IP allocation with network and peer information
type IPAMAllocation struct {
	NetworkID   string `json:"network_id"`
	NetworkName string `json:"network_name"`
	NetworkCIDR string `json:"network_cidr"`
	IP          string `json:"ip"`
	PeerID      string `json:"peer_id,omitempty"`
	PeerName    string `json:"peer_name,omitempty"`
	Allocated   bool   `json:"allocated"`
}

// ListIPAMAllocations godoc
// @Summary      List all IP allocations
// @Description  Get a list of all IP allocations across all networks with pagination and filtering
// @Tags         ipam
// @Produce      json
// @Param        page      query int    false "Page number" default(1)
// @Param        page_size query int    false "Page size" default(20)
// @Param        filter    query string false "Filter by network name, IP, or peer name"
// @Success      200 {object} map[string]any
// @Failure      500 {object} map[string]string
// @Router       /ipam [get]
// @Security     BearerAuth
func (h *Handler) ListIPAMAllocations(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	filter := c.Query("filter")
	user := middleware.GetUserFromContext(c)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	networks, err := h.service.ListNetworks(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var allAllocations []IPAMAllocation

	for _, net := range networks {
		// Skip networks the user doesn't have access to
		if user != nil && !user.HasNetworkAccess(net.ID) {
			continue
		}

		peers, err := h.service.ListPeers(c.Request.Context(), net.ID)
		if err != nil {
			continue
		}

		// Create allocation for each peer
		for _, peer := range peers {
			// Filter by ownership for non-admin users
			if user != nil && !user.IsAdministrator() && peer.OwnerID != user.ID {
				continue
			}

			allocation := IPAMAllocation{
				NetworkID:   net.ID,
				NetworkName: net.Name,
				NetworkCIDR: net.CIDR,
				IP:          peer.Address,
				PeerID:      peer.ID,
				PeerName:    peer.Name,
				Allocated:   true,
			}

			// Apply filter if provided
			if filter != "" {
				filterLower := filter
				if !contains(allocation.NetworkName, filterLower) &&
					!contains(allocation.IP, filterLower) &&
					!contains(allocation.PeerName, filterLower) {
					continue
				}
			}

			allAllocations = append(allAllocations, allocation)
		}
	}

	// Calculate pagination
	total := len(allAllocations)
	start := (page - 1) * pageSize
	end := start + pageSize

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	paginatedData := allAllocations[start:end]

	c.JSON(http.StatusOK, gin.H{
		"data":      paginatedData,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetNetworkIPAM godoc
// @Summary      Get network IPAM allocations
// @Description  Get all IP allocations for a specific network
// @Tags         ipam
// @Produce      json
// @Param        networkId path string true "Network ID"
// @Success      200 {array} IPAMAllocation
// @Failure      404 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /ipam/networks/{networkId} [get]
// @Security     BearerAuth
func (h *Handler) GetNetworkIPAM(c *gin.Context) {
	networkID := c.Param("networkId")

	net, err := h.service.GetNetwork(c.Request.Context(), networkID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "network not found"})
		return
	}

	peers, err := h.service.ListPeers(c.Request.Context(), networkID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var allocations []IPAMAllocation
	for _, peer := range peers {
		allocations = append(allocations, IPAMAllocation{
			NetworkID:   net.ID,
			NetworkName: net.Name,
			NetworkCIDR: net.CIDR,
			IP:          peer.Address,
			PeerID:      peer.ID,
			PeerName:    peer.Name,
			Allocated:   true,
		})
	}

	c.JSON(http.StatusOK, allocations)
}

// Helper function for case-insensitive substring search
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(substr) == 0 ||
		containsIgnoreCase(s, substr))
}

func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}
