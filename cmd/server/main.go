package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"chess-ws-go/internal/config"
	"chess-ws-go/internal/handlers"
	"chess-ws-go/internal/platform"
	"chess-ws-go/internal/services"
	"github.com/gin-gonic/gin"
)

func NewServer(
    cfg *config.Config,
    messageService *services.MessageService,
) http.Handler {

    router := gin.Default()

    // WebSocket route
    wsHandler := handlers.NewWebSocketHandler(messageService, cfg)
	router.GET("/ws", func(c *gin.Context) {
		wsHandler.UpgradeHandler(c.Writer, c.Request)
	})

    // HTTP routes (if needed)
    // router.HandleFunc("/health", healthCheckHandler)

    // Middleware
    var handler http.Handler = router
    // handler = loggingMiddleware(handler) 
    // handler = authMiddleware(handler)    
    // handler = corsMiddleware(handler)    

    return handler
}

func main ()  {
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


}