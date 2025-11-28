package api

import (
	"net/http"

	"wirety/internal/domain/network"

	"github.com/gin-gonic/gin"
)

// CreatePolicy godoc
//
//	@Summary		Create a new policy
//	@Description	Create a new policy in a network (admin only)
//	@Tags			policies
//	@Accept			json
//	@Produce		json
//	@Param			networkId	path		string						true	"Network ID"
//	@Param			policy		body		network.PolicyCreateRequest	true	"Policy creation request"
//	@Success		201			{object}	network.Policy
//	@Failure		400			{object}	map[string]string
//	@Failure		403			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Router			/networks/{networkId}/policies [post]
//	@Security		BearerAuth
func (h *Handler) CreatePolicy(c *gin.Context) {
	networkID := c.Param("networkId")

	var req network.PolicyCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy, err := h.policyService.CreatePolicy(c.Request.Context(), networkID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, policy)
}

// ListPolicies godoc
//
//	@Summary		List policies
//	@Description	Get a list of all policies in a network (admin only)
//	@Tags			policies
//	@Produce		json
//	@Param			networkId	path		string	true	"Network ID"
//	@Success		200			{array}		network.Policy
//	@Failure		403			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Router			/networks/{networkId}/policies [get]
//	@Security		BearerAuth
func (h *Handler) ListPolicies(c *gin.Context) {
	networkID := c.Param("networkId")

	policies, err := h.policyService.ListPolicies(c.Request.Context(), networkID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policies)
}

// GetPolicy godoc
//
//	@Summary		Get a policy
//	@Description	Get a policy by ID (admin only)
//	@Tags			policies
//	@Produce		json
//	@Param			networkId	path		string	true	"Network ID"
//	@Param			policyId	path		string	true	"Policy ID"
//	@Success		200			{object}	network.Policy
//	@Failure		403			{object}	map[string]string
//	@Failure		404			{object}	map[string]string
//	@Router			/networks/{networkId}/policies/{policyId} [get]
//	@Security		BearerAuth
func (h *Handler) GetPolicy(c *gin.Context) {
	networkID := c.Param("networkId")
	policyID := c.Param("policyId")

	policy, err := h.policyService.GetPolicy(c.Request.Context(), networkID, policyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policy)
}

// UpdatePolicy godoc
//
//	@Summary		Update a policy
//	@Description	Update a policy's configuration (admin only)
//	@Tags			policies
//	@Accept			json
//	@Produce		json
//	@Param			networkId	path		string						true	"Network ID"
//	@Param			policyId	path		string						true	"Policy ID"
//	@Param			policy		body		network.PolicyUpdateRequest	true	"Policy update request"
//	@Success		200			{object}	network.Policy
//	@Failure		400			{object}	map[string]string
//	@Failure		403			{object}	map[string]string
//	@Failure		404			{object}	map[string]string
//	@Router			/networks/{networkId}/policies/{policyId} [put]
//	@Security		BearerAuth
func (h *Handler) UpdatePolicy(c *gin.Context) {
	networkID := c.Param("networkId")
	policyID := c.Param("policyId")

	var req network.PolicyUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy, err := h.policyService.UpdatePolicy(c.Request.Context(), networkID, policyID, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policy)
}

// DeletePolicy godoc
//
//	@Summary		Delete a policy
//	@Description	Delete a policy by ID (admin only)
//	@Tags			policies
//	@Param			networkId	path	string	true	"Network ID"
//	@Param			policyId	path	string	true	"Policy ID"
//	@Success		204
//	@Failure		403	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Router			/networks/{networkId}/policies/{policyId} [delete]
//	@Security		BearerAuth
func (h *Handler) DeletePolicy(c *gin.Context) {
	networkID := c.Param("networkId")
	policyID := c.Param("policyId")

	if err := h.policyService.DeletePolicy(c.Request.Context(), networkID, policyID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// AddRuleToPolicy godoc
//
//	@Summary		Add rule to policy
//	@Description	Add a rule to a policy (admin only)
//	@Tags			policies
//	@Accept			json
//	@Produce		json
//	@Param			networkId	path		string				true	"Network ID"
//	@Param			policyId	path		string				true	"Policy ID"
//	@Param			rule		body		network.PolicyRule	true	"Policy rule"
//	@Success		201			{object}	network.PolicyRule
//	@Failure		400			{object}	map[string]string
//	@Failure		403			{object}	map[string]string
//	@Failure		404			{object}	map[string]string
//	@Router			/networks/{networkId}/policies/{policyId}/rules [post]
//	@Security		BearerAuth
func (h *Handler) AddRuleToPolicy(c *gin.Context) {
	networkID := c.Param("networkId")
	policyID := c.Param("policyId")

	var rule network.PolicyRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.policyService.AddRuleToPolicy(c.Request.Context(), networkID, policyID, &rule); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rule)
}

// RemoveRuleFromPolicy godoc
//
//	@Summary		Remove rule from policy
//	@Description	Remove a rule from a policy (admin only)
//	@Tags			policies
//	@Param			networkId	path	string	true	"Network ID"
//	@Param			policyId	path	string	true	"Policy ID"
//	@Param			ruleId		path	string	true	"Rule ID"
//	@Success		204
//	@Failure		403	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Router			/networks/{networkId}/policies/{policyId}/rules/{ruleId} [delete]
//	@Security		BearerAuth
func (h *Handler) RemoveRuleFromPolicy(c *gin.Context) {
	networkID := c.Param("networkId")
	policyID := c.Param("policyId")
	ruleID := c.Param("ruleId")

	if err := h.policyService.RemoveRuleFromPolicy(c.Request.Context(), networkID, policyID, ruleID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetDefaultTemplates godoc
//
//	@Summary		Get default policy templates
//	@Description	Get predefined policy templates (admin only)
//	@Tags			policies
//	@Produce		json
//	@Param			networkId	path		string	true	"Network ID"
//	@Success		200			{array}		PolicyTemplate
//	@Failure		403			{object}	map[string]string
//	@Router			/networks/{networkId}/policies/templates [get]
//	@Security		BearerAuth
func (h *Handler) GetDefaultTemplates(c *gin.Context) {
	templates := h.policyService.GetDefaultTemplates()
	c.JSON(http.StatusOK, templates)
}

// AttachPolicyToGroup godoc
//
//	@Summary		Attach policy to group
//	@Description	Attach a policy to a group (admin only)
//	@Tags			policies
//	@Param			networkId	path	string	true	"Network ID"
//	@Param			groupId		path	string	true	"Group ID"
//	@Param			policyId	path	string	true	"Policy ID"
//	@Success		200
//	@Failure		403	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Router			/networks/{networkId}/groups/{groupId}/policies/{policyId} [post]
//	@Security		BearerAuth
func (h *Handler) AttachPolicyToGroup(c *gin.Context) {
	networkID := c.Param("networkId")
	groupID := c.Param("groupId")
	policyID := c.Param("policyId")

	if err := h.groupService.AttachPolicyToGroup(c.Request.Context(), networkID, groupID, policyID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

// DetachPolicyFromGroup godoc
//
//	@Summary		Detach policy from group
//	@Description	Detach a policy from a group (admin only)
//	@Tags			policies
//	@Param			networkId	path	string	true	"Network ID"
//	@Param			groupId		path	string	true	"Group ID"
//	@Param			policyId	path	string	true	"Policy ID"
//	@Success		204
//	@Failure		403	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Router			/networks/{networkId}/groups/{groupId}/policies/{policyId} [delete]
//	@Security		BearerAuth
func (h *Handler) DetachPolicyFromGroup(c *gin.Context) {
	networkID := c.Param("networkId")
	groupID := c.Param("groupId")
	policyID := c.Param("policyId")

	if err := h.groupService.DetachPolicyFromGroup(c.Request.Context(), networkID, groupID, policyID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetGroupPolicies godoc
//
//	@Summary		Get group policies
//	@Description	Get all policies attached to a group (admin only)
//	@Tags			policies
//	@Produce		json
//	@Param			networkId	path		string	true	"Network ID"
//	@Param			groupId		path		string	true	"Group ID"
//	@Success		200			{array}		network.Policy
//	@Failure		403			{object}	map[string]string
//	@Failure		404			{object}	map[string]string
//	@Router			/networks/{networkId}/groups/{groupId}/policies [get]
//	@Security		BearerAuth
func (h *Handler) GetGroupPolicies(c *gin.Context) {
	networkID := c.Param("networkId")
	groupID := c.Param("groupId")

	policies, err := h.groupService.GetGroupPolicies(c.Request.Context(), networkID, groupID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policies)
}
