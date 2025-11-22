package auth

// OIDCClaims represents the claims extracted from an OIDC JWT token
type OIDCClaims struct {
	Subject           string   `json:"sub"`                // User ID
	Email             string   `json:"email"`              // User email
	EmailVerified     bool     `json:"email_verified"`     // Email verification status
	Name              string   `json:"name"`               // Full name
	PreferredUsername string   `json:"preferred_username"` // Username
	GivenName         string   `json:"given_name"`         // First name
	FamilyName        string   `json:"family_name"`        // Last name
	Issuer            string   `json:"iss"`                // Token issuer
	Audience          []string `json:"aud"`                // Token audience
	ExpiresAt         int64    `json:"exp"`                // Expiration time
	IssuedAt          int64    `json:"iat"`                // Issued at time
	AuthorizedParty   string   `json:"azp"`                // Authorized party
}
