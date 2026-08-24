package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// The legacy ACL system has been removed in favour of policy-based access
// control (groups + policies). The routes are kept so old clients get an
// explicit "gone" answer instead of a confusing 404, but the handlers no
// longer call into the service layer — the former service stubs
// unconditionally returned an error, which made every call site a
// statically-provable dead branch (staticcheck SA4023).

const aclRemovedMsg = "ACL system has been removed - use policy-based access control instead"

// GetACL godoc
//
//	@Summary		Get ACL configuration (removed)
//	@Description	The legacy ACL system has been removed — use policy-based access control instead
//	@Tags			acl
//	@Produce		json
//	@Param			networkId	path		string	true	"Network ID"
//	@Failure		410			{object}	map[string]string
//	@Router			/networks/{networkId}/acl [get]
//
// @Security     BearerAuth
func (h *Handler) GetACL(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{"error": aclRemovedMsg})
}

// UpdateACL godoc
//
//	@Summary		Update ACL configuration (removed)
//	@Description	The legacy ACL system has been removed — use policy-based access control instead
//	@Tags			acl
//	@Accept			json
//	@Produce		json
//	@Param			networkId	path		string	true	"Network ID"
//	@Failure		410			{object}	map[string]string
//	@Router			/networks/{networkId}/acl [put]
//
// @Security     BearerAuth
func (h *Handler) UpdateACL(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{"error": aclRemovedMsg})
}
