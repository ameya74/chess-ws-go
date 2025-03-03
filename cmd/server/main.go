package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"chess-ws-go/internal/config"
	"chess-ws-go/internal/handlers"
	"chess-ws-go/internal/middleware"
	"chess-ws-go/internal/platform"
	"chess-ws-go/internal/services"
	"chess-ws-go/internal/stats"

	"github.com/gin-gonic/gin"
)

func NewServer(
	cfg *config.Config,
	messageService *services.MessageService,
	gameService *services.GameService,
	db *sql.DB,
) http.Handler {

	router := gin.Default()

	// WebSocket route
	wsHandler := handlers.NewWebSocketHandler(messageService, gameService, cfg)
	router.GET("/ws", func(c *gin.Context) {
		wsHandler.UpgradeHandler(c.Writer, c.Request)
	})

	// Health check route
	healthHandler := handlers.NewHealthHandler(db)
	router.GET("/health", healthHandler.HealthCheck)

	// Middleware
	var handler http.Handler = router
	handler = middleware.LoggingMiddleware(handler)
	// handler = authMiddleware(handler)
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

	// Initialize services
	gameService := services.NewGameService()
	messageService := services.NewMessageService(gameService)

	// Initialize stats collector
	statsCollector := stats.NewCollector(
		30*time.Second, // Collect stats every 30 seconds
		gameService.GetActiveGamesCount,
		messageService.GetActiveConnectionsCount,
	)
	statsCollector.Start()

	// Create server
	server := NewServer(config, messageService, gameService, db)

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
