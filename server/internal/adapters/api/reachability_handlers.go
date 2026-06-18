package api

import (
	"net/http"
	"sort"

	"wirety/internal/adapters/api/middleware"
	domain "wirety/internal/domain/network"

	"github.com/gin-gonic/gin"
)

// PeerReachabilityResponse describes the complete reachability picture for a peer.
type PeerReachabilityResponse struct {
	PeerID      string        `json:"peer_id"`
	PeerName    string        `json:"peer_name"`
	PeerAddress string        `json:"peer_address"`
	IsJump      bool          `json:"is_jump"`
	PeerAccess  []PeerAccess  `json:"peer_access"`
	Rules       []RuleAccess  `json:"rules"`
	Routes      []RouteAccess `json:"routes"`
}

// PeerAccess describes whether this peer can communicate with another peer (ACL layer).
type PeerAccess struct {
	PeerID   string `json:"peer_id"`
	PeerName string `json:"peer_name"`
	Address  string `json:"address"`
	IsJump   bool   `json:"is_jump"`
	Allowed  bool   `json:"allowed"`
	// Reason: "acl_disabled" | "blocked" | "deny_rule" | "allow_rule" | "default_allow"
	Reason string `json:"reason"`
}

// RuleAccess describes one effective policy rule for this peer, with resolved target addresses.
type RuleAccess struct {
	Direction   string   `json:"direction"`  // "output" | "input"
	Action      string   `json:"action"`     // "allow" | "deny"
	TargetType  string   `json:"target_type"` // "cidr" | "peer" | "group"
	Target      string   `json:"target"`      // original value from the rule
	Addresses   []string `json:"addresses"`   // resolved IP/CIDR list
	PolicyName  string   `json:"policy_name"`
	GroupName   string   `json:"group_name"`
	Description string   `json:"description,omitempty"`
}

// RouteAccess describes an external network route accessible by this peer.
// Dual-stack routes carry both DestinationCIDR (IPv4) and DestinationCIDRv6.
type RouteAccess struct {
	RouteID           string `json:"route_id"`
	RouteName         string `json:"route_name"`
	DestinationCIDR   string `json:"destination_cidr,omitempty"`
	DestinationCIDRv6 string `json:"destination_cidr_v6,omitempty"`
	JumpPeerID        string `json:"jump_peer_id"`
	JumpPeerName      string `json:"jump_peer_name"`
	GroupName         string `json:"group_name"`
}

// GetPeerReachability godoc
// @Summary      Compute peer reachability
// @Description  Returns which peers, CIDRs, and external routes are reachable from a peer based on ACL and policy rules
// @Tags         peers
// @Produce      json
// @Param        networkId path string true "Network ID"
// @Param        peerId    path string true "Peer ID"
// @Success      200 {object} PeerReachabilityResponse
// @Failure      404 {object} map[string]string
// @Router       /networks/{networkId}/peers/{peerId}/reachability [get]
// @Security     BearerAuth
func (h *Handler) GetPeerReachability(c *gin.Context) {
	networkID := c.Param("networkId")
	peerID := c.Param("peerId")
	ctx := c.Request.Context()
	user := middleware.GetUserFromContext(c)

	// 1. Get target peer
	peer, err := h.service.GetPeer(ctx, networkID, peerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "peer not found"})
		return
	}

	// Object-level authz: reachability enumerates every peer in the network
	// plus the full ACL/policy/route topology, so it is restricted to the
	// peer's owner or an admin. Unlike connectivity status there is no
	// jump-peer exception — querying a jump peer would still leak the whole
	// network map to any member.
	if user != nil && !user.IsAdministrator() && peer.OwnerID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "you can only view your own peers"})
		return
	}

	// 2. Get all peers in the network for name/address resolution
	allPeers, err := h.service.ListPeers(ctx, networkID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	peerByID := make(map[string]*domain.Peer, len(allPeers))
	for _, p := range allPeers {
		peerByID[p.ID] = p
	}

	// 3. Get ACL for the network (GetACL returns interface{} — type-assert to *domain.ACL)
	acl := &domain.ACL{Enabled: false} // default: no ACL restrictions
	if aclRaw, err := h.service.GetACL(ctx, networkID); err == nil {
		if a, ok := aclRaw.(*domain.ACL); ok {
			acl = a
		}
	}

	// 4. Compute peer access (ACL layer)
	var peerAccess []PeerAccess
	for _, p := range allPeers {
		if p.ID == peerID {
			continue
		}
		allowed, reason := aclAccess(acl, peerID, p.ID)
		peerAccess = append(peerAccess, PeerAccess{
			PeerID:   p.ID,
			PeerName: p.Name,
			Address:  p.Address,
			IsJump:   p.IsJump,
			Allowed:  allowed,
			Reason:   reason,
		})
	}
	// Stable sort: allowed first, then by name
	sort.SliceStable(peerAccess, func(i, j int) bool {
		if peerAccess[i].Allowed != peerAccess[j].Allowed {
			return peerAccess[i].Allowed
		}
		return peerAccess[i].PeerName < peerAccess[j].PeerName
	})

	// 5. Compute rule and route access from groups/policies (requires DB services)
	var rules []RuleAccess
	var routes []RouteAccess

	if h.groupService != nil && h.policyService != nil {
		// Collect and sort peer's groups by priority (lower = higher priority)
		var peerGroups []*domain.Group
		for _, gid := range peer.GroupIDs {
			g, err := h.groupService.GetGroup(ctx, networkID, gid)
			if err == nil {
				peerGroups = append(peerGroups, g)
			}
		}
		sort.SliceStable(peerGroups, func(i, j int) bool {
			return peerGroups[i].Priority < peerGroups[j].Priority
		})

		// Deduplicate policies across groups (first group with higher priority wins)
		type policyEntry struct {
			policy    *domain.Policy
			groupName string
		}
		seenPolicy := map[string]bool{}
		var effective []policyEntry

		for _, group := range peerGroups {
			policies, err := h.groupService.GetGroupPolicies(ctx, networkID, group.ID)
			if err != nil {
				continue
			}
			for _, pol := range policies {
				if seenPolicy[pol.ID] {
					continue
				}
				seenPolicy[pol.ID] = true
				effective = append(effective, policyEntry{pol, group.Name})
			}
		}

		// Build rule access list, resolving peer/group targets to IP addresses
		for _, e := range effective {
			for _, rule := range e.policy.Rules {
				addrs := resolveTarget(rule.TargetType, rule.Target, allPeers, peerByID)
				rules = append(rules, RuleAccess{
					Direction:   rule.Direction,
					Action:      rule.Action,
					TargetType:  rule.TargetType,
					Target:      rule.Target,
					Addresses:   addrs,
					PolicyName:  e.policy.Name,
					GroupName:   e.groupName,
					Description: rule.Description,
				})
			}
		}

		// Collect routes from peer's groups (requires routeService)
		if h.routeService != nil {
			seenRoute := map[string]bool{}
			for _, group := range peerGroups {
				for _, routeID := range group.RouteIDs {
					if seenRoute[routeID] {
						continue
					}
					seenRoute[routeID] = true
					route, err := h.routeService.GetRoute(ctx, networkID, routeID)
					if err != nil {
						continue
					}
					jumpName := route.JumpPeerID
					if jp, ok := peerByID[route.JumpPeerID]; ok {
						jumpName = jp.Name
					}
					routes = append(routes, RouteAccess{
						RouteID:           route.ID,
						RouteName:         route.Name,
						DestinationCIDR:   route.DestinationCIDR,
						DestinationCIDRv6: route.DestinationCIDRv6,
						JumpPeerID:        route.JumpPeerID,
						JumpPeerName:      jumpName,
						GroupName:         group.Name,
					})
				}
			}
		}
	}

	c.JSON(http.StatusOK, PeerReachabilityResponse{
		PeerID:      peer.ID,
		PeerName:    peer.Name,
		PeerAddress: peer.Address,
		IsJump:      peer.IsJump,
		PeerAccess:  peerAccess,
		Rules:       rules,
		Routes:      routes,
	})
}

// aclAccess returns (allowed, reason) for sourcePeer→targetPeer based on ACL.
func aclAccess(acl *domain.ACL, srcID, dstID string) (bool, string) {
	if !acl.Enabled {
		return true, "acl_disabled"
	}
	if acl.BlockedPeers[srcID] || acl.BlockedPeers[dstID] {
		return false, "blocked"
	}
	for _, rule := range acl.Rules {
		srcMatch := rule.SourcePeer == "*" || rule.SourcePeer == srcID
		dstMatch := rule.TargetPeer == "*" || rule.TargetPeer == dstID
		if srcMatch && dstMatch {
			if rule.Action == "allow" {
				return true, "allow_rule"
			}
			return false, "deny_rule"
		}
	}
	return true, "default_allow"
}

// resolveTarget converts a policy rule target to a list of IP/CIDR strings.
func resolveTarget(targetType, target string, allPeers []*domain.Peer, peerByID map[string]*domain.Peer) []string {
	switch targetType {
	case "cidr":
		return []string{target}
	case "peer":
		if p, ok := peerByID[target]; ok {
			return []string{p.Address}
		}
		return []string{target}
	case "group":
		// Collect addresses of all peers that belong to this group
		var addrs []string
		for _, p := range allPeers {
			for _, gid := range p.GroupIDs {
				if gid == target {
					addrs = append(addrs, p.Address)
					break
				}
			}
		}
		return addrs
	}
	return []string{target}
}
