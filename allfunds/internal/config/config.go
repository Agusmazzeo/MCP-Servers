package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds the Allfunds MCP server configuration
type Config struct {
	GraphQLURL string
	Email      string
	Password   string
}

// BaseConfig holds common MCP server configuration
type BaseConfig struct {
	ServiceName string
	Version     string
	Timeout     time.Duration
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Try to load .env file (optional)
	_ = godotenv.Load()

	// Load configuration
	graphqlURL := GetEnv("ALLFUNDS_GRAPHQL_URL", "https://app.allfunds.com/graphql")

	email, err := RequireEnv("ALLFUNDS_EMAIL")
	if err != nil {
		return nil, err
	}

	password, err := RequireEnv("ALLFUNDS_PASSWORD")
	if err != nil {
		return nil, err
	}

	return &Config{
		GraphQLURL: graphqlURL,
		Email:      email,
		Password:   password,
	}, nil
}

// LoadBaseConfig loads base configuration from environment variables
func LoadBaseConfig(serviceName, version string) (*BaseConfig, error) {
	// Try to load .env file (optional, ignore error if not found)
	_ = godotenv.Load()

	// Parse timeout (default 60 seconds)
	timeout := 60 * time.Second
	if timeoutStr := os.Getenv("HTTP_TIMEOUT"); timeoutStr != "" {
		if timeoutSeconds, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = time.Duration(timeoutSeconds) * time.Second
		}
	}

	return &BaseConfig{
		ServiceName: serviceName,
		Version:     version,
		Timeout:     timeout,
	}, nil
}

// GetEnv gets an environment variable or returns a default value
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// RequireEnv gets an environment variable or returns an error if not set
func RequireEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%s environment variable is required", key)
	}
	return value, nil
}
