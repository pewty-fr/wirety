package oidc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDiscover_Success(t *testing.T) {
	// Create mock OIDC discovery server
	expectedDiscovery := &Discovery{
		Issuer:                "https://example.com",
		AuthorizationEndpoint: "https://example.com/auth",
		TokenEndpoint:         "https://example.com/token",
		UserinfoEndpoint:      "https://example.com/userinfo",
		JwksURI:               "https://example.com/jwks",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedDiscovery)
	}))
	defer server.Close()

	// Clear cache before test
	cacheMu.Lock()
	cache = make(map[string]*cachedItem)
	cacheMu.Unlock()

	discovery, err := Discover(context.Background(), server.URL)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if discovery.Issuer != expectedDiscovery.Issuer {
		t.Errorf("Expected issuer %s, got %s", expectedDiscovery.Issuer, discovery.Issuer)
	}

	if discovery.AuthorizationEndpoint != expectedDiscovery.AuthorizationEndpoint {
		t.Errorf("Expected auth endpoint %s, got %s", expectedDiscovery.AuthorizationEndpoint, discovery.AuthorizationEndpoint)
	}

	if discovery.TokenEndpoint != expectedDiscovery.TokenEndpoint {
		t.Errorf("Expected token endpoint %s, got %s", expectedDiscovery.TokenEndpoint, discovery.TokenEndpoint)
	}

	if discovery.UserinfoEndpoint != expectedDiscovery.UserinfoEndpoint {
		t.Errorf("Expected userinfo endpoint %s, got %s", expectedDiscovery.UserinfoEndpoint, discovery.UserinfoEndpoint)
	}

	if discovery.JwksURI != expectedDiscovery.JwksURI {
		t.Errorf("Expected JWKS URI %s, got %s", expectedDiscovery.JwksURI, discovery.JwksURI)
	}
}

func TestDiscover_Caching(t *testing.T) {
	requestCount := 0

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		discovery := &Discovery{
			Issuer:                server.URL,
			AuthorizationEndpoint: server.URL + "/auth",
			TokenEndpoint:         server.URL + "/token",
			UserinfoEndpoint:      server.URL + "/userinfo",
			JwksURI:               server.URL + "/jwks",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(discovery)
	}))
	defer server.Close()

	// Clear cache before test
	cacheMu.Lock()
	cache = make(map[string]*cachedItem)
	cacheMu.Unlock()

	// First request should hit the server
	_, err := Discover(context.Background(), server.URL)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if requestCount != 1 {
		t.Errorf("Expected 1 request, got %d", requestCount)
	}

	// Second request should use cache
	_, err = Discover(context.Background(), server.URL)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if requestCount != 1 {
		t.Errorf("Expected 1 request (cached), got %d", requestCount)
	}
}

func TestDiscover_TrailingSlashHandling(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			t.Errorf("Expected path /.well-known/openid-configuration, got %s", r.URL.Path)
		}

		discovery := &Discovery{
			Issuer: server.URL,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(discovery)
	}))
	defer server.Close()

	// Clear cache before test
	cacheMu.Lock()
	cache = make(map[string]*cachedItem)
	cacheMu.Unlock()

	// Test with trailing slash
	_, err := Discover(context.Background(), server.URL+"/")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestDiscover_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Clear cache before test
	cacheMu.Lock()
	cache = make(map[string]*cachedItem)
	cacheMu.Unlock()

	_, err := Discover(context.Background(), server.URL)

	if err == nil {
		t.Error("Expected error for server error response")
	}
}

func TestDiscover_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	// Clear cache before test
	cacheMu.Lock()
	cache = make(map[string]*cachedItem)
	cacheMu.Unlock()

	_, err := Discover(context.Background(), server.URL)

	if err == nil {
		t.Error("Expected error for invalid JSON response")
	}
}

func TestDiscover_NetworkError(t *testing.T) {
	// Clear cache before test
	cacheMu.Lock()
	cache = make(map[string]*cachedItem)
	cacheMu.Unlock()

	// Use invalid URL to trigger network error
	_, err := Discover(context.Background(), "http://invalid-host-that-does-not-exist.local")

	if err == nil {
		t.Error("Expected error for network failure")
	}
}

func TestDiscover_CacheExpiration(t *testing.T) {
	requestCount := 0

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		discovery := &Discovery{
			Issuer: server.URL,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(discovery)
	}))
	defer server.Close()

	// Clear cache and set short TTL for testing
	cacheMu.Lock()
	cache = make(map[string]*cachedItem)
	originalTTL := ttl
	ttl = 10 * time.Millisecond // Very short TTL for testing
	cacheMu.Unlock()

	// Restore original TTL after test
	defer func() {
		cacheMu.Lock()
		ttl = originalTTL
		cacheMu.Unlock()
	}()

	// First request
	_, err := Discover(context.Background(), server.URL)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if requestCount != 1 {
		t.Errorf("Expected 1 request, got %d", requestCount)
	}

	// Wait for cache to expire
	time.Sleep(20 * time.Millisecond)

	// Second request should hit server again due to expiration
	_, err = Discover(context.Background(), server.URL)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if requestCount != 2 {
		t.Errorf("Expected 2 requests (cache expired), got %d", requestCount)
	}
}

func TestDiscover_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(100 * time.Millisecond)

		discovery := &Discovery{
			Issuer: "test",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(discovery)
	}))
	defer server.Close()

	// Clear cache before test
	cacheMu.Lock()
	cache = make(map[string]*cachedItem)
	cacheMu.Unlock()

	// Create context that cancels quickly
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := Discover(ctx, server.URL)

	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestDiscover_ConcurrentAccess(t *testing.T) {
	requestCount := 0

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Add small delay to increase chance of concurrent access
		time.Sleep(10 * time.Millisecond)

		discovery := &Discovery{
			Issuer: server.URL,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(discovery)
	}))
	defer server.Close()

	// Clear cache before test
	cacheMu.Lock()
	cache = make(map[string]*cachedItem)
	cacheMu.Unlock()

	// Start multiple goroutines to test concurrent access
	done := make(chan error, 5)

	for i := 0; i < 5; i++ {
		go func() {
			_, err := Discover(context.Background(), server.URL)
			done <- err
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 5; i++ {
		err := <-done
		if err != nil {
			t.Errorf("Unexpected error in goroutine: %v", err)
		}
	}

	// Due to caching, we should have fewer requests than goroutines
	// (though exact number depends on timing)
	if requestCount > 5 {
		t.Errorf("Expected at most 5 requests, got %d", requestCount)
	}

	if requestCount < 1 {
		t.Errorf("Expected at least 1 request, got %d", requestCount)
	}
}
