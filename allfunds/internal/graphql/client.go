package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// Client handles GraphQL requests to Allfunds Connect
type Client struct {
	httpClient    *http.Client
	graphqlURL    string
	email         string
	password      string
	csrfToken     string
	authenticated bool
}

// GraphQLRequest represents a GraphQL request
type GraphQLRequest struct {
	OperationName string                 `json:"operationName"`
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables"`
}

// GraphQLResponse represents a GraphQL response
type GraphQLResponse struct {
	Data   interface{}    `json:"data"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

// GraphQLError represents a GraphQL error
type GraphQLError struct {
	Message string `json:"message"`
}

// NewClient creates a new GraphQL client for Allfunds Connect
func NewClient(graphqlURL, email, password string) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	return &Client{
		httpClient: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
		},
		graphqlURL: graphqlURL,
		email:      email,
		password:   password,
	}, nil
}

// Login authenticates with Allfunds Connect
func (c *Client) Login(ctx context.Context) error {
	mutation := `
		mutation LogIn($email: String!, $password: String!) {
		  log_in(email: $email, password: $password) {
			user { id }
			csrf_token
			errors
		  }
		}
	`

	req := GraphQLRequest{
		OperationName: "LogIn",
		Query:         mutation,
		Variables: map[string]interface{}{
			"email":    c.email,
			"password": c.password,
		},
	}

	var resp struct {
		LogIn struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
			CSRFToken string   `json:"csrf_token"`
			Errors    []string `json:"errors"`
		} `json:"log_in"`
	}

	if err := c.execute(ctx, req, &resp); err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}

	if len(resp.LogIn.Errors) > 0 {
		return fmt.Errorf("login failed: %v", resp.LogIn.Errors)
	}

	c.csrfToken = resp.LogIn.CSRFToken
	c.authenticated = true
	return nil
}

// Query executes a GraphQL query with auto-authentication
func (c *Client) Query(ctx context.Context, operation, query string, variables map[string]interface{}, result interface{}) error {
	if !c.authenticated {
		if err := c.Login(ctx); err != nil {
			return err
		}
	}

	req := GraphQLRequest{
		OperationName: operation,
		Query:         query,
		Variables:     variables,
	}

	err := c.execute(ctx, req, result)

	// Retry once on auth error
	if isAuthError(err) {
		c.authenticated = false
		if loginErr := c.Login(ctx); loginErr != nil {
			return loginErr
		}
		return c.execute(ctx, req, result)
	}

	return err
}

// HTTPClient returns the underlying HTTP client (for document downloads)
func (c *Client) HTTPClient() *http.Client {
	return c.httpClient
}

// DownloadFile downloads a file from Allfunds using authenticated session
func (c *Client) DownloadFile(ctx context.Context, url string) ([]byte, error) {
	if !c.authenticated {
		if err := c.Login(ctx); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.csrfToken != "" {
		req.Header.Set("X-CSRF-Token", c.csrfToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		// Retry with fresh login
		c.authenticated = false
		if loginErr := c.Login(ctx); loginErr != nil {
			return nil, loginErr
		}
		return c.DownloadFile(ctx, url)
	}

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return content, nil
}

// execute performs the actual GraphQL request
func (c *Client) execute(ctx context.Context, gqlReq GraphQLRequest, result interface{}) error {
	body, err := json.Marshal(gqlReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.graphqlURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers as per Python implementation
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://next.allfunds.com")
	req.Header.Set("Referer", "https://next.allfunds.com/")

	if c.csrfToken != "" {
		req.Header.Set("X-CSRF-Token", c.csrfToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("authentication required")
	}

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var gqlResp GraphQLResponse
	gqlResp.Data = result

	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("GraphQL errors: %v", gqlResp.Errors)
	}

	return nil
}

// isAuthError checks if an error is an authentication error
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "authentication required"
}
