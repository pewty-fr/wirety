package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"wirety/internal/config"
	"wirety/internal/domain/auth"
	"wirety/internal/infrastructure/oidc"

	"github.com/golang-jwt/jwt/v5"
)

// Service handles authentication and authorization
type Service struct {
	config       *config.AuthConfig
	userRepo     auth.Repository
	jwksCache    map[string]interface{}
	jwksCacheMu  sync.RWMutex
	jwksCacheExp time.Time
}

// NewService creates a new authentication service
func NewService(cfg *config.AuthConfig, userRepo auth.Repository) *Service {
	return &Service{
		config:    cfg,
		userRepo:  userRepo,
		jwksCache: make(map[string]interface{}),
	}
}

// ValidateToken validates an OIDC JWT token and returns the claims
func (s *Service) ValidateToken(ctx context.Context, tokenString string) (*auth.OIDCClaims, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("authentication is not enabled")
	}

	// Parse the token without validation first to get the header
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("invalid token format: %w", err)
	}

	// Get the key ID from the token header
	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, fmt.Errorf("token missing kid header")
	}

	// Get the public key from JWKS
	publicKey, err := s.getPublicKey(ctx, kid)
	if err != nil {
		return nil, fmt.Errorf("failed to get public key: %w", err)
	}

	// Parse and validate the token with the public key
	parsedToken, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	if !parsedToken.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Extract claims
	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Convert to OIDCClaims
	oidcClaims := &auth.OIDCClaims{
		Subject:           getStringClaim(claims, "sub"),
		Email:             getStringClaim(claims, "email"),
		EmailVerified:     getBoolClaim(claims, "email_verified"),
		Name:              getStringClaim(claims, "name"),
		PreferredUsername: getStringClaim(claims, "preferred_username"),
		GivenName:         getStringClaim(claims, "given_name"),
		FamilyName:        getStringClaim(claims, "family_name"),
		Issuer:            getStringClaim(claims, "iss"),
		ExpiresAt:         getInt64Claim(claims, "exp"),
		IssuedAt:          getInt64Claim(claims, "iat"),
		AuthorizedParty:   getStringClaim(claims, "azp"),
	}

	// Verify issuer
	if oidcClaims.Issuer != s.config.IssuerURL {
		return nil, fmt.Errorf("invalid issuer: expected %s, got %s", s.config.IssuerURL, oidcClaims.Issuer)
	}

	// Verify azp if ClientID is set
	if s.config.ClientID != "" {
		if azp, ok := claims["azp"].([]interface{}); ok {
			found := false
			for _, a := range azp {
				if a.(string) == s.config.ClientID {
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("invalid audience")
			}
		} else if azp, ok := claims["azp"].(string); ok {
			if azp != s.config.ClientID {
				return nil, fmt.Errorf("invalid audience")
			}
		}
	}

	return oidcClaims, nil
}

// GetOrCreateUser gets an existing user or creates a new one based on OIDC claims
func (s *Service) GetOrCreateUser(ctx context.Context, claims *auth.OIDCClaims) (*auth.User, error) {
	// Try to get existing user
	user, err := s.userRepo.GetUser(claims.Subject)
	if err == nil {
		// Update last login
		user.LastLoginAt = time.Now()
		_ = s.userRepo.UpdateUser(user)
		return user, nil
	}

	// Check if this is the first user
	firstUser, err := s.userRepo.GetFirstUser()
	isFirstUser := err != nil || firstUser == nil

	// Create new user
	user = &auth.User{
		ID:                 claims.Subject,
		Email:              claims.Email,
		Name:               claims.Name,
		Role:               auth.RoleUser,
		AuthorizedNetworks: []string{},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
		LastLoginAt:        time.Now(),
	}

	// First user becomes administrator
	if isFirstUser {
		user.Role = auth.RoleAdministrator
	} else {
		// Apply default permissions for new users
		defaultPerms, err := s.userRepo.GetDefaultPermissions()
		if err == nil && defaultPerms != nil {
			user.Role = defaultPerms.DefaultRole
			user.AuthorizedNetworks = defaultPerms.DefaultAuthorizedNetworks
		}
	}

	if err := s.userRepo.CreateUser(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// getPublicKey retrieves the public key from JWKS endpoint
func (s *Service) getPublicKey(ctx context.Context, kid string) (interface{}, error) {
	// Check cache
	s.jwksCacheMu.RLock()
	if time.Now().Before(s.jwksCacheExp) {
		if key, ok := s.jwksCache[kid]; ok {
			s.jwksCacheMu.RUnlock()
			return key, nil
		}
	}
	s.jwksCacheMu.RUnlock()

	// Discover OIDC endpoints via shared adapter
	discovery, err := oidc.Discover(ctx, s.config.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery failed: %w", err)
	}

	// Fetch JWKS from discovered endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discovery.JwksURI, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch JWKS: status %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []map[string]interface{} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	// Update cache
	s.jwksCacheMu.Lock()
	s.jwksCache = make(map[string]interface{})
	for _, key := range jwks.Keys {
		if keyID, ok := key["kid"].(string); ok {
			// Convert JWK to public key
			pubKey, err := jwkToPublicKey(key)
			if err == nil {
				s.jwksCache[keyID] = pubKey
			}
		}
	}
	s.jwksCacheExp = time.Now().Add(time.Duration(s.config.JWKSCacheTTL) * time.Second)
	s.jwksCacheMu.Unlock()

	// Get the requested key
	s.jwksCacheMu.RLock()
	defer s.jwksCacheMu.RUnlock()
	if key, ok := s.jwksCache[kid]; ok {
		return key, nil
	}

	return nil, fmt.Errorf("key %s not found in JWKS", kid)
}

// Helper functions to extract claims
func getStringClaim(claims jwt.MapClaims, key string) string {
	if val, ok := claims[key].(string); ok {
		return val
	}
	return ""
}

func getBoolClaim(claims jwt.MapClaims, key string) bool {
	if val, ok := claims[key].(bool); ok {
		return val
	}
	return false
}

func getInt64Claim(claims jwt.MapClaims, key string) int64 {
	if val, ok := claims[key].(float64); ok {
		return int64(val)
	}
	return 0
}

// RefreshAccessToken refreshes an access token using a refresh token
func (s *Service) RefreshAccessToken(ctx context.Context, refreshToken string) (string, int, error) {
	if !s.config.Enabled {
		return "", 0, fmt.Errorf("authentication is not enabled")
	}

	// Discover OIDC endpoints
	discovery, err := oidc.Discover(ctx, s.config.IssuerURL)
	if err != nil {
		return "", 0, fmt.Errorf("oidc discovery failed: %w", err)
	}

	// Prepare refresh token request (form-encoded, not JSON)
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", s.config.ClientID)
	data.Set("client_secret", s.config.ClientSecret)

	// Make request to token endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, discovery.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("token refresh failed: %s", string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", 0, fmt.Errorf("failed to parse token response: %w", err)
	}

	return tokenResp.AccessToken, tokenResp.ExpiresIn, nil
}
