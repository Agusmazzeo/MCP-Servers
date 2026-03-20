package config

import (
	"os"
	"strconv"
	"time"

	sharedconfig "github.com/Criteria/MCP-Servers/shared/pkg/config"
	zenconfig "github.com/Agusmazzeo/ZenCRM/app/config"
	"github.com/joho/godotenv"
)

// Config holds the ZenCRM MCP server configuration
type Config struct {
	APIBaseURL      string
	FrontendBaseURL string
	AuthToken       string
	Timeout         time.Duration
	Observability   zenconfig.ObservabilityConfig
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Try to load .env file (optional)
	_ = godotenv.Load()

	// Load required configuration
	apiBaseURL, err := sharedconfig.RequireEnv("API_BASE_URL")
	if err != nil {
		return nil, err
	}

	// Authentication token (optional, only for stdio mode)
	authToken := os.Getenv("JWT_TOKEN")

	// Load optional configuration
	frontendBaseURL := os.Getenv("FRONTEND_BASE_URL")

	// Parse timeout (default 60 seconds)
	timeout := 60 * time.Second
	if timeoutStr := os.Getenv("HTTP_TIMEOUT"); timeoutStr != "" {
		if timeoutSeconds, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = time.Duration(timeoutSeconds) * time.Second
		}
	}

	// Load observability configuration
	observability := loadObservabilityConfig()

	return &Config{
		APIBaseURL:      apiBaseURL,
		FrontendBaseURL: frontendBaseURL,
		AuthToken:       authToken,
		Timeout:         timeout,
		Observability:   observability,
	}, nil
}

// loadObservabilityConfig loads observability configuration from environment
func loadObservabilityConfig() zenconfig.ObservabilityConfig {
	environment := sharedconfig.GetEnv("ENVIRONMENT", "development")
	logLevel := sharedconfig.GetEnv("LOG_LEVEL", "info")

	// OTLP configuration (optional - for Grafana Cloud)
	otlpEnabled := os.Getenv("OTLP_ENABLED") == "true"
	otlpEndpoint := os.Getenv("OTLP_ENDPOINT")
	otlpInstanceID := os.Getenv("OTLP_INSTANCE_ID")
	otlpAPIKey := os.Getenv("OTLP_API_KEY")

	// Tracing configuration
	tracingEnabled := os.Getenv("TRACING_ENABLED") == "true"
	tracingSampleRate := 1.0
	if sampleRateStr := os.Getenv("TRACING_SAMPLE_RATE"); sampleRateStr != "" {
		if rate, err := strconv.ParseFloat(sampleRateStr, 64); err == nil {
			tracingSampleRate = rate
		}
	}

	return zenconfig.ObservabilityConfig{
		Environment:       environment,
		Service:           "zencrm-mcp",
		LogLevel:          logLevel,
		OTLPEnabled:       otlpEnabled,
		OTLPEndpoint:      otlpEndpoint,
		OTLPInstanceID:    otlpInstanceID,
		OTLPAPIKey:        otlpAPIKey,
		TracingEnabled:    tracingEnabled,
		TracingSampleRate: tracingSampleRate,
		Labels: map[string]string{
			"version": "1.0.0",
			"service": "zencrm-mcp",
		},
	}
}
