package api

import (
	"net/http"

	"wirety/internal/adapters/api/middleware"
	"wirety/internal/domain/auth"

	"github.com/gin-gonic/gin"
)

// ListUsers godoc
// @Summary      List all users
// @Description  Get a list of all users (admin only)
// @Tags         users
// @Produce      json
// @Success      200 {array} auth.User
// @Failure      403 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /users [get]
// @Security     BearerAuth
func (h *Handler) ListUsers(c *gin.Context) {
	users, err := h.userRepo.ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, users)
}

// GetUser godoc
// @Summary      Get user details
// @Description  Get details for a specific user (admin only)
// @Tags         users
// @Produce      json
// @Param        userId path string true "User ID"
// @Success      200 {object} auth.User
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /users/{userId} [get]
// @Security     BearerAuth
func (h *Handler) GetUser(c *gin.Context) {
	userID := c.Param("userId")

	user, err := h.userRepo.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdateUser godoc
// @Summary      Update user
// @Description  Update user role and authorized networks (admin only)
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        userId path string true "User ID"
// @Param        user body auth.UserUpdateRequest true "User update request"
// @Success      200 {object} auth.User
// @Failure      400 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /users/{userId} [put]
// @Security     BearerAuth
func (h *Handler) UpdateUser(c *gin.Context) {
	userID := c.Param("userId")

	var req auth.UserUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.userRepo.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Update fields
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Role != "" {
		user.Role = req.Role
	}
	if req.AuthorizedNetworks != nil {
		user.AuthorizedNetworks = req.AuthorizedNetworks
	}

	if err := h.userRepo.UpdateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// DeleteUser godoc
// @Summary      Delete user
// @Description  Delete a user (admin only)
// @Tags         users
// @Param        userId path string true "User ID"
// @Success      204
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /users/{userId} [delete]
// @Security     BearerAuth
func (h *Handler) DeleteUser(c *gin.Context) {
	userID := c.Param("userId")

	if err := h.userRepo.DeleteUser(userID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetCurrentUser godoc
// @Summary      Get current user
// @Description  Get the currently authenticated user's information
// @Tags         users
// @Produce      json
// @Success      200 {object} auth.User
// @Failure      401 {object} map[string]string
// @Router       /users/me [get]
// @Security     BearerAuth
func (h *Handler) GetCurrentUser(c *gin.Context) {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// GetDefaultPermissions godoc
// @Summary      Get default permissions
// @Description  Get default permissions for new users (admin only)
// @Tags         users
// @Produce      json
// @Success      200 {object} auth.DefaultNetworkPermissions
// @Failure      403 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /users/defaults [get]
// @Security     BearerAuth
func (h *Handler) GetDefaultPermissions(c *gin.Context) {
	perms, err := h.userRepo.GetDefaultPermissions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, perms)
}

// UpdateDefaultPermissions godoc
// @Summary      Update default permissions
// @Description  Update default permissions for new users (admin only)
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        defaults body auth.DefaultNetworkPermissions true "Default permissions"
// @Success      200 {object} auth.DefaultNetworkPermissions
// @Failure      400 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Router       /users/defaults [put]
// @Security     BearerAuth
func (h *Handler) UpdateDefaultPermissions(c *gin.Context) {
	var perms auth.DefaultNetworkPermissions
	if err := c.ShouldBindJSON(&perms); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.userRepo.SetDefaultPermissions(&perms); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, perms)
}
