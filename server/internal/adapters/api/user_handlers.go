package api

import (
	"net/http"

	"wirety/internal/adapters/api/middleware"
	"wirety/internal/audit"
	"wirety/internal/domain/auth"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// minPasswordLength is the minimum length enforced when setting a user password.
const minPasswordLength = 8

// hashPassword bcrypts a password with the default cost. Returns "" if password is "".
func hashPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CreateUser godoc
// @Summary      Create user (admin only, simple-auth mode)
// @Description  Create a new local user with email + password. Only available when AUTH_ENABLED=false; in OIDC mode users are auto-created on first login.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        user body auth.UserCreateRequest true "User to create"
// @Success      201 {object} auth.User
// @Failure      400 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Router       /users [post]
// @Security     BearerAuth
func (h *Handler) CreateUser(c *gin.Context) {
	// Local user creation is only meaningful in simple-auth mode — OIDC mode
	// auto-creates users on first login.
	if h.authConfig != nil && h.authConfig.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "user creation via API is only available when AUTH_ENABLED=false"})
		return
	}

	var req auth.UserCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Password) < minPasswordLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}
	if req.Role != auth.RoleAdministrator && req.Role != auth.RoleUser {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be 'administrator' or 'user'"})
		return
	}

	if existing, _ := h.userRepo.GetUserByEmail(req.Email); existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "a user with this email already exists"})
		return
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	user := &auth.User{
		ID:                 uuid.New().String(),
		Email:              req.Email,
		Name:               req.Name,
		Role:               req.Role,
		AuthorizedNetworks: req.AuthorizedNetworks,
		PasswordHash:       hash,
	}
	if err := h.userRepo.CreateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	id, email := actor(c)
	audit.Server(id, email, c.ClientIP()).
		Str("action", "user.create").
		Str("target_user_id", user.ID).
		Str("target_email", user.Email).
		Str("target_role", string(user.Role)).
		Msg("audit")

	c.JSON(http.StatusCreated, user)
}

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

	// Optional password reset (admin-only; OIDC users have no password to reset).
	passwordReset := false
	if req.Password != "" {
		if len(req.Password) < minPasswordLength {
			c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
			return
		}
		hash, err := hashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
			return
		}
		user.PasswordHash = hash
		passwordReset = true
	}

	if err := h.userRepo.UpdateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	id, email := actor(c)
	ev := audit.Server(id, email, c.ClientIP()).
		Str("action", "user.update").
		Str("target_user_id", userID)
	if passwordReset {
		ev = ev.Bool("password_reset", true)
		// Invalidate all existing sessions so the user must log in again with the new password.
		_ = h.userRepo.DeleteUserSessions(userID)
	}
	ev.Msg("audit")

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

	id, email := actor(c)
	audit.Server(id, email, c.ClientIP()).
		Str("action", "user.delete").
		Str("target_user_id", userID).
		Msg("audit")

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

	id, email := actor(c)
	audit.Server(id, email, c.ClientIP()).
		Str("action", "user.defaults.update").
		Msg("audit")

	c.JSON(http.StatusOK, perms)
}
