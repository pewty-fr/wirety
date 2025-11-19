package main

import (
	"context"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"wirety/internal/adapters/api"
	"wirety/internal/adapters/api/middleware"
	"wirety/internal/adapters/db/memory"
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

func main() {
	// Configure zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Load configuration (simplified for demo)
	cfg := config.LoadConfig()

	// Get HTTP port from environment variable or config
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = cfg.HTTPPort
		if port == "" {
			port = "8080"
		}
	}

	// Initialize in-memory repositories
	networkRepo := memory.NewRepository()
	ipamRepo := memory.NewIPAMRepository(context.Background())
	userRepo := memory.NewUserRepository()

	// Initialize services
	networkService := network.NewService(networkRepo, ipamRepo)
	ipamService := ipam.NewService(ipamRepo)

	// Initialize API handler
	handler := api.NewHandler(networkService, ipamService, userRepo)

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Configure CORS
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	// Setup minimal middleware (no auth in demo)
	authMiddleware := middleware.AuthMiddleware(nil, &cfg.Auth)
	requireAdmin := middleware.RequireAdmin()
	requireNetworkAccess := middleware.RequireNetworkAccess()

	// Register routes
	handler.RegisterRoutes(r, authMiddleware, requireAdmin, requireNetworkAccess)

	// Start server
	log.Info().Msgf("Starting Wirety server on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}
