package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the application configuration
type Config struct {
	HTTPPort string     `json:"http_port"`
	Auth     AuthConfig `json:"auth"`
	Database DBConfig   `json:"database"`
}

// AuthConfig holds authentication-related configuration
type AuthConfig struct {
	Enabled      bool   `json:"enabled"`        // Enable OIDC authentication
	IssuerURL    string `json:"issuer_url"`     // OIDC provider URL (e.g., https://keycloak.example.com/realms/wirety)
	ClientID     string `json:"client_id"`      // OIDC client ID
	ClientSecret string `json:"client_secret"`  // OIDC client secret
	JWKSCacheTTL int    `json:"jwks_cache_ttl"` // JWKS cache duration in seconds (default: 3600)
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		HTTPPort: getEnv("HTTP_PORT", "8080"),
		Auth: AuthConfig{
			Enabled:      getEnv("AUTH_ENABLED", "false") == "true",
			IssuerURL:    getEnv("AUTH_ISSUER_URL", ""),
			ClientID:     getEnv("AUTH_CLIENT_ID", ""),
			ClientSecret: getEnv("AUTH_CLIENT_SECRET", ""),
			JWKSCacheTTL: getEnvAsInt("AUTH_JWKS_CACHE_TTL", 3600),
		},
		Database: DBConfig{
			Enabled:    getEnv("DB_ENABLED", "false") == "true",
			DSN:        getEnv("DB_DSN", "postgres://wirety:wirety@localhost:5432/wirety?sslmode=disable"),
			Migrations: fmt.Sprintf("%s/migrations", getEnv("KO_DATA_PATH", "kodata")),
		},
	}
}

// DBConfig holds database configuration
type DBConfig struct {
	Enabled    bool   `json:"enabled"`
	DSN        string `json:"dsn"`
	Migrations string `json:"migrations"`
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}
