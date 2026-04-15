package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"wirety/internal/audit"
	appauth "wirety/internal/application/auth"
	"wirety/internal/domain/auth"
	"wirety/internal/infrastructure/github"
	"wirety/internal/infrastructure/oidc"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// Uses shared OIDC discovery adapter (internal/infrastructure/oidc)

// flexInt unmarshals a JSON number or a quoted number string into an int.
// Azure Entra ID returns expires_in as a string ("3600") rather than an integer.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	// Try as a plain number first
	var n int
	if err := json.Unmarshal(b, &n); err == nil {
		*f = flexInt(n)
		return nil
	}
	// Fall back to a quoted string
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	var n2 int
	if _, err := fmt.Sscan(s, &n2); err != nil {
		return fmt.Errorf("flexInt: cannot parse %q as int", s)
	}
	*f = flexInt(n2)
	return nil
}

// AuthConfigResponse contains public authentication configuration
type AuthConfigResponse struct {
	Enabled               bool   `json:"enabled"`
	IssuerURL             string `json:"issuer_url"`
	ClientID              string `json:"client_id"`
	SimpleAuth            bool   `json:"simple_auth"`             // true when AUTH_ENABLED=false (admin/password login)
	AuthorizationEndpoint string `json:"authorization_endpoint"` // populated from OIDC discovery (or hardcoded for GitHub)
	EndSessionEndpoint    string `json:"end_session_endpoint"`    // populated from OIDC discovery; empty if provider does not support RP-initiated logout
	Scope                 string `json:"scope"`                  // OAuth scopes the frontend must request; provider-specific
}

// GetAuthConfig godoc
// @Summary      Get authentication configuration
// @Description  Get public authentication configuration (no auth required). When OIDC is enabled, authorization_endpoint and end_session_endpoint are resolved server-side to avoid CORS issues with the IdP discovery document.
// @Tags         auth
// @Produce      json
// @Success      200 {object} AuthConfigResponse
// @Router       /auth/config [get]
func (h *Handler) GetAuthConfig(c *gin.Context) {
	resp := AuthConfigResponse{
		Enabled:    h.authConfig.Enabled,
		IssuerURL:  h.authConfig.IssuerURL,
		ClientID:   h.authConfig.ClientID,
		SimpleAuth: !h.authConfig.Enabled,
		Scope:      "openid profile email offline_access",
	}

	if h.authConfig.Enabled && h.authConfig.IssuerURL != "" {
		if github.IsGitHub(h.authConfig.IssuerURL) {
			resp.AuthorizationEndpoint = github.AuthorizationEndpoint
			scope := github.Scope
			if h.authConfig.AdminGroup != "" || h.authConfig.UserGroup != "" {
				scope += " " + github.ScopeOrg
			}
			resp.Scope = scope
		} else if discovery, err := oidc.Discover(c.Request.Context(), h.authConfig.IssuerURL); err == nil {
			resp.AuthorizationEndpoint = discovery.AuthorizationEndpoint
			resp.EndSessionEndpoint = discovery.EndSessionEndpoint
		} else {
			log.Warn().Err(err).Msg("GetAuthConfig: could not fetch OIDC discovery document")
		}
	}

	c.JSON(http.StatusOK, resp)
}

// TokenRequest contains the authorization code exchange request
type TokenRequest struct {
	Code        string `json:"code" binding:"required"`
	RedirectURI string `json:"redirect_uri" binding:"required"`
}

// TokenResponse contains the session hash response
type TokenResponse struct {
	SessionHash string `json:"session_hash"`
	ExpiresIn   int    `json:"expires_in"`
}

// ExchangeToken godoc
// @Summary      Exchange authorization code for session
// @Description  Exchange OIDC authorization code for access token and create server-side session
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body TokenRequest true "Token exchange request"
// @Success      200 {object} TokenResponse
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /auth/token [post]
func (h *Handler) ExchangeToken(c *gin.Context) {
	if !h.authConfig.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authentication is not enabled"})
		return
	}

	var req TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// GitHub OAuth: bypass the standard OIDC flow (no JWT, no discovery document).
	if github.IsGitHub(h.authConfig.IssuerURL) {
		h.exchangeGitHubToken(c, req)
		return
	}

	// Discover OIDC endpoints via shared adapter
	discovery, err := oidc.Discover(c.Request.Context(), h.authConfig.IssuerURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to discover OIDC endpoints: %v", err)})
		return
	}

	// Prepare token exchange request with offline_access scope to get refresh token
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", req.Code)
	data.Set("redirect_uri", req.RedirectURI)
	data.Set("client_id", h.authConfig.ClientID)
	data.Set("client_secret", h.authConfig.ClientSecret)

	// Make request to token endpoint from discovery
	resp, err := http.DefaultClient.Post(discovery.TokenEndpoint, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to exchange token: %v", err)})
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read response: %v", err)})
		return
	}

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Token exchange failed: %s", string(body))})
		return
	}

	var oidcTokenResp struct {
		AccessToken  string  `json:"access_token"`
		IDToken      string  `json:"id_token"`      // OIDC identity token — always a JWT
		RefreshToken string  `json:"refresh_token"`
		ExpiresIn    flexInt `json:"expires_in"`
		TokenType    string  `json:"token_type"`
	}
	if err := json.Unmarshal(body, &oidcTokenResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse token response: %v", err)})
		return
	}

	if oidcTokenResp.RefreshToken == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No refresh token received. Ensure offline_access scope is requested."})
		return
	}

	// Prefer id_token for validation: it is always a standard JWT per the OIDC spec.
	// Some providers (e.g. Azure Entra ID) return an opaque, non-JWT access_token
	// intended for Microsoft APIs — parsing it as a JWT fails with "invalid number of segments".
	identityToken := oidcTokenResp.IDToken
	if identityToken == "" {
		identityToken = oidcTokenResp.AccessToken
	}

	// Validate the identity token and get user claims
	claims, err := h.authService.ValidateToken(c.Request.Context(), identityToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to validate token: %v", err)})
		return
	}

	// Some providers (e.g. Azure Entra ID) do not include the email claim in the
	// id_token by default. Fall back to the userinfo endpoint using the access_token.
	if claims.Email == "" && discovery.UserinfoEndpoint != "" {
		log.Debug().Str("userinfo_endpoint", discovery.UserinfoEndpoint).Msg("email missing from token claims, fetching userinfo")
		uiReq, uiErr := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, discovery.UserinfoEndpoint, nil)
		if uiErr == nil {
			uiReq.Header.Set("Authorization", "Bearer "+oidcTokenResp.AccessToken)
			uiResp, uiErr := http.DefaultClient.Do(uiReq)
			if uiErr != nil {
				log.Debug().Err(uiErr).Msg("userinfo request failed")
			} else {
				defer func() { _ = uiResp.Body.Close() }()
				body, _ := io.ReadAll(uiResp.Body)
				log.Debug().Int("status", uiResp.StatusCode).RawJSON("body", body).Msg("userinfo response")
				var uiClaims struct {
					Email         string `json:"email"`
					EmailVerified bool   `json:"email_verified"`
					Name          string `json:"name"`
					UPN           string `json:"upn"` // Azure Entra ID: user principal name
				}
				if json.Unmarshal(body, &uiClaims) == nil {
					email := uiClaims.Email
					if email == "" {
						email = uiClaims.UPN // Azure fallback
					}
					if email != "" {
						claims.Email = email
						claims.EmailVerified = uiClaims.EmailVerified
					}
					if claims.Name == "" && uiClaims.Name != "" {
						claims.Name = uiClaims.Name
					}
				}
			}
		}
	}

	if claims.Email == "" {
		log.Debug().
			Str("subject", claims.Subject).
			Str("name", claims.Name).
			Str("preferred_username", claims.PreferredUsername).
			Str("given_name", claims.GivenName).
			Str("family_name", claims.FamilyName).
			Str("issuer", claims.Issuer).
			Bool("email_verified", claims.EmailVerified).
			Str("userinfo_endpoint", discovery.UserinfoEndpoint).
			Msg("OIDC login blocked: email claim is empty after token + userinfo resolution")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Your account does not have an email address. Please configure your identity provider to expose the email claim."})
		return
	}

	// Get or create user
	user, err := h.authService.GetOrCreateUser(c.Request.Context(), claims)
	if err != nil {
		if errors.Is(err, appauth.ErrNotInAuthorizedGroup) {
			audit.Server(claims.Subject, claims.Email, c.ClientIP()).
				Str("action", "auth.rejected").
				Str("reason", "not_in_authorized_group").
				Strs("groups", claims.Groups).
				Str("issuer", claims.Issuer).
				Msg("audit")
			c.JSON(http.StatusForbidden, gin.H{"error": "You are not authorized"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get user: %v", err)})
		return
	}

	// Store the identity token (id_token / JWT) in the session so that middleware can
	// re-validate it without hitting the same opaque-token problem on refresh.
	session, err := h.createSession(user.ID, identityToken, oidcTokenResp.RefreshToken, int(oidcTokenResp.ExpiresIn))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create session: %v", err)})
		return
	}

	h.setSessionCookie(c, session.SessionHash, 30*24*3600)
	c.JSON(http.StatusOK, TokenResponse{
		SessionHash: session.SessionHash,
		ExpiresIn:   int(oidcTokenResp.ExpiresIn),
	})
}

// createSession creates a new session with a secure hash.
// For OIDC sessions pass the real tokens and expiresIn; for simple auth pass empty strings and 0.
func (h *Handler) createSession(userID, accessToken, refreshToken string, expiresIn int) (*auth.Session, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate session hash: %w", err)
	}
	hash := sha256.Sum256(randomBytes)
	sessionHash := hex.EncodeToString(hash[:])

	var accessTokenExpiresAt time.Time
	if expiresIn > 0 {
		accessTokenExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	} else {
		// Simple auth: access token never expires (no OIDC token to refresh)
		accessTokenExpiresAt = time.Now().Add(100 * 365 * 24 * time.Hour)
	}

	session := &auth.Session{
		SessionHash:           sessionHash,
		UserID:                userID,
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		AccessTokenExpiresAt:  accessTokenExpiresAt,
		RefreshTokenExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}

	if err := h.userRepo.CreateSession(session); err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	return session, nil
}

// LogoutRequest contains the session hash to logout (optional when using cookie-based auth)
type LogoutRequest struct {
	SessionHash string `json:"session_hash"`
}

// Logout godoc
// @Summary      Logout and invalidate session
// @Description  Logout user and delete server-side session
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body LogoutRequest true "Logout request"
// @Success      200 {object} map[string]string
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	// Resolve session hash: prefer cookie, fall back to request body
	sessionHash := c.GetHeader("Authorization")
	if strings.HasPrefix(sessionHash, "Session ") {
		sessionHash = strings.TrimPrefix(sessionHash, "Session ")
	} else if cookie, err := c.Cookie("wirety_session"); err == nil {
		sessionHash = cookie
	} else {
		var req LogoutRequest
		if err := c.ShouldBindJSON(&req); err == nil {
			sessionHash = req.SessionHash
		}
	}

	if sessionHash != "" {
		_ = h.userRepo.DeleteSession(sessionHash)
	}

	h.clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// SimpleLoginRequest contains credentials for simple auth login
type SimpleLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// SimpleLogin godoc
// @Summary      Login with admin password
// @Description  Authenticate using admin credentials when OIDC is disabled (AUTH_ENABLED=false)
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body SimpleLoginRequest true "Login credentials"
// @Success      200 {object} TokenResponse
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Router       /auth/login [post]
func (h *Handler) SimpleLogin(c *gin.Context) {
	if h.authConfig.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "simple auth is not available when OIDC is enabled"})
		return
	}

	var req SimpleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Constant-time comparison to prevent timing attacks
	usernameMatch := subtle.ConstantTimeCompare([]byte(req.Username), []byte("admin"))
	passwordMatch := subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.authConfig.AdminPassword))
	if usernameMatch != 1 || passwordMatch != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	session, err := h.createSession("admin", "", "", 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	audit.Server("admin", "admin@wirety.local", c.ClientIP()).
		Str("action", "auth.login").
		Str("username", req.Username).
		Msg("audit")

	h.setSessionCookie(c, session.SessionHash, 30*24*3600)
	c.JSON(http.StatusOK, TokenResponse{
		SessionHash: session.SessionHash,
		ExpiresIn:   30 * 24 * 3600,
	})
}

// exchangeGitHubToken handles the /auth/token exchange for GitHub OAuth.
// It replaces the standard OIDC flow because GitHub does not issue JWT ID tokens.
func (h *Handler) exchangeGitHubToken(c *gin.Context, req TokenRequest) {
	// 1. Exchange authorization code for a GitHub access token.
	accessToken, err := github.ExchangeCode(c.Request.Context(), h.authConfig.ClientID, h.authConfig.ClientSecret, req.Code, req.RedirectURI)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("GitHub token exchange failed: %v", err)})
		return
	}

	// 2. Fetch user profile.
	ghUser, err := github.FetchUser(c.Request.Context(), accessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch GitHub user: %v", err)})
		return
	}

	// 3. Resolve email — GitHub users may hide their email; fall back to /user/emails.
	email := ghUser.Email
	if email == "" {
		log.Debug().Int("github_id", ghUser.ID).Msg("email missing from GitHub user profile, fetching /user/emails")
		email, err = github.FetchPrimaryEmail(c.Request.Context(), accessToken)
		if err != nil {
			log.Warn().Err(err).Msg("failed to fetch GitHub user emails")
		}
	}
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Your GitHub account does not have a verified email address. Please add and verify an email in your GitHub settings."})
		return
	}

	// 4. Resolve display name — fall back to login handle if name is not set.
	name := ghUser.Name
	if name == "" {
		name = ghUser.Login
	}

	// 5. Resolve group memberships (only when group access control is configured).
	//    Requires the read:org scope which is added to the OAuth consent screen when
	//    AUTH_ADMIN_GROUP or AUTH_USER_GROUP are set.
	var ghGroups []string
	if h.authConfig.AdminGroup != "" || h.authConfig.UserGroup != "" {
		groups, groupErr := github.FetchUserGroups(c.Request.Context(), accessToken)
		if groupErr != nil {
			log.Warn().Err(groupErr).Msg("exchangeGitHubToken: failed to fetch GitHub user groups")
		} else {
			ghGroups = groups
		}
	}

	// 6. Build OIDCClaims so we can reuse the existing GetOrCreateUser logic.
	claims := &auth.OIDCClaims{
		Subject:       fmt.Sprintf("github:%d", ghUser.ID),
		Email:         email,
		EmailVerified: true,
		Name:          name,
		Issuer:        github.IssuerURL,
		Groups:        ghGroups,
	}

	// 7. Get or create the Wirety user.
	user, err := h.authService.GetOrCreateUser(c.Request.Context(), claims)
	if err != nil {
		if errors.Is(err, appauth.ErrNotInAuthorizedGroup) {
			audit.Server(claims.Subject, claims.Email, c.ClientIP()).
				Str("action", "auth.rejected").
				Str("reason", "not_in_authorized_group").
				Strs("groups", claims.Groups).
				Str("issuer", claims.Issuer).
				Msg("audit")
			c.JSON(http.StatusForbidden, gin.H{"error": "You are not authorized"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get user: %v", err)})
		return
	}

	// 8. Create server-side session.
	// Store the GitHub token with a prefix so the middleware can skip JWT validation.
	// expiresIn=0 → AccessTokenExpiresAt is set to 100 years from now (GitHub tokens
	// do not expire unless explicitly revoked).
	session, err := h.createSession(user.ID, github.TokenPrefix+accessToken, "", 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create session: %v", err)})
		return
	}

	h.setSessionCookie(c, session.SessionHash, 30*24*3600)
	c.JSON(http.StatusOK, TokenResponse{
		SessionHash: session.SessionHash,
		ExpiresIn:   30 * 24 * 3600,
	})
}

const sessionCookieName = "wirety_session"

// setSessionCookie sets an HttpOnly session cookie on the response.
// The Secure flag is controlled by the handler's authConfig.CookieSecure setting.
func (h *Handler) setSessionCookie(c *gin.Context, sessionHash string, maxAge int) {
	c.SetCookie(sessionCookieName, sessionHash, maxAge, "/", "", h.authConfig.CookieSecure, true)
}

// clearSessionCookie clears the session cookie.
func (h *Handler) clearSessionCookie(c *gin.Context) {
	c.SetCookie(sessionCookieName, "", -1, "/", "", h.authConfig.CookieSecure, true)
}

