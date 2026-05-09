package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds the application configuration
type Config struct {
	HTTPPort    string     `json:"http_port"`
	CORSOrigins []string   `json:"cors_origins"` // CORS_ORIGIN env var — comma-separated list of allowed origins (use * only in development)
	AuditLog    bool       `json:"audit_log"`    // AUDIT_LOG env var — emit JSON audit events to stdout
	LogLevel    string     `json:"log_level"`    // LOG_LEVEL env var — trace|debug|info|warn|error|fatal (default: info)
	LogFormat   string     `json:"log_format"`   // LOG_FORMAT env var — text|json (default: text)
	Auth        AuthConfig `json:"auth"`
	Database    DBConfig   `json:"database"`
}

// AuthConfig holds authentication-related configuration
type AuthConfig struct {
	Enabled       bool   `json:"enabled"`        // Enable OIDC authentication
	IssuerURL     string `json:"issuer_url"`     // OIDC provider URL (e.g., https://keycloak.example.com/realms/wirety)
	ClientID      string `json:"client_id"`      // OIDC client ID
	ClientSecret  string `json:"client_secret"`  // OIDC client secret
	JWKSCacheTTL  int    `json:"jwks_cache_ttl"` // JWKS cache duration in seconds (default: 3600)
	AdminPassword string `json:"-"`              // Admin password for simple auth mode (AUTH_ENABLED=false)
	CookieSecure  bool   `json:"cookie_secure"`  // Set Secure flag on session cookie (default: true)

	// Group-based access control (all optional)
	EmailClaim  string `json:"email_claim"`  // AUTH_EMAIL_CLAIM — JWT claim to use as email (default: "email")
	GroupsClaim string `json:"groups_claim"` // AUTH_GROUPS_CLAIM — JWT claim that carries group memberships
	AdminGroup  string `json:"admin_group"`  // AUTH_ADMIN_GROUP — comma-separated groups granting administrator role
	UserGroup   string `json:"user_group"`   // AUTH_USER_GROUP — comma-separated groups required for regular user login
}

// Validate returns an error for invalid auth configuration combinations.
func (a *AuthConfig) Validate() error {
	if a.UserGroup != "" && a.AdminGroup == "" {
		return fmt.Errorf("AUTH_USER_GROUP is set but AUTH_ADMIN_GROUP is not — without an admin group no administrator can ever be created; either set AUTH_ADMIN_GROUP or remove AUTH_USER_GROUP")
	}
	return nil
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		HTTPPort:    getEnv("HTTP_PORT", "8080"),
		CORSOrigins: getCORSOrigins(),
		AuditLog:    getEnv("AUDIT_LOG", "false") == "true",
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		LogFormat:   getEnv("LOG_FORMAT", "text"),
		Auth: AuthConfig{
			Enabled:       getEnv("AUTH_ENABLED", "false") == "true",
			IssuerURL:     getEnv("AUTH_ISSUER_URL", ""),
			ClientID:      getEnv("AUTH_CLIENT_ID", ""),
			ClientSecret:  getEnv("AUTH_CLIENT_SECRET", ""),
			JWKSCacheTTL:  getEnvAsInt("AUTH_JWKS_CACHE_TTL", 3600),
			AdminPassword: getEnv("AUTH_PASSWORD", ""),
			CookieSecure:  getEnv("COOKIE_SECURE", "true") != "false",
			EmailClaim:    getEnv("AUTH_EMAIL_CLAIM", ""),
			GroupsClaim:   getEnv("AUTH_GROUPS_CLAIM", ""),
			AdminGroup:    getEnv("AUTH_ADMIN_GROUP", ""),
			UserGroup:     getEnv("AUTH_USER_GROUP", ""),
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

// getCORSOrigins reads CORS_ORIGIN (or legacy ALLOWED_ORIGIN) and returns a
// slice of allowed origins.  Multiple origins can be specified as a
// comma-separated list, e.g. "https://app.example.com,https://admin.example.com".
func getCORSOrigins() []string {
	raw := os.Getenv("CORS_ORIGIN")
	if raw == "" {
		raw = os.Getenv("ALLOWED_ORIGIN")
	}
	if raw == "" {
		return []string{"*"}
	}
	var origins []string
	for _, o := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	if len(origins) == 0 {
		return []string{"*"}
	}
	return origins
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
