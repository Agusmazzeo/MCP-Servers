package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Client handles HTTP requests to 1816 API with Keycloak OAuth
type Client struct {
	httpClient   *http.Client
	apiURL       string
	tokenURL     string
	clientID     string
	email        string
	password     string
	refreshToken string
	accessToken  string
	tokenExpiry  time.Time
	mu           sync.RWMutex
}

// TokenResponse represents the Keycloak token response
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
}

// NewClient creates a new 1816 API client with Keycloak OAuth
func NewClient(apiURL, tokenURL, clientID, email, password, refreshToken string) (*Client, error) {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	client := &Client{
		httpClient:   httpClient,
		apiURL:       apiURL,
		tokenURL:     tokenURL,
		clientID:     clientID,
		email:        email,
		password:     password,
		refreshToken: refreshToken,
	}

	// Get initial access token
	if err := client.refreshAuth(context.Background(), 0); err != nil {
		return nil, fmt.Errorf("failed to authenticate: %w", err)
	}

	return client, nil
}

// refreshAuth refreshes the access token using the refresh token
func (c *Client) refreshAuth(ctx context.Context, retries int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if retries > 3 {
		return fmt.Errorf("max retries exceeded for token refresh")
	}

	log.Printf("[1816] Refreshing access token (attempt %d)", retries+1)

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", c.clientID)
	data.Set("refresh_token", c.refreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", c.tokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token refresh failed: HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	c.accessToken = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	// Update refresh token if a new one was provided
	if tokenResp.RefreshToken != "" {
		c.refreshToken = tokenResp.RefreshToken
	}

	log.Printf("[1816] Access token refreshed successfully (expires in %d seconds)", tokenResp.ExpiresIn)
	return nil
}

// ensureAuth ensures we have a valid access token
func (c *Client) ensureAuth(ctx context.Context) error {
	c.mu.RLock()
	needsRefresh := time.Now().After(c.tokenExpiry.Add(-30 * time.Second))
	c.mu.RUnlock()

	if needsRefresh {
		return c.refreshAuth(ctx, 0)
	}
	return nil
}

// Get performs a GET request to the 1816 API
func (c *Client) Get(ctx context.Context, path string, params map[string]interface{}) (interface{}, error) {
	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}

	fullURL := c.apiURL + path
	if len(params) > 0 {
		queryParams := url.Values{}
		for k, v := range params {
			queryParams.Set(k, fmt.Sprintf("%v", v))
		}
		fullURL += "?" + queryParams.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.mu.RLock()
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	c.mu.RUnlock()

	log.Printf("[1816] GET %s", path)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// If unauthorized, try refreshing token once
	if resp.StatusCode == http.StatusUnauthorized {
		log.Println("[1816] Token expired, refreshing and retrying...")
		if err := c.refreshAuth(ctx, 0); err != nil {
			return nil, err
		}
		return c.Get(ctx, path, params)
	}

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

// Post performs a POST request to the 1816 API
func (c *Client) Post(ctx context.Context, path string, body interface{}) (interface{}, error) {
	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.mu.RLock()
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	c.mu.RUnlock()
	req.Header.Set("Content-Type", "application/json")

	log.Printf("[1816] POST %s", path)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// If unauthorized, try refreshing token once
	if resp.StatusCode == http.StatusUnauthorized {
		log.Println("[1816] Token expired, refreshing and retrying...")
		if err := c.refreshAuth(ctx, 0); err != nil {
			return nil, err
		}
		return c.Post(ctx, path, body)
	}

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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
