package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the OMS configuration
type Config struct {
	BaseURL          string
	OMSIP            string
	Username         string
	Password         string
	DefaultAdvisorID int
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	baseURL := os.Getenv("OMS_BASE_URL")
	if baseURL == "" {
		baseURL = "https://oms.cdfbox.com"
	}

	omsIP := os.Getenv("OMS_IP")
	if omsIP == "" {
		omsIP = "52.184.149.124"
	}

	username := os.Getenv("OMS_USERNAME")
	password := os.Getenv("OMS_PASSWORD")

	defaultAdvisorID := 136
	if advisorIDStr := os.Getenv("OMS_DEFAULT_ADVISOR_ID"); advisorIDStr != "" {
		if id, err := strconv.Atoi(advisorIDStr); err == nil {
			defaultAdvisorID = id
		}
	}

	return &Config{
		BaseURL:          baseURL,
		OMSIP:            omsIP,
		Username:         username,
		Password:         password,
		DefaultAdvisorID: defaultAdvisorID,
	}, nil
}

// Validate ensures required fields are set
func (c *Config) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("OMS_BASE_URL is required")
	}
	if c.OMSIP == "" {
		return fmt.Errorf("OMS_IP is required")
	}
	return nil
}
