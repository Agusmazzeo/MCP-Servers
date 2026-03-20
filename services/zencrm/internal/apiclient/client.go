package apiclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Criteria/MCP-Servers/services/zencrm/internal/config"
)

const MaxResponseSize = 80000 // 80KB limit for responses

// Client is the HTTP client for the ZenCRM API
type Client struct {
	config     *config.Config
	httpClient *http.Client
	routes     map[string]RouteConfig
	exportData *ExportData

	// Session token management
	sessionToken string
	tokenMu      sync.RWMutex
}

// ExportData holds temporary export file data
type ExportData struct {
	Filename    string
	ContentType string
	Data        string
}

// ExecuteResult represents the result of a tool execution
type ExecuteResult struct {
	Success      bool
	Data         interface{}
	Metadata     map[string]interface{}
	IsFatalError bool
	Error        string
}

// NewClient creates a new API client
func NewClient(cfg *config.Config) *Client {
	client := &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90,
				TLSNextProto:        make(map[string]func(authority string, c *tls.Conn) http.RoundTripper), // Disable HTTP/2
			},
		},
		sessionToken: cfg.AuthToken, // Initialize with config token if provided
	}
	client.initializeRoutes()
	return client
}

// NewClientWithToken creates a new API client with a JWT token
// Used by HTTP transport to create per-request clients
func NewClientWithToken(jwtToken string) *Client {
	// Parse timeout from environment (default 60 seconds)
	timeout := 60 * time.Second
	if timeoutStr := os.Getenv("HTTP_TIMEOUT"); timeoutStr != "" {
		if timeoutSeconds, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = time.Duration(timeoutSeconds) * time.Second
		}
	}

	// Create default config for HTTP transport
	cfg := &config.Config{
		APIBaseURL: getEnvOrDefault("API_BASE_URL", "http://localhost:8000"),
		AuthToken:  jwtToken,
		Timeout:    timeout,
	}

	client := &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90,
				TLSNextProto:        make(map[string]func(authority string, c *tls.Conn) http.RoundTripper), // Disable HTTP/2
			},
		},
		sessionToken: jwtToken,
	}
	client.initializeRoutes()
	return client
}

// Execute executes a tool by name with the given input
func (c *Client) Execute(ctx context.Context, toolName string, input map[string]interface{}) (*ExecuteResult, error) {
	// Look up route configuration
	route, exists := c.routes[toolName]
	if !exists {
		return &ExecuteResult{
			Success:      false,
			IsFatalError: false,
			Error:        fmt.Sprintf("unknown tool: %s", toolName),
		}, nil
	}

	// Build request path
	path := route.PathFunc(input)

	// Build request body (for POST/PUT)
	var body map[string]interface{}
	if route.BodyFunc != nil {
		body = route.BodyFunc(input)
	}

	// Determine if we should use TOON format
	useToon := route.Method == "GET" && !isMetadataEndpoint(toolName)

	// Execute the request
	data, err := c.doRequest(ctx, route.Method, path, body, useToon)
	if err != nil {
		// Enhance error message
		enhancedError := enhanceErrorMessage(0, err.Error(), route.Method, path, body)
		return &ExecuteResult{
			Success:      false,
			IsFatalError: isFatalError(enhancedError),
			Error:        enhancedError,
		}, nil
	}

	fmt.Fprintf(os.Stderr, "[DEBUG Execute] Data type after doRequest: %T, data: %v\n", data, data)

	// For export tools, handle specially
	if strings.HasPrefix(toolName, "generate_") && strings.HasSuffix(toolName, "_export_url") {
		// data should be the export content
		if dataStr, ok := data.(string); ok {
			c.exportData = &ExportData{
				Filename:    "export.csv", // Could extract from headers
				ContentType: "text/csv",
				Data:        dataStr,
			}
			return &ExecuteResult{
				Success: true,
				Data: map[string]interface{}{
					"success":  true,
					"filename": c.exportData.Filename,
					"size":     len(c.exportData.Data),
					"message":  fmt.Sprintf("Export generated: %s (%d bytes)", c.exportData.Filename, len(c.exportData.Data)),
				},
				Metadata: make(map[string]interface{}),
			}, nil
		}
	}

	// Truncate large responses
	dataStr := fmt.Sprintf("%v", data)
	if len(dataStr) > MaxResponseSize {
		data = map[string]interface{}{
			"truncated": true,
			"message":   "Response truncated due to size. Use export tools for full data.",
			"size":      len(dataStr),
			"preview":   dataStr[:MaxResponseSize],
		}
	}

	return &ExecuteResult{
		Success:  true,
		Data:     data,
		Metadata: make(map[string]interface{}),
	}, nil
}

// doRequest performs the HTTP request
func (c *Client) doRequest(ctx context.Context, method, path string, body map[string]interface{}, useToon bool) (interface{}, error) {
	url := c.config.APIBaseURL + path
	fmt.Fprintf(os.Stderr, "[DEBUG doRequest] %s %s (useToon: %v)\n", method, url, useToon)

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Use session token with thread-safe read
	c.tokenMu.RLock()
	token := c.sessionToken
	c.tokenMu.RUnlock()

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Use TOON format for GET requests (except metadata endpoints)
	if useToon {
		req.Header.Set("x-toon-parse", "true")
	}

	// Use count-only optimization for count queries
	if method == "GET" && isCountOnlyRequest(path) {
		req.Header.Set("x-count-only", "true")
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Retry GET requests once on network errors
		if method == "GET" && strings.Contains(err.Error(), "connection") {
			resp, err = c.httpClient.Do(req)
			if err != nil {
				return nil, fmt.Errorf("request failed after retry: %w", err)
			}
		} else {
			return nil, fmt.Errorf("request failed: %w", err)
		}
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		errorDetails := string(respBody)

		// Special handling for 401 Unauthorized
		if resp.StatusCode == 401 {
			return nil, fmt.Errorf("authentication required: please login first using the login tool")
		}

		enhancedError := enhanceErrorMessage(resp.StatusCode, errorDetails, method, path, body)
		return nil, fmt.Errorf("%s", enhancedError)
	}

	// Parse response based on content type
	contentType := resp.Header.Get("Content-Type")
	fmt.Fprintf(os.Stderr, "[DEBUG doRequest] Content-Type: %q, Body length: %d\n", contentType, len(respBody))

	if strings.Contains(contentType, "application/toon") {
		// Return TOON format as-is for Claude to parse
		toonData := string(respBody)
		preview := toonData
		if len(preview) > 200 {
			preview = preview[:200]
		}
		fmt.Fprintf(os.Stderr, "[DEBUG doRequest] Returning TOON data, length: %d, preview: %s...\n", len(toonData), preview)
		return toonData, nil
	}

	// For export endpoints, return raw data
	if strings.Contains(path, "/export") {
		return string(respBody), nil
	}

	// Parse JSON response
	var result interface{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &result); err != nil {
			// If JSON parsing fails, return as string
			return string(respBody), nil
		}
	}

	return result, nil
}

// GetLastExportData returns the last export data
func (c *Client) GetLastExportData() *ExportData {
	return c.exportData
}

// LoginRequest represents the login request payload
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse represents the login response payload
type LoginResponse struct {
	Token string                 `json:"token"`
	User  map[string]interface{} `json:"user"`
}

// Login authenticates with the CRM API and stores the JWT token
func (c *Client) Login(ctx context.Context, email, password string) error {
	// Build login request
	loginReq := LoginRequest{
		Email:    email,
		Password: password,
	}

	jsonData, err := json.Marshal(loginReq)
	if err != nil {
		return fmt.Errorf("failed to marshal login request: %w", err)
	}

	// Make login request
	url := c.config.APIBaseURL + "/v1/auth/login"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read login response: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		return fmt.Errorf("login failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse login response
	var loginResp LoginResponse
	if err := json.Unmarshal(respBody, &loginResp); err != nil {
		return fmt.Errorf("failed to parse login response: %w", err)
	}

	if loginResp.Token == "" {
		return fmt.Errorf("login response did not contain a token")
	}

	// Store the token
	c.tokenMu.Lock()
	c.sessionToken = loginResp.Token
	c.tokenMu.Unlock()

	return nil
}

// RefreshToken refreshes the current JWT token
func (c *Client) RefreshToken(ctx context.Context) error {
	c.tokenMu.RLock()
	currentToken := c.sessionToken
	c.tokenMu.RUnlock()

	if currentToken == "" {
		return fmt.Errorf("no token to refresh - please login first")
	}

	// Build refresh request
	refreshReq := map[string]string{
		"token": currentToken,
	}

	jsonData, err := json.Marshal(refreshReq)
	if err != nil {
		return fmt.Errorf("failed to marshal refresh request: %w", err)
	}

	// Make refresh request
	url := c.config.APIBaseURL + "/v1/auth/refresh"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("refresh request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read refresh response: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		return fmt.Errorf("token refresh failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse refresh response
	var refreshResp map[string]string
	if err := json.Unmarshal(respBody, &refreshResp); err != nil {
		return fmt.Errorf("failed to parse refresh response: %w", err)
	}

	newToken, ok := refreshResp["token"]
	if !ok || newToken == "" {
		return fmt.Errorf("refresh response did not contain a token")
	}

	// Store the new token
	c.tokenMu.Lock()
	c.sessionToken = newToken
	c.tokenMu.Unlock()

	return nil
}

// IsAuthenticated checks if the client has a valid token
func (c *Client) IsAuthenticated() bool {
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.sessionToken != ""
}

// isMetadataEndpoint checks if a tool is a metadata endpoint (should not use TOON format)
func isMetadataEndpoint(toolName string) bool {
	metadataEndpoints := map[string]bool{
		"list_client_categories": true,
		"list_client_types":      true,
		"list_client_statuses":   true,
		"list_account_types":     true,
		"list_account_platforms": true,
		"list_interaction_types": true,
		"list_roles":             true,
		"list_contact_roles":     true,
	}
	return metadataEndpoints[toolName]
}

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
