package api

import (
	"net/http"

	"wirety/internal/domain/network"

	"github.com/gin-gonic/gin"
)

// CreateGroup godoc
//
//	@Summary		Create a new group
//	@Description	Create a new group in a network (admin only)
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			networkId	path		string						true	"Network ID"
//	@Param			group		body		network.GroupCreateRequest	true	"Group creation request"
//	@Success		201			{object}	network.Group
//	@Failure		400			{object}	map[string]string
//	@Failure		403			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Router			/networks/{networkId}/groups [post]
//	@Security		BearerAuth
func (h *Handler) CreateGroup(c *gin.Context) {
	networkID := c.Param("networkId")

	var req network.GroupCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := h.groupService.CreateGroup(c.Request.Context(), networkID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, group)
}

// ListGroups godoc
//
//	@Summary		List groups
//	@Description	Get a list of all groups in a network (admin only)
//	@Tags			groups
//	@Produce		json
//	@Param			networkId	path		string	true	"Network ID"
//	@Success		200			{array}		network.Group
//	@Failure		403			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Router			/networks/{networkId}/groups [get]
//	@Security		BearerAuth
func (h *Handler) ListGroups(c *gin.Context) {
	networkID := c.Param("networkId")

	groups, err := h.groupService.ListGroups(c.Request.Context(), networkID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, groups)
}

// GetGroup godoc
//
//	@Summary		Get a group
//	@Description	Get a group by ID (admin only)
//	@Tags			groups
//	@Produce		json
//	@Param			networkId	path		string	true	"Network ID"
//	@Param			groupId		path		string	true	"Group ID"
//	@Success		200			{object}	network.Group
//	@Failure		403			{object}	map[string]string
//	@Failure		404			{object}	map[string]string
//	@Router			/networks/{networkId}/groups/{groupId} [get]
//	@Security		BearerAuth
func (h *Handler) GetGroup(c *gin.Context) {
	networkID := c.Param("networkId")
	groupID := c.Param("groupId")

	group, err := h.groupService.GetGroup(c.Request.Context(), networkID, groupID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, group)
}

// UpdateGroup godoc
//
//	@Summary		Update a group
//	@Description	Update a group's configuration (admin only)
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			networkId	path		string						true	"Network ID"
//	@Param			groupId		path		string						true	"Group ID"
//	@Param			group		body		network.GroupUpdateRequest	true	"Group update request"
//	@Success		200			{object}	network.Group
//	@Failure		400			{object}	map[string]string
//	@Failure		403			{object}	map[string]string
//	@Failure		404			{object}	map[string]string
//	@Router			/networks/{networkId}/groups/{groupId} [put]
//	@Security		BearerAuth
func (h *Handler) UpdateGroup(c *gin.Context) {
	networkID := c.Param("networkId")
	groupID := c.Param("groupId")

	var req network.GroupUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := h.groupService.UpdateGroup(c.Request.Context(), networkID, groupID, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, group)
}

// DeleteGroup godoc
//
//	@Summary		Delete a group
//	@Description	Delete a group by ID (admin only)
//	@Tags			groups
//	@Param			networkId	path	string	true	"Network ID"
//	@Param			groupId		path	string	true	"Group ID"
//	@Success		204
//	@Failure		403	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Router			/networks/{networkId}/groups/{groupId} [delete]
//	@Security		BearerAuth
func (h *Handler) DeleteGroup(c *gin.Context) {
	networkID := c.Param("networkId")
	groupID := c.Param("groupId")

	if err := h.groupService.DeleteGroup(c.Request.Context(), networkID, groupID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// AddPeerToGroup godoc
//
//	@Summary		Add peer to group
//	@Description	Add a peer to a group (admin only)
//	@Tags			groups
//	@Param			networkId	path	string	true	"Network ID"
//	@Param			groupId		path	string	true	"Group ID"
//	@Param			peerId		path	string	true	"Peer ID"
//	@Success		200
//	@Failure		403	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Router			/networks/{networkId}/groups/{groupId}/peers/{peerId} [post]
//	@Security		BearerAuth
func (h *Handler) AddPeerToGroup(c *gin.Context) {
	networkID := c.Param("networkId")
	groupID := c.Param("groupId")
	peerID := c.Param("peerId")

	if err := h.groupService.AddPeerToGroup(c.Request.Context(), networkID, groupID, peerID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

// RemovePeerFromGroup godoc
//
//	@Summary		Remove peer from group
//	@Description	Remove a peer from a group (admin only)
//	@Tags			groups
//	@Param			networkId	path	string	true	"Network ID"
//	@Param			groupId		path	string	true	"Group ID"
//	@Param			peerId		path	string	true	"Peer ID"
//	@Success		204
//	@Failure		403	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Router			/networks/{networkId}/groups/{groupId}/peers/{peerId} [delete]
//	@Security		BearerAuth
func (h *Handler) RemovePeerFromGroup(c *gin.Context) {
	networkID := c.Param("networkId")
	groupID := c.Param("groupId")
	peerID := c.Param("peerId")

	if err := h.groupService.RemovePeerFromGroup(c.Request.Context(), networkID, groupID, peerID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
