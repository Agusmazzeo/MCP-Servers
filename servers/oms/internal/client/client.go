package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// Client handles HTTP requests to OMS with cookie-based authentication
type Client struct {
	httpClient  *http.Client
	baseURL     string
	omsIP       string
	username    string
	password    string
	isLoggedIn  bool
	cookieJar   *cookiejar.Jar
}

// LoginResponse represents the OMS login response
type LoginResponse struct {
	Login bool              `json:"login"`
	Error *LoginError       `json:"error,omitempty"`
}

// LoginError represents the login error structure
type LoginError struct {
	Message string `json:"message"`
}

// NewClient creates a new OMS API client with DNS bypass
func NewClient(baseURL, omsIP, username, password string) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	// Create custom dialer that bypasses DNS for oms.cdfbox.com
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	// Custom transport with DNS bypass
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// If connecting to oms.cdfbox.com, use hardcoded IP
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			if host == "oms.cdfbox.com" {
				// Use hardcoded IP but preserve SNI servername
				addr = net.JoinHostPort(omsIP, port)
			}

			return dialer.DialContext(ctx, network, addr)
		},
		TLSClientConfig: &tls.Config{
			ServerName: "oms.cdfbox.com", // Force SNI for certificate validation
		},
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{
		Jar:       jar,
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
		omsIP:      omsIP,
		username:   username,
		password:   password,
		isLoggedIn: false,
		cookieJar:  jar,
	}, nil
}

// Login authenticates with OMS
func (c *Client) Login(ctx context.Context) error {
	if c.isLoggedIn {
		return nil
	}

	log.Printf("[OMS] Login: Authenticating with username=%s", c.username)

	credentials := map[string]string{
		"user":     c.username,
		"password": c.password,
	}

	bodyBytes, err := json.Marshal(credentials)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/login", bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OMS login failed: HTTP %d", resp.StatusCode)
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return fmt.Errorf("failed to decode login response: %w", err)
	}

	if !loginResp.Login {
		errMsg := "unknown error"
		if loginResp.Error != nil {
			errMsg = loginResp.Error.Message
		}
		return fmt.Errorf("OMS login failed: %s", errMsg)
	}

	c.isLoggedIn = true
	log.Printf("[OMS] Login: SUCCESS")
	return nil
}

// Get performs a GET request to the OMS API
func (c *Client) Get(ctx context.Context, path string) (interface{}, error) {
	if err := c.ensureSession(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	log.Printf("[OMS] GET %s", path)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes[:min(300, len(bodyBytes))]))
	}

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// Post performs a POST request to the OMS API
func (c *Client) Post(ctx context.Context, path string, body interface{}) (interface{}, error) {
	if err := c.ensureSession(ctx); err != nil {
		return nil, err
	}

	// Wrap body in parentOrder structure
	wrappedBody := map[string]interface{}{
		"parentOrder": body,
	}

	bodyBytes, err := json.Marshal(wrappedBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %w", err)
	}

	// Add random sId to prevent caching
	fullPath := fmt.Sprintf("%s?sId=%f", path, rand.Float64())

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+fullPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	log.Printf("[OMS] POST %s", path)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes[:min(300, len(bodyBytes))]))
	}

	// Read response with timeout
	responseChan := make(chan []byte, 1)
	errorChan := make(chan error, 1)

	go func() {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			errorChan <- err
			return
		}
		responseChan <- data
	}()

	var responseData []byte
	select {
	case responseData = <-responseChan:
		// Success
	case err := <-errorChan:
		return nil, fmt.Errorf("failed to read response: %w", err)
	case <-time.After(10 * time.Second):
		// Timeout - return empty response
		log.Printf("[OMS] POST response timeout, assuming success")
		return map[string]interface{}{"status": "submitted"}, nil
	}

	if len(responseData) == 0 {
		return map[string]interface{}{"status": "submitted"}, nil
	}

	var result interface{}
	if err := json.Unmarshal(responseData, &result); err != nil {
		// If parsing fails, return raw text
		return map[string]interface{}{"raw": string(responseData)}, nil
	}

	return result, nil
}

// ensureSession ensures the client is logged in
func (c *Client) ensureSession(ctx context.Context) error {
	if c.isLoggedIn {
		return nil
	}
	return c.Login(ctx)
}

// IsLoggedIn returns the login status
func (c *Client) IsLoggedIn() bool {
	return c.isLoggedIn
}

// ResetSession resets the login state
func (c *Client) ResetSession() {
	c.isLoggedIn = false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
