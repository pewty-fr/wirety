package api

import (
	"net/http"
	"strconv"

	"wirety/internal/adapters/api/middleware"

	"github.com/gin-gonic/gin"
)

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
	const maxCount = 20
	countStr := c.DefaultQuery("count", "1")
	count, err := strconv.Atoi(countStr)
	if err != nil || count <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "count must be a positive integer"})
		return
	}
	if count > maxCount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "count must not exceed 20"})
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
		if user != nil && !user.HasNetworkAccess(net.ID) {
			continue
		}

		peers, err := h.service.ListPeers(c.Request.Context(), net.ID)
		if err != nil {
			continue
		}

		for _, peer := range peers {
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

			if filter != "" {
				if !contains(allocation.NetworkName, filter) &&
					!contains(allocation.IP, filter) &&
					!contains(allocation.PeerName, filter) {
					continue
				}
			}

			allAllocations = append(allAllocations, allocation)
		}
	}

	total := len(allAllocations)
	start := (page - 1) * pageSize
	end := start + pageSize

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      allAllocations[start:end],
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
