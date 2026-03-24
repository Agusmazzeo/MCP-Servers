package config

import (
	"fmt"
	"os"
)

// Config holds the 1816 API configuration
type Config struct {
	APIURL       string
	TokenURL     string
	ClientID     string
	RefreshToken string
	Email        string
	Password     string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	apiURL := os.Getenv("API_1816_URL")
	if apiURL == "" {
		apiURL = "https://api-service.1816.com.ar/v1"
	}

	tokenURL := os.Getenv("TOKEN_1816_URL")
	if tokenURL == "" {
		tokenURL = "https://auth.1816.com.ar/realms/milochorep/protocol/openid-connect/token"
	}

	clientID := os.Getenv("CLIENT_ID_1816")
	if clientID == "" {
		clientID = "user-console-prod"
	}

	refreshToken := os.Getenv("REFRESH_TOKEN_1816")
	email := os.Getenv("EMAIL_1816")
	password := os.Getenv("PASSWORD_1816")

	// For stdio mode, require refresh token
	// For HTTP mode, refresh token is provided via OAuth

	return &Config{
		APIURL:       apiURL,
		TokenURL:     tokenURL,
		ClientID:     clientID,
		RefreshToken: refreshToken,
		Email:        email,
		Password:     password,
	}, nil
}

// Validate ensures required fields are set
func (c *Config) Validate() error {
	if c.APIURL == "" {
		return fmt.Errorf("API_1816_URL is required")
	}
	if c.TokenURL == "" {
		return fmt.Errorf("TOKEN_1816_URL is required")
	}
	if c.ClientID == "" {
		return fmt.Errorf("CLIENT_ID_1816 is required")
	}
	return nil
}
