// Package github implements GitHub OAuth 2.0 support for Wirety.
//
// GitHub is not a standard OIDC provider (no discovery document, no JWT ID token),
// but it can be used as an identity source when AUTH_ISSUER_URL=https://github.com.
// Token exchange and user-info resolution are handled server-side; the session is
// stored the same way as an OIDC session, with the raw GitHub access token prefixed
// by TokenPrefix so the auth middleware can recognise and skip JWT validation.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	// IssuerURL is the value to set in AUTH_ISSUER_URL to enable GitHub OAuth.
	IssuerURL = "https://github.com"

	// AuthorizationEndpoint is GitHub's OAuth authorization URL.
	AuthorizationEndpoint = "https://github.com/login/oauth/authorize"

	// TokenEndpoint is GitHub's OAuth token exchange URL.
	TokenEndpoint = "https://github.com/login/oauth/access_token"

	// Scope requests read access to the user's profile and email addresses.
	Scope = "read:user user:email"

	// TokenPrefix is prepended to GitHub access tokens stored in sessions so that
	// the auth middleware can identify GitHub sessions and skip JWT validation.
	TokenPrefix = "github:"
)

// IsGitHub reports whether issuerURL refers to github.com.
func IsGitHub(issuerURL string) bool {
	return strings.TrimSuffix(issuerURL, "/") == strings.TrimSuffix(IssuerURL, "/")
}

// ExchangeCode exchanges an OAuth authorisation code for a GitHub access token.
func ExchangeCode(ctx context.Context, clientID, clientSecret, code, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, body)
	}

	var tokenResp struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if tokenResp.Error != "" {
		return "", fmt.Errorf("github: %s — %s", tokenResp.Error, tokenResp.ErrorDescription)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token in github response")
	}
	return tokenResp.AccessToken, nil
}

// UserInfo holds the GitHub user fields Wirety cares about.
type UserInfo struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"` // empty when the user has set their email to private
}

// FetchUser returns the authenticated user's GitHub profile.
func FetchUser(ctx context.Context, accessToken string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("create user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("user request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read user response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user endpoint returned %d: %s", resp.StatusCode, body)
	}

	var user UserInfo
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("parse user response: %w", err)
	}
	return &user, nil
}

// FetchPrimaryEmail returns the user's primary verified email address.
// Use this when FetchUser returns a user with an empty Email field (private email setting).
func FetchPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", fmt.Errorf("create emails request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("emails request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read emails response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("emails endpoint returned %d: %s", resp.StatusCode, body)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", fmt.Errorf("parse emails response: %w", err)
	}
	// Prefer primary verified email, fall back to any verified email.
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}
	return "", nil
}
