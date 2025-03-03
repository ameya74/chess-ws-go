package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	db *sql.DB
}

func NewHealthHandler(db *sql.DB) *HealthHandler {
	return &HealthHandler{
		db: db,
	}
}

// HealthCheck handles the health check endpoint
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	isHealthy := true
	dbStatus := "up"
	statusCode := http.StatusOK

	// Check database connection
	if err := h.db.Ping(); err != nil {
		isHealthy = false
		dbStatus = "down"
		statusCode = http.StatusServiceUnavailable
	}

	response := gin.H{
		"status":    map[bool]string{true: "healthy", false: "unhealthy"}[isHealthy],
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"dependencies": gin.H{
			"database": dbStatus,
		},
	}

	// Handle Prometheus format if requested
	if c.GetHeader("Accept") == "text/plain" {
		c.String(statusCode, "health_status %d\ndatabase_status %d\n",
			map[bool]int{true: 1, false: 0}[isHealthy],
			map[string]int{"up": 1, "down": 0}[dbStatus])
		return
	}

	c.JSON(statusCode, response)
}
