package api

import (
	"net/http"

	"wirety/internal/domain/network"

	"github.com/gin-gonic/gin"
)

// CreateRoute godoc
//
//	@Summary		Create a new route
//	@Description	Create a new route in a network (admin only)
//	@Tags			routes
//	@Accept			json
//	@Produce		json
//	@Param			networkId	path		string						true	"Network ID"
//	@Param			route		body		network.RouteCreateRequest	true	"Route creation request"
//	@Success		201			{object}	network.Route
//	@Failure		400			{object}	map[string]string
//	@Failure		403			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Router			/networks/{networkId}/routes [post]
//	@Security		BearerAuth
func (h *Handler) CreateRoute(c *gin.Context) {
	networkID := c.Param("networkId")

	var req network.RouteCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	route, err := h.routeService.CreateRoute(c.Request.Context(), networkID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, route)
}

// ListRoutes godoc
//
//	@Summary		List routes
//	@Description	Get a list of all routes in a network (admin only)
//	@Tags			routes
//	@Produce		json
//	@Param			networkId	path		string	true	"Network ID"
//	@Success		200			{array}		network.Route
//	@Failure		403			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Router			/networks/{networkId}/routes [get]
//	@Security		BearerAuth
func (h *Handler) ListRoutes(c *gin.Context) {
	networkID := c.Param("networkId")

	routes, err := h.routeService.ListRoutes(c.Request.Context(), networkID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, routes)
}

// GetRoute godoc
//
//	@Summary		Get a route
//	@Description	Get a route by ID (admin only)
//	@Tags			routes
//	@Produce		json
//	@Param			networkId	path		string	true	"Network ID"
//	@Param			routeId		path		string	true	"Route ID"
//	@Success		200			{object}	network.Route
//	@Failure		403			{object}	map[string]string
//	@Failure		404			{object}	map[string]string
//	@Router			/networks/{networkId}/routes/{routeId} [get]
//	@Security		BearerAuth
func (h *Handler) GetRoute(c *gin.Context) {
	networkID := c.Param("networkId")
	routeID := c.Param("routeId")

	route, err := h.routeService.GetRoute(c.Request.Context(), networkID, routeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, route)
}

// UpdateRoute godoc
//
//	@Summary		Update a route
//	@Description	Update a route's configuration (admin only)
//	@Tags			routes
//	@Accept			json
//	@Produce		json
//	@Param			networkId	path		string						true	"Network ID"
//	@Param			routeId		path		string						true	"Route ID"
//	@Param			route		body		network.RouteUpdateRequest	true	"Route update request"
//	@Success		200			{object}	network.Route
//	@Failure		400			{object}	map[string]string
//	@Failure		403			{object}	map[string]string
//	@Failure		404			{object}	map[string]string
//	@Router			/networks/{networkId}/routes/{routeId} [put]
//	@Security		BearerAuth
func (h *Handler) UpdateRoute(c *gin.Context) {
	networkID := c.Param("networkId")
	routeID := c.Param("routeId")

	var req network.RouteUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	route, err := h.routeService.UpdateRoute(c.Request.Context(), networkID, routeID, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, route)
}

// DeleteRoute godoc
//
//	@Summary		Delete a route
//	@Description	Delete a route by ID (admin only)
//	@Tags			routes
//	@Param			networkId	path	string	true	"Network ID"
//	@Param			routeId		path	string	true	"Route ID"
//	@Success		204
//	@Failure		403	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Router			/networks/{networkId}/routes/{routeId} [delete]
//	@Security		BearerAuth
func (h *Handler) DeleteRoute(c *gin.Context) {
	networkID := c.Param("networkId")
	routeID := c.Param("routeId")

	if err := h.routeService.DeleteRoute(c.Request.Context(), networkID, routeID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// AttachRouteToGroup godoc
//
//	@Summary		Attach route to group
//	@Description	Attach a route to a group (admin only)
//	@Tags			routes
//	@Param			networkId	path	string	true	"Network ID"
//	@Param			groupId		path	string	true	"Group ID"
//	@Param			routeId		path	string	true	"Route ID"
//	@Success		200
//	@Failure		403	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Router			/networks/{networkId}/groups/{groupId}/routes/{routeId} [post]
//	@Security		BearerAuth
func (h *Handler) AttachRouteToGroup(c *gin.Context) {
	networkID := c.Param("networkId")
	groupID := c.Param("groupId")
	routeID := c.Param("routeId")

	if err := h.groupService.AttachRouteToGroup(c.Request.Context(), networkID, groupID, routeID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

// DetachRouteFromGroup godoc
//
//	@Summary		Detach route from group
//	@Description	Detach a route from a group (admin only)
//	@Tags			routes
//	@Param			networkId	path	string	true	"Network ID"
//	@Param			groupId		path	string	true	"Group ID"
//	@Param			routeId		path	string	true	"Route ID"
//	@Success		204
//	@Failure		403	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Router			/networks/{networkId}/groups/{groupId}/routes/{routeId} [delete]
//	@Security		BearerAuth
func (h *Handler) DetachRouteFromGroup(c *gin.Context) {
	networkID := c.Param("networkId")
	groupID := c.Param("groupId")
	routeID := c.Param("routeId")

	if err := h.groupService.DetachRouteFromGroup(c.Request.Context(), networkID, groupID, routeID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetGroupRoutes godoc
//
//	@Summary		Get group routes
//	@Description	Get all routes attached to a group (admin only)
//	@Tags			routes
//	@Produce		json
//	@Param			networkId	path		string	true	"Network ID"
//	@Param			groupId		path		string	true	"Group ID"
//	@Success		200			{array}		network.Route
//	@Failure		403			{object}	map[string]string
//	@Failure		404			{object}	map[string]string
//	@Router			/networks/{networkId}/groups/{groupId}/routes [get]
//	@Security		BearerAuth
func (h *Handler) GetGroupRoutes(c *gin.Context) {
	networkID := c.Param("networkId")
	groupID := c.Param("groupId")

	routes, err := h.groupRepo.GetGroupRoutes(c.Request.Context(), networkID, groupID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, routes)
}
