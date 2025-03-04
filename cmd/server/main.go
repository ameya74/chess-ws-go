package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"chess-ws-go/internal/auth"
	"chess-ws-go/internal/config"
	"chess-ws-go/internal/handlers"
	"chess-ws-go/internal/middleware"
	"chess-ws-go/internal/platform"
	"chess-ws-go/internal/repositories"
	"chess-ws-go/internal/services"
	"chess-ws-go/internal/stats"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func NewServer(
	cfg *config.Config,
	messageService *services.MessageService,
	gameService *services.GameService,
	userRepo repositories.UserRepository,
	authService *services.AuthService,
	db *sql.DB,
) http.Handler {

	router := gin.Default()

	// Public routes
	router.GET("/health", handlers.NewHealthHandler(db).HealthCheck)

	// Auth routes
	authHandler := handlers.NewAuthHandler(authService)
	authGroup := router.Group("/auth")
	{
		authGroup.GET("/status", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "Authentication service running"})
		})
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/refresh", authHandler.RefreshToken)
		authGroup.POST("/register", authHandler.Register)
		authGroup.GET("/verify", authHandler.VerifyEmail)
	}

	// Protected routes
	protected := router.Group("")
	protected.Use(middleware.AuthMiddleware(&cfg.JWT))
	{
		// WebSocket route with authentication
		wsHandler := handlers.NewWebSocketHandler(messageService, gameService, cfg)
		protected.GET("/ws", func(c *gin.Context) {
			// Extract user info from context
			userID := c.GetString("user_id")
			username := c.GetString("username")

			// Add user info to request context for WebSocket handler
			c.Request = c.Request.WithContext(
				context.WithValue(c.Request.Context(), "user_id", userID),
			)
			c.Request = c.Request.WithContext(
				context.WithValue(c.Request.Context(), "username", username),
			)

			wsHandler.UpgradeHandler(c.Writer, c.Request)
		})

		// Game management routes (will be implemented later)
		gameGroup := protected.Group("/game")
		{
			// These routes will require CREATE_GAME permission
			gameGroup.Use(middleware.RequirePermission(auth.PermissionCreateGame))
			// gameGroup.POST("/create", gameHandler.CreateGame)
		}

		// Admin routes (will be implemented later)
		adminGroup := protected.Group("/admin")
		{
			// These routes will require ADMIN role
			adminGroup.Use(middleware.RequireRole(auth.RoleAdmin))
			// adminGroup.GET("/stats", adminHandler.GetStats)
		}
	}

	// Middleware
	var handler http.Handler = router
	handler = middleware.LoggingMiddleware(handler)
	handler = middleware.CorsMiddleware(cfg.AllowedOrigins)(handler)

	return handler
}

func main() {
	config, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Platform initialization (Database connection)
	db, err := platform.ConnectDB(config.DatabaseURL)
	if err != nil {
		log.Fatal("Error connecting to database:", err)
	}
	defer db.Close() // Close the database connection when the server exits

	// Initialize repositories
	dbx := sqlx.NewDb(db, "postgres") // Assuming PostgreSQL, adjust if using a different database
	userRepo := repositories.NewSQLUserRepository(dbx)

	// Initialize services
	gameService := services.NewGameService()
	messageService := services.NewMessageService(gameService)
	authService := services.NewAuthService(userRepo, &config.JWT)

	// Initialize stats collector
	statsCollector := stats.NewCollector(
		30*time.Second, // Collect stats every 30 seconds
		gameService.GetActiveGamesCount,
		messageService.GetActiveConnectionsCount,
	)
	statsCollector.Start()

	// Create server
	server := NewServer(config, messageService, gameService, userRepo, authService, db)

	// Configure HTTP server
	srv := &http.Server{
		Addr:    config.ServerAddress,
		Handler: server,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on %s", config.ServerAddress)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited properly")
}
