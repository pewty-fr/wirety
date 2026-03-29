package api

// MCP server embedded in the main Wirety server.
//
// Exposed at GET/POST /mcp (Streamable HTTP transport, MCP 2025-03-26 spec).
// Authentication: same API tokens as the REST API (Authorization: Bearer wirety_*).
// The authenticated user is injected into the request context so tool
// handlers can filter results by network access and enforce admin-only ops.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"wirety/internal/adapters/api/middleware"
	domainauth "wirety/internal/domain/auth"
	domain "wirety/internal/domain/network"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpUserKey is the context key used to pass the authenticated user to MCP tools.
type mcpUserKey struct{}

// MCPHandler returns a gin.HandlerFunc that serves the embedded MCP endpoint
// using the Streamable HTTP transport (MCP 2025-03-26 spec), which is required
// by Claude Desktop and Claude Code. It must be mounted behind the auth middleware.
func (h *Handler) MCPHandler() gin.HandlerFunc {
	server := h.buildMCPServer()
	streamHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, nil)

	return func(c *gin.Context) {
		// Inject the already-authenticated user into the standard request context
		// so MCP tool handlers can read it without depending on gin.
		user := middleware.GetUserFromContext(c)
		ctx := context.WithValue(c.Request.Context(), mcpUserKey{}, user)
		c.Request = c.Request.WithContext(ctx)
		streamHandler.ServeHTTP(c.Writer, c.Request)
	}
}

// mcpUser extracts the authenticated user from a tool handler context.
func mcpUserFrom(ctx context.Context) *domainauth.User {
	u, _ := ctx.Value(mcpUserKey{}).(*domainauth.User)
	return u
}

// ok returns a successful JSON tool result.
func ok(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcpErr(fmt.Sprintf("marshal: %v", err)), nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

// mcpErr returns a tool error result.
func mcpErr(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}
}

// buildMCPServer constructs the shared *mcp.Server with all Wirety tools.
// Tool handlers call domain services directly — no HTTP round-trips.
func (h *Handler) buildMCPServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "wirety", Version: "v1.0.0"}, nil)

	// ── Current user ──────────────────────────────────────────────────────────
	mcp.AddTool(s,
		&mcp.Tool{Name: "get_current_user", Description: "Get the authenticated user profile."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
			user := mcpUserFrom(ctx)
			if user == nil {
				return mcpErr("not authenticated"), nil, nil
			}
			return ok(user)
		},
	)

	// ── Users (admin only) ────────────────────────────────────────────────────
	mcp.AddTool(s,
		&mcp.Tool{Name: "list_users", Description: "List all users (admin only)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
			user := mcpUserFrom(ctx)
			if user == nil || user.Role != "administrator" {
				return mcpErr("admin access required"), nil, nil
			}
			users, err := h.userRepo.ListUsers()
			if err != nil {
				return mcpErr(err.Error()), nil, nil
			}
			return ok(users)
		},
	)

	// ── Networks ──────────────────────────────────────────────────────────────
	mcp.AddTool(s,
		&mcp.Tool{Name: "list_networks", Description: "List all WireGuard networks accessible to the current user."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
			user := mcpUserFrom(ctx)
			networks, err := h.service.ListNetworks(ctx)
			if err != nil {
				return mcpErr(err.Error()), nil, nil
			}
			var filtered []*domain.Network
			for _, n := range networks {
				if user != nil && user.HasNetworkAccess(n.ID) {
					filtered = append(filtered, n)
				}
			}
			return ok(filtered)
		},
	)

	type NetworkIDParams struct {
		NetworkID string `json:"network_id"`
	}

	mcp.AddTool(s,
		&mcp.Tool{Name: "get_network", Description: "Get details of a network by ID."},
		func(ctx context.Context, _ *mcp.CallToolRequest, p NetworkIDParams) (*mcp.CallToolResult, any, error) {
			net, err := h.service.GetNetwork(ctx, p.NetworkID)
			if err != nil {
				return mcpErr(err.Error()), nil, nil
			}
			return ok(net)
		},
	)

	type CreateNetworkParams struct {
		Name         string   `json:"name"`
		CIDR         string   `json:"cidr"`
		DNS          []string `json:"dns,omitempty"`
		DomainSuffix string   `json:"domain_suffix,omitempty"`
	}
	mcp.AddTool(s,
		&mcp.Tool{Name: "create_network", Description: "Create a new WireGuard network (admin only)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, p CreateNetworkParams) (*mcp.CallToolResult, any, error) {
			user := mcpUserFrom(ctx)
			if user == nil || user.Role != "administrator" {
				return mcpErr("admin access required"), nil, nil
			}
			net, err := h.service.CreateNetwork(ctx, &domain.NetworkCreateRequest{
				Name:         p.Name,
				CIDR:         p.CIDR,
				DNS:          p.DNS,
				DomainSuffix: p.DomainSuffix,
			})
			if err != nil {
				return mcpErr(err.Error()), nil, nil
			}
			return ok(net)
		},
	)

	type UpdateNetworkParams struct {
		NetworkID    string   `json:"network_id"`
		Name         string   `json:"name,omitempty"`
		DNS          []string `json:"dns,omitempty"`
		DomainSuffix string   `json:"domain_suffix,omitempty"`
	}
	mcp.AddTool(s,
		&mcp.Tool{Name: "update_network", Description: "Update a network (admin only)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, p UpdateNetworkParams) (*mcp.CallToolResult, any, error) {
			user := mcpUserFrom(ctx)
			if user == nil || user.Role != "administrator" {
				return mcpErr("admin access required"), nil, nil
			}
			net, err := h.service.UpdateNetwork(ctx, p.NetworkID, &domain.NetworkUpdateRequest{
				Name:         p.Name,
				DNS:          p.DNS,
				DomainSuffix: p.DomainSuffix,
			})
			if err != nil {
				return mcpErr(err.Error()), nil, nil
			}
			return ok(net)
		},
	)

	mcp.AddTool(s,
		&mcp.Tool{Name: "delete_network", Description: "Delete a network (admin only)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, p NetworkIDParams) (*mcp.CallToolResult, any, error) {
			user := mcpUserFrom(ctx)
			if user == nil || user.Role != "administrator" {
				return mcpErr("admin access required"), nil, nil
			}
			if err := h.service.DeleteNetwork(ctx, p.NetworkID); err != nil {
				return mcpErr(err.Error()), nil, nil
			}
			return ok(map[string]string{"status": "deleted"})
		},
	)

	// ── Peers ─────────────────────────────────────────────────────────────────
	mcp.AddTool(s,
		&mcp.Tool{Name: "list_peers", Description: "List all peers in a network."},
		func(ctx context.Context, _ *mcp.CallToolRequest, p NetworkIDParams) (*mcp.CallToolResult, any, error) {
			peers, err := h.service.ListPeers(ctx, p.NetworkID)
			if err != nil {
				return mcpErr(err.Error()), nil, nil
			}
			return ok(peers)
		},
	)

	type PeerParams struct {
		NetworkID string `json:"network_id"`
		PeerID    string `json:"peer_id"`
	}

	mcp.AddTool(s,
		&mcp.Tool{Name: "get_peer", Description: "Get details of a peer."},
		func(ctx context.Context, _ *mcp.CallToolRequest, p PeerParams) (*mcp.CallToolResult, any, error) {
			peer, err := h.service.GetPeer(ctx, p.NetworkID, p.PeerID)
			if err != nil {
				return mcpErr(err.Error()), nil, nil
			}
			return ok(peer)
		},
	)

	type CreatePeerParams struct {
		NetworkID string `json:"network_id"`
		Name      string `json:"name"`
		IsJump    bool   `json:"is_jump,omitempty"`
		UseAgent  bool   `json:"use_agent,omitempty"`
	}
	mcp.AddTool(s,
		&mcp.Tool{Name: "create_peer", Description: "Create a new peer in a network."},
		func(ctx context.Context, _ *mcp.CallToolRequest, p CreatePeerParams) (*mcp.CallToolResult, any, error) {
			user := mcpUserFrom(ctx)
			ownerID := ""
			if user != nil {
				ownerID = user.ID
			}
			peer, err := h.service.AddPeer(ctx, p.NetworkID, &domain.PeerCreateRequest{
				Name:     p.Name,
				IsJump:   p.IsJump,
				UseAgent: p.UseAgent,
			}, ownerID)
			if err != nil {
				return mcpErr(err.Error()), nil, nil
			}
			return ok(peer)
		},
	)

	type UpdatePeerParams struct {
		NetworkID string `json:"network_id"`
		PeerID    string `json:"peer_id"`
		Name      string `json:"name,omitempty"`
	}
	mcp.AddTool(s,
		&mcp.Tool{Name: "update_peer", Description: "Update a peer's name."},
		func(ctx context.Context, _ *mcp.CallToolRequest, p UpdatePeerParams) (*mcp.CallToolResult, any, error) {
			peer, err := h.service.UpdatePeer(ctx, p.NetworkID, p.PeerID, &domain.PeerUpdateRequest{
				Name: p.Name,
			})
			if err != nil {
				return mcpErr(err.Error()), nil, nil
			}
			return ok(peer)
		},
	)

	mcp.AddTool(s,
		&mcp.Tool{Name: "delete_peer", Description: "Delete a peer from a network."},
		func(ctx context.Context, _ *mcp.CallToolRequest, p PeerParams) (*mcp.CallToolResult, any, error) {
			if err := h.service.DeletePeer(ctx, p.NetworkID, p.PeerID); err != nil {
				return mcpErr(err.Error()), nil, nil
			}
			return ok(map[string]string{"status": "deleted"})
		},
	)

	mcp.AddTool(s,
		&mcp.Tool{Name: "get_peer_config", Description: "Get the WireGuard configuration file for a peer."},
		func(ctx context.Context, _ *mcp.CallToolRequest, p PeerParams) (*mcp.CallToolResult, any, error) {
			cfg, err := h.service.GeneratePeerConfig(ctx, p.NetworkID, p.PeerID)
			if err != nil {
				return mcpErr(err.Error()), nil, nil
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: cfg}},
			}, nil, nil
		},
	)

	// ── Groups (requires DB) ──────────────────────────────────────────────────
	if h.groupService != nil {
		mcp.AddTool(s,
			&mcp.Tool{Name: "list_groups", Description: "List groups in a network (requires DB)."},
			func(ctx context.Context, _ *mcp.CallToolRequest, p NetworkIDParams) (*mcp.CallToolResult, any, error) {
				groups, err := h.groupService.ListGroups(ctx, p.NetworkID)
				if err != nil {
					return mcpErr(err.Error()), nil, nil
				}
				return ok(groups)
			},
		)

		type CreateGroupParams struct {
			NetworkID   string `json:"network_id"`
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Priority    *int   `json:"priority,omitempty"`
		}
		mcp.AddTool(s,
			&mcp.Tool{Name: "create_group", Description: "Create a new group in a network (admin only)."},
			func(ctx context.Context, _ *mcp.CallToolRequest, p CreateGroupParams) (*mcp.CallToolResult, any, error) {
				user := mcpUserFrom(ctx)
				if user == nil || user.Role != "administrator" {
					return mcpErr("admin access required"), nil, nil
				}
				group, err := h.groupService.CreateGroup(ctx, p.NetworkID, &domain.GroupCreateRequest{
					Name:        p.Name,
					Description: p.Description,
					Priority:    p.Priority,
				})
				if err != nil {
					return mcpErr(err.Error()), nil, nil
				}
				return ok(group)
			},
		)

		type UpdateGroupParams struct {
			NetworkID   string `json:"network_id"`
			GroupID     string `json:"group_id"`
			Name        string `json:"name,omitempty"`
			Description string `json:"description,omitempty"`
			Priority    *int   `json:"priority,omitempty"`
		}
		mcp.AddTool(s,
			&mcp.Tool{Name: "update_group", Description: "Update a group's name, description, or priority (admin only)."},
			func(ctx context.Context, _ *mcp.CallToolRequest, p UpdateGroupParams) (*mcp.CallToolResult, any, error) {
				user := mcpUserFrom(ctx)
				if user == nil || user.Role != "administrator" {
					return mcpErr("admin access required"), nil, nil
				}
				group, err := h.groupService.UpdateGroup(ctx, p.NetworkID, p.GroupID, &domain.GroupUpdateRequest{
					Name:        p.Name,
					Description: p.Description,
					Priority:    p.Priority,
				})
				if err != nil {
					return mcpErr(err.Error()), nil, nil
				}
				return ok(group)
			},
		)
	}

	// ── Policies (requires DB) ────────────────────────────────────────────────
	if h.policyService != nil {
		mcp.AddTool(s,
			&mcp.Tool{Name: "list_policies", Description: "List policies in a network (requires DB)."},
			func(ctx context.Context, _ *mcp.CallToolRequest, p NetworkIDParams) (*mcp.CallToolResult, any, error) {
				policies, err := h.policyService.ListPolicies(ctx, p.NetworkID)
				if err != nil {
					return mcpErr(err.Error()), nil, nil
				}
				return ok(policies)
			},
		)

		type CreatePolicyParams struct {
			NetworkID   string              `json:"network_id"`
			Name        string              `json:"name"`
			Description string              `json:"description,omitempty"`
			Rules       []domain.PolicyRule `json:"rules,omitempty"`
		}
		mcp.AddTool(s,
			&mcp.Tool{Name: "create_policy", Description: "Create a new policy in a network (admin only). Rules have fields: direction, action, target, target_type, description."},
			func(ctx context.Context, _ *mcp.CallToolRequest, p CreatePolicyParams) (*mcp.CallToolResult, any, error) {
				user := mcpUserFrom(ctx)
				if user == nil || user.Role != "administrator" {
					return mcpErr("admin access required"), nil, nil
				}
				rules := make([]domain.PolicyRule, len(p.Rules))
				copy(rules, p.Rules)
				policy, err := h.policyService.CreatePolicy(ctx, p.NetworkID, &domain.PolicyCreateRequest{
					Name:        p.Name,
					Description: p.Description,
					Rules:       rules,
				})
				if err != nil {
					return mcpErr(err.Error()), nil, nil
				}
				return ok(policy)
			},
		)

		type UpdatePolicyParams struct {
			NetworkID   string `json:"network_id"`
			PolicyID    string `json:"policy_id"`
			Name        string `json:"name,omitempty"`
			Description string `json:"description,omitempty"`
		}
		mcp.AddTool(s,
			&mcp.Tool{Name: "update_policy", Description: "Update a policy's name or description (admin only)."},
			func(ctx context.Context, _ *mcp.CallToolRequest, p UpdatePolicyParams) (*mcp.CallToolResult, any, error) {
				user := mcpUserFrom(ctx)
				if user == nil || user.Role != "administrator" {
					return mcpErr("admin access required"), nil, nil
				}
				policy, err := h.policyService.UpdatePolicy(ctx, p.NetworkID, p.PolicyID, &domain.PolicyUpdateRequest{
					Name:        p.Name,
					Description: p.Description,
				})
				if err != nil {
					return mcpErr(err.Error()), nil, nil
				}
				return ok(policy)
			},
		)
	}

	// ── Routes (requires DB) ──────────────────────────────────────────────────
	if h.routeService != nil {
		mcp.AddTool(s,
			&mcp.Tool{Name: "list_routes", Description: "List routes in a network (requires DB)."},
			func(ctx context.Context, _ *mcp.CallToolRequest, p NetworkIDParams) (*mcp.CallToolResult, any, error) {
				routes, err := h.routeService.ListRoutes(ctx, p.NetworkID)
				if err != nil {
					return mcpErr(err.Error()), nil, nil
				}
				return ok(routes)
			},
		)

		type CreateRouteParams struct {
			NetworkID       string `json:"network_id"`
			Name            string `json:"name"`
			Description     string `json:"description,omitempty"`
			DestinationCIDR string `json:"destination_cidr"`
			JumpPeerID      string `json:"jump_peer_id"`
			DomainSuffix    string `json:"domain_suffix,omitempty"`
		}
		mcp.AddTool(s,
			&mcp.Tool{Name: "create_route", Description: "Create a new route in a network (admin only). Specifies a destination CIDR reachable via a jump peer."},
			func(ctx context.Context, _ *mcp.CallToolRequest, p CreateRouteParams) (*mcp.CallToolResult, any, error) {
				user := mcpUserFrom(ctx)
				if user == nil || user.Role != "administrator" {
					return mcpErr("admin access required"), nil, nil
				}
				route, err := h.routeService.CreateRoute(ctx, p.NetworkID, &domain.RouteCreateRequest{
					Name:            p.Name,
					Description:     p.Description,
					DestinationCIDR: p.DestinationCIDR,
					JumpPeerID:      p.JumpPeerID,
					DomainSuffix:    p.DomainSuffix,
				})
				if err != nil {
					return mcpErr(err.Error()), nil, nil
				}
				return ok(route)
			},
		)

		type UpdateRouteParams struct {
			NetworkID       string `json:"network_id"`
			RouteID         string `json:"route_id"`
			Name            string `json:"name,omitempty"`
			Description     string `json:"description,omitempty"`
			DestinationCIDR string `json:"destination_cidr,omitempty"`
			JumpPeerID      string `json:"jump_peer_id,omitempty"`
			DomainSuffix    string `json:"domain_suffix,omitempty"`
		}
		mcp.AddTool(s,
			&mcp.Tool{Name: "update_route", Description: "Update a route's configuration (admin only)."},
			func(ctx context.Context, _ *mcp.CallToolRequest, p UpdateRouteParams) (*mcp.CallToolResult, any, error) {
				user := mcpUserFrom(ctx)
				if user == nil || user.Role != "administrator" {
					return mcpErr("admin access required"), nil, nil
				}
				route, err := h.routeService.UpdateRoute(ctx, p.NetworkID, p.RouteID, &domain.RouteUpdateRequest{
					Name:            p.Name,
					Description:     p.Description,
					DestinationCIDR: p.DestinationCIDR,
					JumpPeerID:      p.JumpPeerID,
					DomainSuffix:    p.DomainSuffix,
				})
				if err != nil {
					return mcpErr(err.Error()), nil, nil
				}
				return ok(route)
			},
		)
	}

	// ── Security incidents ────────────────────────────────────────────────────
	mcp.AddTool(s,
		&mcp.Tool{Name: "list_incidents", Description: "List all security incidents."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
			incidents, err := h.service.ListSecurityIncidents(ctx, nil)
			if err != nil {
				return mcpErr(err.Error()), nil, nil
			}
			return ok(incidents)
		},
	)

	type IncidentParams struct {
		IncidentID string `json:"incident_id"`
	}

	mcp.AddTool(s,
		&mcp.Tool{Name: "get_incident", Description: "Get a security incident by ID."},
		func(ctx context.Context, _ *mcp.CallToolRequest, p IncidentParams) (*mcp.CallToolResult, any, error) {
			incident, err := h.service.GetSecurityIncident(ctx, p.IncidentID)
			if err != nil {
				return mcpErr(err.Error()), nil, nil
			}
			return ok(incident)
		},
	)

	mcp.AddTool(s,
		&mcp.Tool{Name: "resolve_incident", Description: "Mark a security incident as resolved."},
		func(ctx context.Context, _ *mcp.CallToolRequest, p IncidentParams) (*mcp.CallToolResult, any, error) {
			user := mcpUserFrom(ctx)
			resolvedBy := "mcp"
			if user != nil {
				resolvedBy = user.Email
			}
			if err := h.service.ResolveSecurityIncident(ctx, p.IncidentID, resolvedBy); err != nil {
				return mcpErr(err.Error()), nil, nil
			}
			return ok(map[string]string{"status": "resolved"})
		},
	)

	return s
}
