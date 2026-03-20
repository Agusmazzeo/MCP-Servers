package config

import (
	sharedconfig "github.com/Agusmazzeo/MCP-Servers/shared/pkg/config"
	"github.com/joho/godotenv"
)

// Config holds the Allfunds MCP server configuration
type Config struct {
	GraphQLURL string
	Email      string
	Password   string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Try to load .env file (optional)
	_ = godotenv.Load()

	// Load configuration
	graphqlURL := sharedconfig.GetEnv("ALLFUNDS_GRAPHQL_URL", "https://app.allfunds.com/graphql")

	email, err := sharedconfig.RequireEnv("ALLFUNDS_EMAIL")
	if err != nil {
		return nil, err
	}

	password, err := sharedconfig.RequireEnv("ALLFUNDS_PASSWORD")
	if err != nil {
		return nil, err
	}

	return &Config{
		GraphQLURL: graphqlURL,
		Email:      email,
		Password:   password,
	}, nil
}
