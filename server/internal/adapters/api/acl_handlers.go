package api

import (
	"net/http"

	"wirety/internal/audit"
	domain "wirety/internal/domain/network"

	"github.com/gin-gonic/gin"
)

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

	h.wsManager.NotifyNetworkPeers(networkID)

	id, email := actor(c)
	audit.Server(id, email, c.ClientIP()).
		Str("action", "acl.update").
		Str("network_id", networkID).
		Msg("audit")

	c.JSON(http.StatusOK, acl)
}
