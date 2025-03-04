// load application-specific configuration settings from environment variables

package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL    string
	ServerAddress  string
	AllowedOrigins string
	JWT            JWTConfig
}

type JWTConfig struct {
	SecretKey            string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
}

func LoadConfig() (*Config, error) {

	errEnv := godotenv.Load()
	if errEnv != nil {
		log.Fatal("Error loading .env file")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable not set")
	}

	serverAddress := os.Getenv("SERVER_ADDRESS")
	if serverAddress == "" {
		serverAddress = ":8080" // Default
	}

	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "*" // Default to allow all origins
	}

	// JWT Configuration
	secretKey := os.Getenv("JWT_SECRET_KEY")
	if secretKey == "" {
		return nil, fmt.Errorf("JWT_SECRET_KEY environment variable not set")
	}

	accessTokenDuration := 15 * time.Minute    // Default 15 minutes
	refreshTokenDuration := 7 * 24 * time.Hour // Default 7 days

	if envDuration := os.Getenv("JWT_ACCESS_TOKEN_DURATION"); envDuration != "" {
		duration, err := time.ParseDuration(envDuration)
		if err == nil {
			accessTokenDuration = duration
		}
	}

	if envDuration := os.Getenv("JWT_REFRESH_TOKEN_DURATION"); envDuration != "" {
		duration, err := time.ParseDuration(envDuration)
		if err == nil {
			refreshTokenDuration = duration
		}
	}

	return &Config{
		DatabaseURL:    databaseURL,
		ServerAddress:  serverAddress,
		AllowedOrigins: allowedOrigins,
		JWT: JWTConfig{
			SecretKey:            secretKey,
			AccessTokenDuration:  accessTokenDuration,
			RefreshTokenDuration: refreshTokenDuration,
		},
	}, nil
}
