package middleware

import (
	"chess-ws-go/internal/auth"
	"chess-ws-go/internal/config"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter for failed auth attempts
type AuthRateLimiter struct {
	visitors map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

func NewAuthRateLimiter(r rate.Limit, b int) *AuthRateLimiter {
	return &AuthRateLimiter{
		visitors: make(map[string]*rate.Limiter),
		rate:     r,
		burst:    b,
	}
}

func (rl *AuthRateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.visitors[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.visitors[ip] = limiter
	}

	return limiter
}

// AuthMiddleware creates a middleware for authentication
func AuthMiddleware(cfg *config.JWTConfig) gin.HandlerFunc {
	jwtMaker := auth.NewJWTMaker(cfg.SecretKey)
	rateLimiter := NewAuthRateLimiter(rate.Every(1*time.Minute), 5) // 5 attempts per minute

	return func(c *gin.Context) {
		// Rate limiting check
		limiter := rateLimiter.getLimiter(c.ClientIP())
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "too many failed authentication attempts",
			})
			c.Abort()
			return
		}

		token := extractToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "no authorization token provided",
			})
			c.Abort()
			return
		}

		claims, err := jwtMaker.VerifyToken(token)
		if err != nil {
			status := http.StatusUnauthorized
			if err == auth.ErrExpiredToken {
				status = http.StatusUnauthorized
			}
			c.JSON(status, gin.H{
				"error": err.Error(),
			})
			c.Abort()
			return
		}

		// Store claims in context for later use
		c.Set("claims", claims)
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("permissions", claims.Permissions)

		c.Next()
	}
}

// extractToken extracts the JWT token from various sources
func extractToken(c *gin.Context) string {
	// 1. Try Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}

	// 2. Try WebSocket protocol header (for WS connections)
	if c.GetHeader("Upgrade") == "websocket" {
		protocols := c.GetHeader("Sec-WebSocket-Protocol")
		if protocols != "" {
			parts := strings.Split(protocols, ", ")
			for _, part := range parts {
				if strings.HasPrefix(part, "token.") {
					return strings.TrimPrefix(part, "token.")
				}
			}
		}
	}

	// 3. Try query parameter (less secure, but sometimes necessary for WebSocket)
	token := c.Query("token")
	if token != "" {
		return token
	}

	return ""
}

// RequirePermission middleware checks if the user has the required permission
func RequirePermission(permission auth.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get("claims")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "no authentication claims found",
			})
			c.Abort()
			return
		}

		if authClaims, ok := claims.(*auth.Claims); ok {
			if !authClaims.HasPermission(permission) {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "insufficient permissions",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// RequireRole middleware checks if the user has the required role
func RequireRole(role auth.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get("claims")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "no authentication claims found",
			})
			c.Abort()
			return
		}

		if authClaims, ok := claims.(*auth.Claims); ok {
			if authClaims.Role != role && !authClaims.IsAdmin() {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "insufficient role",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}
