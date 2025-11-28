package api

import (
	"net/http"

	"wirety/internal/domain/network"

	"github.com/gin-gonic/gin"
)

// CreateDNSMapping godoc
//
//	@Summary		Create a new DNS mapping
//	@Description	Create a new DNS mapping for a route (admin only)
//	@Tags			dns
//	@Accept			json
//	@Produce		json
//	@Param			networkId	path		string							true	"Network ID"
//	@Param			routeId		path		string							true	"Route ID"
//	@Param			dns			body		network.DNSMappingCreateRequest	true	"DNS mapping creation request"
//	@Success		201			{object}	network.DNSMapping
//	@Failure		400			{object}	map[string]string
//	@Failure		403			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Router			/networks/{networkId}/routes/{routeId}/dns [post]
//	@Security		BearerAuth
func (h *Handler) CreateDNSMapping(c *gin.Context) {
	routeID := c.Param("routeId")

	var req network.DNSMappingCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mapping, err := h.dnsService.CreateDNSMapping(c.Request.Context(), routeID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, mapping)
}

// ListDNSMappings godoc
//
//	@Summary		List DNS mappings
//	@Description	Get a list of all DNS mappings for a route (admin only)
//	@Tags			dns
//	@Produce		json
//	@Param			networkId	path		string	true	"Network ID"
//	@Param			routeId		path		string	true	"Route ID"
//	@Success		200			{array}		network.DNSMapping
//	@Failure		403			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Router			/networks/{networkId}/routes/{routeId}/dns [get]
//	@Security		BearerAuth
func (h *Handler) ListDNSMappings(c *gin.Context) {
	routeID := c.Param("routeId")

	mappings, err := h.dnsService.ListDNSMappings(c.Request.Context(), routeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, mappings)
}

// UpdateDNSMapping godoc
//
//	@Summary		Update a DNS mapping
//	@Description	Update a DNS mapping's configuration (admin only)
//	@Tags			dns
//	@Accept			json
//	@Produce		json
//	@Param			networkId	path		string							true	"Network ID"
//	@Param			routeId		path		string							true	"Route ID"
//	@Param			dnsId		path		string							true	"DNS Mapping ID"
//	@Param			dns			body		network.DNSMappingUpdateRequest	true	"DNS mapping update request"
//	@Success		200			{object}	network.DNSMapping
//	@Failure		400			{object}	map[string]string
//	@Failure		403			{object}	map[string]string
//	@Failure		404			{object}	map[string]string
//	@Router			/networks/{networkId}/routes/{routeId}/dns/{dnsId} [put]
//	@Security		BearerAuth
func (h *Handler) UpdateDNSMapping(c *gin.Context) {
	routeID := c.Param("routeId")
	dnsID := c.Param("dnsId")

	var req network.DNSMappingUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mapping, err := h.dnsService.UpdateDNSMapping(c.Request.Context(), routeID, dnsID, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, mapping)
}

// DeleteDNSMapping godoc
//
//	@Summary		Delete a DNS mapping
//	@Description	Delete a DNS mapping by ID (admin only)
//	@Tags			dns
//	@Param			networkId	path	string	true	"Network ID"
//	@Param			routeId		path	string	true	"Route ID"
//	@Param			dnsId		path	string	true	"DNS Mapping ID"
//	@Success		204
//	@Failure		403	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Router			/networks/{networkId}/routes/{routeId}/dns/{dnsId} [delete]
//	@Security		BearerAuth
func (h *Handler) DeleteDNSMapping(c *gin.Context) {
	routeID := c.Param("routeId")
	dnsID := c.Param("dnsId")

	if err := h.dnsService.DeleteDNSMapping(c.Request.Context(), routeID, dnsID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetNetworkDNSRecords godoc
//
//	@Summary		Get network DNS records
//	@Description	Get all DNS records for a network including peer and route mappings (admin only)
//	@Tags			dns
//	@Produce		json
//	@Param			networkId	path		string	true	"Network ID"
//	@Success		200			{array}		map[string]any
//	@Failure		403			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Router			/networks/{networkId}/dns [get]
//	@Security		BearerAuth
func (h *Handler) GetNetworkDNSRecords(c *gin.Context) {
	networkID := c.Param("networkId")

	records, err := h.dnsService.GetNetworkDNSRecords(c.Request.Context(), networkID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, records)
}
