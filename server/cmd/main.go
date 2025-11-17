package main

import (
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"wirety/internal/adapters/api"
	"wirety/internal/adapters/api/middleware"
	"wirety/internal/adapters/db/memory"
	"wirety/internal/application/auth"
	"wirety/internal/application/ipam"
	"wirety/internal/application/network"
	"wirety/internal/config"
)

//	@title			Wirety Server API
//	@version		1.0
//	@description	WireGuard network management API with hexagonal architecture
//	@termsOfService	http://swagger.io/terms/

//	@contact.name	API Support
//	@contact.url	http://www.wirety.io/support
//	@contact.email	support@wirety.io

//	@license.name	MIT
//	@license.url	https://opensource.org/licenses/MIT

//	@host		localhost:8080
//	@BasePath	/api/v1

//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Type "Bearer" followed by a space and JWT token.

func main() {
	// Configure zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Load configuration
	cfg := config.LoadConfig()

	log.Info().
		Str("http_port", cfg.HTTPPort).
		Bool("auth_enabled", cfg.Auth.Enabled).
		Str("issuer_url", cfg.Auth.IssuerURL).
		Msg("Starting Wirety server")

	// Initialize repositories (in-memory for now)
	repo := memory.NewRepository()
	userRepo := memory.NewUserRepository()

	// Initialize services
	networkService := network.NewService(repo)
	ipamService := ipam.NewService(repo)

	var authService *auth.Service
	if cfg.Auth.Enabled {
		authService = auth.NewService(&cfg.Auth, userRepo)
		log.Info().Msg("OIDC authentication enabled")
	} else {
		log.Warn().Msg("Authentication disabled - running in open mode with admin permissions")
	}

	// Initialize API handler
	handler := api.NewHandler(networkService, ipamService, userRepo)

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Configure CORS to allow all origins
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	// Setup authentication middleware
	authMiddleware := middleware.AuthMiddleware(authService, &cfg.Auth)
	requireAdmin := middleware.RequireAdmin()
	requireNetworkAccess := middleware.RequireNetworkAccess()

	// Register routes with middleware
	handler.RegisterRoutes(r, authMiddleware, requireAdmin, requireNetworkAccess)

	// Start server
	log.Info().Msgf("Starting Wirety server on port %s", cfg.HTTPPort)
	if err := r.Run(":" + cfg.HTTPPort); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}
