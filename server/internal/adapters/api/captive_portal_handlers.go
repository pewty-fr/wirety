package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type createCaptiveTokenRequest struct {
	PeerIP       string `json:"peer_ip" binding:"required"`
	PeerEndpoint string `json:"peer_endpoint"` // full "ip:port" at connect time (optional)
}

// CreateCaptivePortalToken creates a captive portal token for a connecting peer.
// Called by the jump peer agent (authenticated via enrollment token) when a peer
// connects to the WireGuard tunnel and needs to authenticate.
func (h *Handler) CreateCaptivePortalToken(c *gin.Context) {
	// Captive portal requires OIDC so that each user has a distinct identity.
	// Simple auth (AUTH_ENABLED=false) uses a shared admin password — there is no
	// per-user identity to enforce peer ownership, so captive portal is disabled.
	if !h.authConfig.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "captive portal is not available when AUTH_ENABLED=false"})
		return
	}

	token := extractBearerToken(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}

	networkID, peer, err := h.service.ResolveAgentToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	if !peer.IsJump {
		c.JSON(http.StatusForbidden, gin.H{"error": "only jump peers can create captive portal tokens"})
		return
	}

	var req createCaptiveTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cpt, err := h.service.CreateCaptivePortalToken(c.Request.Context(), networkID, peer.ID, req.PeerIP, req.PeerEndpoint)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create token"})
		return
	}

	c.JSON(http.StatusCreated, cpt)
}

type authenticateCaptivePortalRequest struct {
	CaptiveToken string `json:"captive_token" binding:"required"`
}

// captivePortalStateCookieName is the same-origin cookie that binds a token to
// the browser session that received its URL via the agent's redirect.  See
// migration 026 + service.BindCaptivePortalTokenToBrowser for the rationale.
const captivePortalStateCookieName = "wirety_cp_state"

// captivePortalStateCookieTTL matches the captive-portal token lifetime.  No
// point keeping the cookie alive longer than the token it's bound to.
const captivePortalStateCookieTTL = 600 // seconds (10 min)

// CaptivePortalStart is the bouncer that the agent's redirect targets.  It
// generates a per-token consume_state, sets it as a same-origin cookie on the
// browser that landed here, and 302s the browser onward to the captive portal
// page (which now finds both the token in the URL AND the cookie that binds
// it).  See migration 026 for the rationale (phishing defense).
//
// GET /api/v1/captive-portal/start?token=cpt_…&redirect=…
func (h *Handler) CaptivePortalStart(c *gin.Context) {
	captiveToken := c.Query("token")
	if captiveToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}
	state, err := h.service.BindCaptivePortalTokenToBrowser(c.Request.Context(), captiveToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Cookie configuration:
	//   - Path=/ so it's available on /api/v1/captive-portal/authenticate AND
	//     /captive-portal (frontend route).
	//   - HttpOnly: JS in the captive portal page does not need to read it; the
	//     auth POST sends it automatically as a regular cookie.  Reduces XSS
	//     impact.
	//   - Secure: HTTPS only.
	//   - SameSite=Lax: allows the cookie to ride along with the top-level
	//     navigation that lands on /captive-portal (which is a same-origin
	//     redirect from /start, but a phisher's link would be cross-origin
	//     navigation if anything).  Strict would also work for our flow but
	//     Lax is more lenient and lets the cookie survive a future evolution
	//     where /start is reached via a redirect-from-elsewhere.
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		captivePortalStateCookieName,
		state,
		captivePortalStateCookieTTL,
		"/",
		"",   // domain — empty means current host only (correct)
		true, // Secure
		true, // HttpOnly
	)

	// Carry the original token + optional redirect-after-success through to
	// the frontend page.  The frontend's CaptivePortalPage already reads
	// these from the URL.
	target := "/captive-portal?token=" + captiveToken
	if r := c.Query("redirect"); r != "" {
		target += "&redirect=" + r
	}
	c.Redirect(http.StatusFound, target)
}

// CaptivePortalPreview returns peer details for the captive-portal page to
// display BEFORE the user clicks Continue, so they can verify the device name
// and the public IP that triggered the sign-in request.  This is the user-
// visible half of phishing defense — the user can spot a token issued from
// an unfamiliar IP and abort.
//
// GET /api/v1/captive-portal/preview?token=cpt_…
//
// Authed callers only — same session cookie as /authenticate.  Owner check
// is enforced (a user shouldn't be able to peek at someone else's tokens).
func (h *Handler) CaptivePortalPreview(c *gin.Context) {
	if !h.authConfig.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "captive portal is not available when AUTH_ENABLED=false"})
		return
	}
	captiveToken := c.Query("token")
	if captiveToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}
	sessionHash, err := c.Cookie(sessionCookieName)
	if err != nil || sessionHash == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session found, please log in first"})
		return
	}
	preview, err := h.service.PreviewCaptivePortalToken(c.Request.Context(), captiveToken, sessionHash)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, preview)
}

// AuthenticateCaptivePortal validates the captive portal token and session, then whitelists the peer.
// Called by the frontend captive portal page after the user authenticates via OIDC or password.
// The session is read from the wirety_session cookie — no need to send session_hash in the body.
//
// Browser-binding: also reads the wirety_cp_state cookie set by /start and
// passes it down to the service so the consume_state on the token row can be
// verified.  See migration 026 for the phishing-defense rationale.
func (h *Handler) AuthenticateCaptivePortal(c *gin.Context) {
	if !h.authConfig.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "captive portal is not available when AUTH_ENABLED=false"})
		return
	}

	var req authenticateCaptivePortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Resolve session hash from cookie (same cookie used by every authenticated page)
	sessionHash, err := c.Cookie(sessionCookieName)
	if err != nil || sessionHash == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session found, please log in first"})
		return
	}

	// Browser-binding cookie set by /start.  Missing or non-matching → reject.
	browserState, _ := c.Cookie(captivePortalStateCookieName)

	cpt, err := h.service.AuthenticateCaptivePortal(c.Request.Context(), req.CaptiveToken, sessionHash, browserState)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Auth succeeded — the cookie has done its job, clear it so a forgotten
	// browser tab can't replay against another captive-portal token later.
	c.SetCookie(captivePortalStateCookieName, "", -1, "/", "", true, true)

	c.JSON(http.StatusOK, gin.H{
		"peer_ip":     cpt.PeerIP,
		"network_id":  cpt.NetworkID,
		"whitelisted": true,
	})
}
