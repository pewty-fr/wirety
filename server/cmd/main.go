package main

import (
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"wirety/internal/adapters/api"
	"wirety/internal/adapters/db/memory"
	"wirety/internal/application/ipam"
	"wirety/internal/application/network"
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

	// Get HTTP port from environment variable
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize repository (in-memory for now)
	repo := memory.NewRepository()

	// Initialize services
	networkService := network.NewService(repo)
	ipamService := ipam.NewService(repo)

	// Initialize API handler
	handler := api.NewHandler(networkService, ipamService)

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

	// Register routes
	handler.RegisterRoutes(r)

	// Start server
	log.Info().Msgf("Starting Wirety server on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}
