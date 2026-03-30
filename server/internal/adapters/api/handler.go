package api

import (
	"context"
	"net/http"

	appauth "wirety/internal/application/auth"
	"wirety/internal/application/ipam"
	"wirety/internal/application/network"
	"wirety/internal/adapters/api/middleware"
	"wirety/internal/config"
	"wirety/internal/domain/auth"
	domain "wirety/internal/domain/network"
	"wirety/internal/infrastructure/validation"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"     // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger" // gin-swagger middleware

	_ "wirety/docs" // swagger docs
)

// actor extracts the authenticated user's ID and email from the gin context.
// Returns empty strings when no user is present (e.g. public endpoints).
func actor(c *gin.Context) (id, email string) {
	user := middleware.GetUserFromContext(c)
	if user != nil {
		return user.ID, user.Email
	}
	return "", ""
}

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
	authConfig    *config.AuthConfig
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
	ReorderGroupPolicies(ctx context.Context, networkID, groupID string, policyIDs []string) error
	AttachRouteToGroup(ctx context.Context, networkID, groupID, routeID string) error
	DetachRouteFromGroup(ctx context.Context, networkID, groupID, routeID string) error
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

// NewHandler creates a new API handler
func NewHandler(service *network.Service, ipamService *ipam.Service, authService *appauth.Service, groupService GroupService, policyService PolicyService, routeService RouteService, dnsService DNSService, groupRepo domain.GroupRepository, userRepo auth.Repository, authConfig *config.AuthConfig) *Handler {
	wsManager := NewWebSocketManager(service, authConfig)

	service.SetWebSocketNotifier(wsManager)
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
		authConfig:    authConfig,
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
		api.POST("/auth/login", h.SimpleLogin)
		api.POST("/auth/logout", h.Logout)
		api.GET("/agent/resolve", h.ResolveAgent)
		api.GET("/ws", h.HandleWebSocketToken)               // token-based WebSocket
		api.GET("/ws/:networkId/:peerId", h.HandleWebSocket) // legacy WebSocket

		// Captive portal: token creation is agent-authenticated (enrollment token),
		// authenticate is unauthenticated (uses captive_token + session_hash).
		api.POST("/captive-portal/token", h.CreateCaptivePortalToken)
		api.POST("/captive-portal/authenticate", h.AuthenticateCaptivePortal)
	}

	// Protected routes (auth required)
	protected := api.Group("")
	protected.Use(authMiddleware)
	{
		// User management routes
		users := protected.Group("/users")
		{
			users.GET("/me", h.GetCurrentUser)
			users.GET("/me/tokens", h.ListAPITokens)
			users.POST("/me/tokens", h.CreateAPIToken)
			users.DELETE("/me/tokens/:tokenId", h.DeleteAPIToken)
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

		// Network routes
		networks := protected.Group("/networks")
		{
			networks.GET("", h.ListNetworks)
			networks.POST("", requireAdmin, h.CreateNetwork)

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
					peers.GET("/:peerId/reachability", h.GetPeerReachability)
				}

				networkOps.GET("/sessions", h.ListNetworkSessions)
				networkOps.GET("/security/incidents", h.ListNetworkSecurityIncidents)
				networkOps.GET("/security/config", requireAdmin, h.GetNetworkSecurityConfig)
				networkOps.PUT("/security/config", requireAdmin, h.UpdateNetworkSecurityConfig)

				// ACL routes (admin only)
				acl := networkOps.Group("/acl")
				acl.Use(requireAdmin)
				{
					acl.GET("", h.GetACL)
					acl.PUT("", h.UpdateACL)
				}

				// Group routes (admin only) — requires DB_ENABLED=true
				if h.groupService != nil {
					groups := networkOps.Group("/groups")
					groups.Use(requireAdmin)
					{
						groups.POST("", h.CreateGroup)
						groups.GET("", h.ListGroups)
						groups.GET("/:groupId", h.GetGroup)
						groups.PUT("/:groupId", h.UpdateGroup)
						groups.DELETE("/:groupId", h.DeleteGroup)
						groups.POST("/:groupId/peers/:peerId", h.AddPeerToGroup)
						groups.DELETE("/:groupId/peers/:peerId", h.RemovePeerFromGroup)
						groups.POST("/:groupId/policies/:policyId", h.AttachPolicyToGroup)
						groups.DELETE("/:groupId/policies/:policyId", h.DetachPolicyFromGroup)
						groups.GET("/:groupId/policies", h.GetGroupPolicies)
						groups.PUT("/:groupId/policies/order", h.ReorderGroupPolicies)
						groups.POST("/:groupId/routes/:routeId", h.AttachRouteToGroup)
						groups.DELETE("/:groupId/routes/:routeId", h.DetachRouteFromGroup)
						groups.GET("/:groupId/routes", h.GetGroupRoutes)
					}
				} else {
					networkOps.Any("/groups/*path", requireAdmin, dbOnlyHandler("groups"))
				}

				// Policy routes (admin only) — requires DB_ENABLED=true
				if h.policyService != nil {
					policies := networkOps.Group("/policies")
					policies.Use(requireAdmin)
					{
						policies.POST("", h.CreatePolicy)
						policies.GET("", h.ListPolicies)
						policies.GET("/:policyId", h.GetPolicy)
						policies.PUT("/:policyId", h.UpdatePolicy)
						policies.DELETE("/:policyId", h.DeletePolicy)
						policies.POST("/:policyId/rules", h.AddRuleToPolicy)
						policies.DELETE("/:policyId/rules/:ruleId", h.RemoveRuleFromPolicy)
					}
				} else {
					networkOps.Any("/policies/*path", requireAdmin, dbOnlyHandler("policies"))
				}

				// Route + DNS routes (admin only) — requires DB_ENABLED=true
				if h.routeService != nil {
					routes := networkOps.Group("/routes")
					routes.Use(requireAdmin)
					{
						routes.POST("", h.CreateRoute)
						routes.GET("", h.ListRoutes)
						routes.GET("/:routeId", h.GetRoute)
						routes.PUT("/:routeId", h.UpdateRoute)
						routes.DELETE("/:routeId", h.DeleteRoute)
						routes.POST("/:routeId/dns", h.CreateDNSMapping)
						routes.GET("/:routeId/dns", h.ListDNSMappings)
						routes.PUT("/:routeId/dns/:dnsId", h.UpdateDNSMapping)
						routes.DELETE("/:routeId/dns/:dnsId", h.DeleteDNSMapping)
					}
					networkOps.GET("/dns", requireAdmin, h.GetNetworkDNSRecords)
				} else {
					networkOps.Any("/routes/*path", requireAdmin, dbOnlyHandler("routes"))
					networkOps.GET("/dns", requireAdmin, dbOnlyHandler("DNS records"))
				}
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

	// MCP endpoint (SSE transport) — both GET (stream) and POST (messages) at same path
	mcpH := h.MCPHandler()
	r.GET("/mcp", authMiddleware, mcpH)
	r.POST("/mcp", authMiddleware, mcpH)

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

// dbOnlyHandler returns a 503 handler explaining a feature requires DB_ENABLED=true
func dbOnlyHandler(feature string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": feature + " require a database (set DB_ENABLED=true)",
		})
	}
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

// isValidationError checks if an error is a validation error
func isValidationError(err error) bool {
	return err == validation.ErrInvalidDNSName ||
		err == validation.ErrNameTooLong ||
		err == validation.ErrNameEmpty ||
		err == validation.ErrNameStartsWithHyphen ||
		err == validation.ErrNameEndsWithHyphen
}

// contains checks if s contains substr (case-insensitive)
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

