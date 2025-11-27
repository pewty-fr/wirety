package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Discovery represents the OIDC discovery document fields we need.
type Discovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	JwksURI               string `json:"jwks_uri"`
}

var (
	cacheMu sync.RWMutex
	cache   = map[string]*cachedItem{}
	// ttl defines how long we keep discovery metadata.
	ttl = time.Hour
)

type cachedItem struct {
	value     *Discovery
	expiresAt time.Time
}

// Discover returns provider metadata, performing a network request only when necessary.
func Discover(ctx context.Context, issuerURL string) (*Discovery, error) {
	issuerURL = strings.TrimSuffix(issuerURL, "/")
	cacheMu.RLock()
	item, found := cache[issuerURL]
	if found && time.Now().Before(item.expiresAt) {
		val := item.value
		cacheMu.RUnlock()
		return val, nil
	}
	cacheMu.RUnlock()
	discoveryURL := issuerURL + "/.well-known/openid-configuration"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch discovery document: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery endpoint returned status %d", resp.StatusCode)
	}
	var doc Discovery
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("failed to parse discovery document: %w", err)
	}

	cacheMu.Lock()
	cache[issuerURL] = &cachedItem{value: &doc, expiresAt: time.Now().Add(ttl)}
	cacheMu.Unlock()
	return &doc, nil
}
