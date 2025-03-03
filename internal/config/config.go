// load application-specific configuration settings from environment variables

package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL    string
	ServerAddress  string
	AllowedOrigins string
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

	return &Config{
		DatabaseURL:    databaseURL,
		ServerAddress:  serverAddress,
		AllowedOrigins: allowedOrigins,
	}, nil
}
