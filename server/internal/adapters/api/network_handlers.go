package api

import (
	"net/http"
	"strconv"

	"wirety/internal/adapters/api/middleware"
	"wirety/internal/audit"
	domain "wirety/internal/domain/network"

	"github.com/gin-gonic/gin"
)

// PaginatedNetworks represents a paginated list of networks
type PaginatedNetworks struct {
	Data     []*domain.Network `json:"data"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
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
		if isValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	id, email := actor(c)
	audit.Server(id, email, c.ClientIP()).
		Str("action", "network.create").
		Str("network_id", net.ID).
		Str("network_name", net.Name).
		Msg("audit")

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

	var hasAccess []*domain.Network
	for _, n := range networks {
		if user.HasNetworkAccess(n.ID) {
			hasAccess = append(hasAccess, n)
		}
	}
	networks = hasAccess

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
		if isValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		}
		return
	}

	id, email := actor(c)
	audit.Server(id, email, c.ClientIP()).
		Str("action", "network.update").
		Str("network_id", networkID).
		Str("network_name", net.Name).
		Msg("audit")

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
//
// @Security     BearerAuth
func (h *Handler) DeleteNetwork(c *gin.Context) {
	networkID := c.Param("networkId")

	if err := h.service.DeleteNetwork(c.Request.Context(), networkID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	id, email := actor(c)
	audit.Server(id, email, c.ClientIP()).
		Str("action", "network.delete").
		Str("network_id", networkID).
		Msg("audit")

	c.Status(http.StatusNoContent)
}
