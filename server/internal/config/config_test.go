package config

import (
	"os"
	"testing"
)

func TestLoadConfig_DefaultValues(t *testing.T) {
	// Clear all environment variables
	clearEnvVars()

	config := LoadConfig()

	// Test default values
	if config.HTTPPort != "8080" {
		t.Errorf("Expected HTTPPort to be '8080', got '%s'", config.HTTPPort)
	}

	if config.AllowedOrigin != "*" {
		t.Errorf("Expected AllowedOrigin to be '*', got '%s'", config.AllowedOrigin)
	}

	// Test Auth defaults
	if config.Auth.Enabled != false {
		t.Errorf("Expected Auth.Enabled to be false, got %v", config.Auth.Enabled)
	}

	if config.Auth.IssuerURL != "" {
		t.Errorf("Expected Auth.IssuerURL to be empty, got '%s'", config.Auth.IssuerURL)
	}

	if config.Auth.ClientID != "" {
		t.Errorf("Expected Auth.ClientID to be empty, got '%s'", config.Auth.ClientID)
	}

	if config.Auth.ClientSecret != "" {
		t.Errorf("Expected Auth.ClientSecret to be empty, got '%s'", config.Auth.ClientSecret)
	}

	if config.Auth.JWKSCacheTTL != 3600 {
		t.Errorf("Expected Auth.JWKSCacheTTL to be 3600, got %d", config.Auth.JWKSCacheTTL)
	}

	// Test Database defaults
	if config.Database.Enabled != false {
		t.Errorf("Expected Database.Enabled to be false, got %v", config.Database.Enabled)
	}

	expectedDSN := "postgres://wirety:wirety@localhost:5432/wirety?sslmode=disable"
	if config.Database.DSN != expectedDSN {
		t.Errorf("Expected Database.DSN to be '%s', got '%s'", expectedDSN, config.Database.DSN)
	}

	expectedMigrations := "kodata/migrations"
	if config.Database.Migrations != expectedMigrations {
		t.Errorf("Expected Database.Migrations to be '%s', got '%s'", expectedMigrations, config.Database.Migrations)
	}
}

func TestLoadConfig_EnvironmentVariables(t *testing.T) {
	// Clear all environment variables first
	clearEnvVars()

	// Set environment variables
	_ = os.Setenv("HTTP_PORT", "9090")
	_ = os.Setenv("ALLOWED_ORIGIN", "https://example.com")
	_ = os.Setenv("AUTH_ENABLED", "true")
	_ = os.Setenv("AUTH_ISSUER_URL", "https://keycloak.example.com/realms/test")
	_ = os.Setenv("AUTH_CLIENT_ID", "test-client")
	_ = os.Setenv("AUTH_CLIENT_SECRET", "secret123")
	_ = os.Setenv("AUTH_JWKS_CACHE_TTL", "7200")
	_ = os.Setenv("DB_ENABLED", "true")
	_ = os.Setenv("DB_DSN", "postgres://test:test@localhost:5433/testdb")
	_ = os.Setenv("KO_DATA_PATH", "/custom/path")

	defer clearEnvVars() // Clean up after test

	config := LoadConfig()

	// Test environment variable values
	if config.HTTPPort != "9090" {
		t.Errorf("Expected HTTPPort to be '9090', got '%s'", config.HTTPPort)
	}

	if config.AllowedOrigin != "https://example.com" {
		t.Errorf("Expected AllowedOrigin to be 'https://example.com', got '%s'", config.AllowedOrigin)
	}

	// Test Auth environment values
	if config.Auth.Enabled != true {
		t.Errorf("Expected Auth.Enabled to be true, got %v", config.Auth.Enabled)
	}

	if config.Auth.IssuerURL != "https://keycloak.example.com/realms/test" {
		t.Errorf("Expected Auth.IssuerURL to be 'https://keycloak.example.com/realms/test', got '%s'", config.Auth.IssuerURL)
	}

	if config.Auth.ClientID != "test-client" {
		t.Errorf("Expected Auth.ClientID to be 'test-client', got '%s'", config.Auth.ClientID)
	}

	if config.Auth.ClientSecret != "secret123" {
		t.Errorf("Expected Auth.ClientSecret to be 'secret123', got '%s'", config.Auth.ClientSecret)
	}

	if config.Auth.JWKSCacheTTL != 7200 {
		t.Errorf("Expected Auth.JWKSCacheTTL to be 7200, got %d", config.Auth.JWKSCacheTTL)
	}

	// Test Database environment values
	if config.Database.Enabled != true {
		t.Errorf("Expected Database.Enabled to be true, got %v", config.Database.Enabled)
	}

	if config.Database.DSN != "postgres://test:test@localhost:5433/testdb" {
		t.Errorf("Expected Database.DSN to be 'postgres://test:test@localhost:5433/testdb', got '%s'", config.Database.DSN)
	}

	expectedMigrations := "/custom/path/migrations"
	if config.Database.Migrations != expectedMigrations {
		t.Errorf("Expected Database.Migrations to be '%s', got '%s'", expectedMigrations, config.Database.Migrations)
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "environment variable exists",
			key:          "TEST_KEY",
			defaultValue: "default",
			envValue:     "env_value",
			expected:     "env_value",
		},
		{
			name:         "environment variable does not exist",
			key:          "NONEXISTENT_KEY",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
		{
			name:         "empty environment variable",
			key:          "EMPTY_KEY",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up
			_ = os.Unsetenv(tt.key)

			// Set environment variable if provided
			if tt.envValue != "" {
				_ = os.Setenv(tt.key, tt.envValue)
				defer func() { _ = os.Unsetenv(tt.key) }()
			}

			result := getEnv(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestGetEnvAsInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int
		envValue     string
		expected     int
	}{
		{
			name:         "valid integer environment variable",
			key:          "TEST_INT_KEY",
			defaultValue: 100,
			envValue:     "200",
			expected:     200,
		},
		{
			name:         "environment variable does not exist",
			key:          "NONEXISTENT_INT_KEY",
			defaultValue: 100,
			envValue:     "",
			expected:     100,
		},
		{
			name:         "invalid integer environment variable",
			key:          "INVALID_INT_KEY",
			defaultValue: 100,
			envValue:     "not_a_number",
			expected:     100,
		},
		{
			name:         "empty environment variable",
			key:          "EMPTY_INT_KEY",
			defaultValue: 100,
			envValue:     "",
			expected:     100,
		},
		{
			name:         "zero value",
			key:          "ZERO_INT_KEY",
			defaultValue: 100,
			envValue:     "0",
			expected:     0,
		},
		{
			name:         "negative value",
			key:          "NEGATIVE_INT_KEY",
			defaultValue: 100,
			envValue:     "-50",
			expected:     -50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up
			_ = os.Unsetenv(tt.key)

			// Set environment variable if provided
			if tt.envValue != "" {
				_ = os.Setenv(tt.key, tt.envValue)
				defer func() { _ = os.Unsetenv(tt.key) }()
			}

			result := getEnvAsInt(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestAuthConfig_BooleanParsing(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"true value", "true", true},
		{"false value", "false", false},
		{"empty value", "", false},
		{"invalid value", "invalid", false},
		{"TRUE uppercase", "TRUE", false}, // Should be case sensitive
		{"1 value", "1", false},           // Should only accept "true"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Unsetenv("AUTH_ENABLED")
			_ = os.Unsetenv("DB_ENABLED")

			if tt.envValue != "" {
				_ = os.Setenv("AUTH_ENABLED", tt.envValue)
				_ = os.Setenv("DB_ENABLED", tt.envValue)
			}

			config := LoadConfig()

			if config.Auth.Enabled != tt.expected {
				t.Errorf("Expected Auth.Enabled to be %v, got %v", tt.expected, config.Auth.Enabled)
			}

			if config.Database.Enabled != tt.expected {
				t.Errorf("Expected Database.Enabled to be %v, got %v", tt.expected, config.Database.Enabled)
			}

			_ = os.Unsetenv("AUTH_ENABLED")
			_ = os.Unsetenv("DB_ENABLED")
		})
	}
}

// Helper function to clear environment variables used in tests
func clearEnvVars() {
	envVars := []string{
		"HTTP_PORT",
		"ALLOWED_ORIGIN",
		"AUTH_ENABLED",
		"AUTH_ISSUER_URL",
		"AUTH_CLIENT_ID",
		"AUTH_CLIENT_SECRET",
		"AUTH_JWKS_CACHE_TTL",
		"DB_ENABLED",
		"DB_DSN",
		"KO_DATA_PATH",
	}

	for _, env := range envVars {
		_ = os.Unsetenv(env)
	}
}
