package main

import (
	"context"
	"database/sql"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"wirety/internal/adapters/api"
	"wirety/internal/adapters/api/middleware"
	"wirety/internal/adapters/db/memory"
	pgrepo "wirety/internal/adapters/db/postgres"
	appauth "wirety/internal/application/auth"
	"wirety/internal/application/ipam"
	appnetwork "wirety/internal/application/network"
	"wirety/internal/config"
	domainauth "wirety/internal/domain/auth"
	domainipam "wirety/internal/domain/ipam"
	domainnetwork "wirety/internal/domain/network"
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

	// Initialize repositories (choose Postgres or in-memory)
	var networkRepo domainnetwork.Repository
	var ipamRepo domainipam.Repository
	var userRepo domainauth.Repository

	if cfg.Database.Enabled {
		log.Info().Str("dsn", cfg.Database.DSN).Msg("Initializing Postgres repositories")
		db, err := sql.Open("postgres", cfg.Database.DSN)
		if err != nil {
			log.Fatal().Err(err).Msg("open postgres")
		}
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(30 * time.Minute)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			log.Fatal().Err(err).Msg("ping postgres")
		}
		// migrations
		if err := pgrepo.RunMigrations(ctx, db, cfg.Database.Migrations); err != nil {
			log.Fatal().Err(err).Msg("run migrations")
		}
		networkRepo = pgrepo.NewNetworkRepository(db)
		var ipErr error
		ipamRepo, ipErr = pgrepo.NewIPAMRepository(ctx, db)
		if ipErr != nil {
			log.Fatal().Err(ipErr).Msg("init ipam repository")
		}
		userRepo = pgrepo.NewUserRepository(db)
	} else {
		log.Warn().Msg("DB disabled - using in-memory repositories")
		networkRepo = memory.NewRepository()
		ipamRepo = memory.NewIPAMRepository(context.Background())
		userRepo = memory.NewUserRepository()
	}

	// Initialize services
	networkService := appnetwork.NewService(networkRepo, ipamRepo)
	ipamService := ipam.NewService(ipamRepo)

	var authService *appauth.Service
	if cfg.Auth.Enabled {
		authService = appauth.NewService(&cfg.Auth, userRepo)
		log.Info().Msg("OIDC authentication enabled")
	} else {
		log.Warn().Msg("Authentication disabled - running in open mode with admin permissions")
	}

	// Initialize API handler
	handler := api.NewHandler(networkService, ipamService, authService, userRepo, &cfg.Auth)

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Configure CORS to allow all origins
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.AllowedOrigin},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	// Setup authentication middleware
	authMiddleware := middleware.AuthMiddleware(authService, userRepo, &cfg.Auth)
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

// ctxWithLog creates a basic context for db operations (placeholder for structured contexts)
