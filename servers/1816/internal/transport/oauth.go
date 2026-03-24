package transport

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/Agusmazzeo/1816-mcp/internal/client"
)

// OAuthState represents an OAuth authorization state
type OAuthState struct {
	State         string
	CodeChallenge string
	RedirectURI   string
	ClientID      string
	RefreshToken  string // 1816 Keycloak refresh token
	CreatedAt     time.Time
	AuthCode      string
	Client        *client.Client // Authenticated 1816 client
	ExpiresAt     time.Time
}

// OAuthClient represents a registered OAuth client
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	RefreshToken string // 1816 refresh token (from client_secret)
	RedirectURIs []string
	CreatedAt    time.Time
}

// OAuthManager manages OAuth 2.0 with PKCE flow for 1816
type OAuthManager struct {
	states   sync.Map // map[state]*OAuthState
	codes    sync.Map // map[authCode]*OAuthState
	clients  sync.Map // map[clientID]*OAuthClient
	apiURL   string
	tokenURL string
	clientIDConfig string // Keycloak client ID
}

// NewOAuthManager creates a new OAuth manager
func NewOAuthManager(apiURL, tokenURL, clientIDConfig string) *OAuthManager {
	manager := &OAuthManager{
		apiURL:         apiURL,
		tokenURL:       tokenURL,
		clientIDConfig: clientIDConfig,
	}

	// Start cleanup goroutine for expired states
	go manager.cleanupExpiredStates()

	return manager
}

// HandleRegister handles Dynamic Client Registration (RFC 7591)
func (m *OAuthManager) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("[OAuth] Register: Failed to read body: %v\n", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	fmt.Printf("[OAuth] Register: Received request body: %s\n", string(body))

	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		fmt.Printf("[OAuth] Register: Failed to parse JSON: %v\n", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	fmt.Printf("[OAuth] Register: Parsed request: %+v\n", req)

	// Extract redirect_uris
	redirectURIs := []string{}
	if uris, ok := req["redirect_uris"].([]interface{}); ok {
		for _, uri := range uris {
			if uriStr, ok := uri.(string); ok {
				redirectURIs = append(redirectURIs, uriStr)
			}
		}
	}

	// Extract client_id and client_secret
	var clientID string
	var clientSecret string

	if id, ok := req["client_id"].(string); ok && id != "" {
		clientID = id
	}
	if secret, ok := req["client_secret"].(string); ok && secret != "" {
		clientSecret = secret
	}

	// If not in body, try Basic Auth header
	if clientID == "" || clientSecret == "" {
		if username, password, ok := r.BasicAuth(); ok {
			if clientID == "" {
				clientID = username
			}
			if clientSecret == "" {
				clientSecret = password
			}
		}
	}

	// Generate client_id if not provided
	if clientID == "" {
		generatedID, err := generateRandomString(24)
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		clientID = generatedID
	}

	// Store client (client_secret is the 1816 refresh token)
	client := &OAuthClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: clientSecret, // client_secret is the 1816 refresh token
		RedirectURIs: redirectURIs,
		CreatedAt:    time.Now(),
	}
	m.clients.Store(clientID, client)

	fmt.Printf("[OAuth] Register: Registered client_id=%s with %d redirect URIs\n", clientID, len(redirectURIs))

	// Return registration response
	response := map[string]interface{}{
		"client_id":                  clientID,
		"redirect_uris":              redirectURIs,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"token_endpoint_auth_method": "client_secret_post",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleAuthorize handles OAuth authorization requests
func (m *OAuthManager) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	responseType := query.Get("response_type")
	clientID := query.Get("client_id")
	redirectURI := query.Get("redirect_uri")
	state := query.Get("state")
	codeChallenge := query.Get("code_challenge")
	codeChallengeMethod := query.Get("code_challenge_method")

	fmt.Printf("[OAuth] Authorize: client_id=%s redirect_uri=%s state=%s\n", clientID, redirectURI, state)

	// Validate required parameters
	if responseType != "code" {
		m.sendError(w, redirectURI, state, "unsupported_response_type", "Only 'code' response type is supported")
		return
	}

	if clientID == "" || redirectURI == "" || state == "" {
		http.Error(w, "Missing required OAuth parameters (client_id, redirect_uri, state)", http.StatusBadRequest)
		return
	}

	// Try to get client - if not registered, auto-register
	clientObj, exists := m.clients.Load(clientID)
	var client *OAuthClient

	if !exists {
		client = &OAuthClient{
			ClientID:     clientID,
			ClientSecret: "",
			RefreshToken: "",
			RedirectURIs: []string{redirectURI},
			CreatedAt:    time.Now(),
		}
		m.clients.Store(clientID, client)
	} else {
		client = clientObj.(*OAuthClient)
	}

	// Validate redirect URI
	validRedirect := false
	for _, uri := range client.RedirectURIs {
		if redirectURI == uri {
			validRedirect = true
			break
		}
	}

	// Allow localhost and Claude redirects
	if !validRedirect {
		parsedURI, err := url.Parse(redirectURI)
		if err == nil {
			hostname := parsedURI.Hostname()
			if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" || hostname == "claude.ai" {
				validRedirect = true
				client.RedirectURIs = append(client.RedirectURIs, redirectURI)
				m.clients.Store(clientID, client)
			}
		}
	}

	if !validRedirect {
		m.sendError(w, redirectURI, state, "invalid_request", "Invalid redirect_uri")
		return
	}

	// Validate PKCE
	if codeChallenge == "" {
		m.sendError(w, redirectURI, state, "invalid_request", "PKCE code_challenge is required")
		return
	}
	if codeChallengeMethod != "S256" {
		m.sendError(w, redirectURI, state, "invalid_request", "Only S256 code_challenge_method is supported")
		return
	}

	// Generate authorization code
	authCode, err := generateRandomString(32)
	if err != nil {
		m.sendError(w, redirectURI, state, "server_error", "Failed to generate authorization code")
		return
	}

	// Create OAuth state
	oauthState := &OAuthState{
		State:         state,
		CodeChallenge: codeChallenge,
		RedirectURI:   redirectURI,
		ClientID:      clientID,
		AuthCode:      authCode,
		RefreshToken:  client.RefreshToken,
		CreatedAt:     time.Now(),
	}
	m.states.Store(state, oauthState)
	m.codes.Store(authCode, oauthState)

	fmt.Printf("[OAuth] Authorize: Generated auth code for client_id=%s\n", clientID)

	// Redirect back with authorization code
	redirectURL := fmt.Sprintf("%s?code=%s&state=%s",
		redirectURI,
		url.QueryEscape(authCode),
		url.QueryEscape(state))

	fmt.Printf("[OAuth] Authorize: Redirecting to %s\n", redirectURL)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// HandleToken handles OAuth token exchange
func (m *OAuthManager) HandleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		m.sendTokenError(w, "invalid_request", "Failed to parse request")
		return
	}

	grantType := r.FormValue("grant_type")
	code := r.FormValue("code")
	redirectURI := r.FormValue("redirect_uri")
	codeVerifier := r.FormValue("code_verifier")
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	refreshToken := r.FormValue("refresh_token")

	fmt.Printf("[OAuth] Token: grant_type=%s client_id=%s has_secret=%v\n", grantType, clientID, clientSecret != "")

	// Update client with secret if provided
	if clientID != "" && clientSecret != "" {
		if clientObj, exists := m.clients.Load(clientID); exists {
			client := clientObj.(*OAuthClient)
			client.ClientSecret = clientSecret
			client.RefreshToken = clientSecret // client_secret is the 1816 refresh token
			m.clients.Store(clientID, client)
			fmt.Printf("[OAuth] Token: Updated client credentials for client_id=%s\n", clientID)
		}
	}

	switch grantType {
	case "authorization_code":
		m.handleAuthorizationCodeGrant(w, code, redirectURI, codeVerifier, clientID)
	case "refresh_token":
		m.handleRefreshTokenGrant(w, refreshToken, clientID)
	default:
		m.sendTokenError(w, "unsupported_grant_type", "Supported types: authorization_code, refresh_token")
	}
}

func (m *OAuthManager) handleAuthorizationCodeGrant(w http.ResponseWriter, code, redirectURI, codeVerifier, clientID string) {
	if code == "" {
		m.sendTokenError(w, "invalid_request", "Missing authorization code")
		return
	}

	// Retrieve OAuth state by code
	stateObj, exists := m.codes.Load(code)
	if !exists {
		m.sendTokenError(w, "invalid_grant", "Invalid or expired authorization code")
		return
	}

	oauthState := stateObj.(*OAuthState)

	// Validate client_id
	if clientID != "" && clientID != oauthState.ClientID {
		m.sendTokenError(w, "invalid_client", "Client ID mismatch")
		return
	}

	// Validate redirect URI
	if redirectURI != oauthState.RedirectURI {
		m.sendTokenError(w, "invalid_grant", "Redirect URI mismatch")
		return
	}

	// Verify PKCE
	if !m.verifyCodeChallenge(codeVerifier, oauthState.CodeChallenge) {
		m.sendTokenError(w, "invalid_grant", "Code verifier verification failed")
		return
	}

	// Get client to extract refresh token
	clientObj, exists := m.clients.Load(oauthState.ClientID)
	if !exists {
		m.sendTokenError(w, "invalid_client", "Client not found")
		return
	}
	oauthClient := clientObj.(*OAuthClient)

	// Extract refresh token from client_secret
	refreshToken := oauthClient.RefreshToken

	// Validate we have refresh token
	if refreshToken == "" {
		m.sendTokenError(w, "invalid_client", "Client secret (1816 refresh token) required. Configure client_secret in Claude Desktop OAuth settings.")
		return
	}

	fmt.Printf("[OAuth] Token: Authenticating with 1816 using refresh token\n")

	// Create 1816 client with refresh token
	ctx := context.Background()
	client1816, err := client.NewClient(m.apiURL, m.tokenURL, m.clientIDConfig, "", "", refreshToken)
	if err != nil {
		fmt.Printf("[OAuth] Token: Failed to create 1816 client: %v\n", err)
		m.sendTokenError(w, "server_error", fmt.Sprintf("Failed to create 1816 client: %v", err))
		return
	}

	// Test authentication with a simple request (this will trigger token refresh internally)
	_, err = client1816.Get(ctx, "/user/session-info", nil)
	if err != nil {
		fmt.Printf("[OAuth] Token: 1816 authentication failed: %v\n", err)
		m.sendTokenError(w, "invalid_grant", fmt.Sprintf("1816 authentication failed: %v. Check your refresh token in Claude Desktop OAuth settings.", err))
		return
	}

	fmt.Printf("[OAuth] Token: 1816 authentication successful\n")

	// Generate access token
	accessToken, err := generateRandomString(32)
	if err != nil {
		m.sendTokenError(w, "server_error", "Failed to generate access token")
		return
	}

	// Generate refresh token for MCP session
	newRefreshToken, err := generateRandomString(32)
	if err != nil {
		m.sendTokenError(w, "server_error", "Failed to generate refresh token")
		return
	}

	// Store session with the authenticated 1816 client
	sessionState := &OAuthState{
		State:        accessToken,
		ClientID:     oauthState.ClientID,
		RefreshToken: refreshToken,
		Client:       client1816,
		ExpiresAt:    time.Now().Add(30 * 24 * time.Hour), // 30 days
		CreatedAt:    time.Now(),
	}
	m.codes.Store(accessToken, sessionState)
	m.codes.Store(newRefreshToken, sessionState)

	// Clean up used code
	m.codes.Delete(code)
	m.states.Delete(oauthState.State)

	fmt.Printf("[OAuth] Token: Issued access_token and refresh_token for client_id=%s\n", oauthState.ClientID)

	// Return tokens
	response := map[string]interface{}{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    2592000, // 30 days
		"refresh_token": newRefreshToken,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *OAuthManager) handleRefreshTokenGrant(w http.ResponseWriter, refreshToken, clientID string) {
	if refreshToken == "" {
		m.sendTokenError(w, "invalid_request", "Missing refresh token")
		return
	}

	// Retrieve session by refresh token
	stateObj, exists := m.codes.Load(refreshToken)
	if !exists {
		m.sendTokenError(w, "invalid_grant", "Invalid or expired refresh token")
		return
	}

	oauthState := stateObj.(*OAuthState)

	// Validate client_id
	if clientID != "" && clientID != oauthState.ClientID {
		m.sendTokenError(w, "invalid_client", "Client ID mismatch")
		return
	}

	// Check expiration
	if time.Now().After(oauthState.ExpiresAt) {
		m.codes.Delete(refreshToken)
		m.sendTokenError(w, "invalid_grant", "Refresh token expired")
		return
	}

	// Re-authenticate with 1816
	ctx := context.Background()
	client1816, err := client.NewClient(m.apiURL, m.tokenURL, m.clientIDConfig, "", "", oauthState.RefreshToken)
	if err != nil {
		m.sendTokenError(w, "server_error", "Failed to create 1816 client")
		return
	}

	// Test authentication
	_, err = client1816.Get(ctx, "/user/session-info", nil)
	if err != nil {
		m.sendTokenError(w, "invalid_grant", fmt.Sprintf("1816 re-authentication failed: %v", err))
		return
	}

	// Generate new tokens
	newAccessToken, err := generateRandomString(32)
	if err != nil {
		m.sendTokenError(w, "server_error", "Failed to generate access token")
		return
	}

	newRefreshToken, err := generateRandomString(32)
	if err != nil {
		m.sendTokenError(w, "server_error", "Failed to generate refresh token")
		return
	}

	// Store new session
	newState := &OAuthState{
		State:        newAccessToken,
		ClientID:     oauthState.ClientID,
		RefreshToken: oauthState.RefreshToken,
		Client:       client1816,
		ExpiresAt:    time.Now().Add(30 * 24 * time.Hour),
		CreatedAt:    time.Now(),
	}
	m.codes.Store(newAccessToken, newState)
	m.codes.Store(newRefreshToken, newState)

	// Delete old tokens
	m.codes.Delete(refreshToken)

	// Return new tokens
	response := map[string]interface{}{
		"access_token":  newAccessToken,
		"token_type":    "Bearer",
		"expires_in":    2592000,
		"refresh_token": newRefreshToken,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetClient retrieves an authenticated 1816 client by access token
func (m *OAuthManager) GetClient(accessToken string) (*client.Client, error) {
	stateObj, exists := m.codes.Load(accessToken)
	if !exists {
		return nil, fmt.Errorf("invalid access token")
	}

	oauthState := stateObj.(*OAuthState)

	// Check expiration
	if time.Now().After(oauthState.ExpiresAt) {
		m.codes.Delete(accessToken)
		return nil, fmt.Errorf("access token expired")
	}

	return oauthState.Client, nil
}

// verifyCodeChallenge verifies PKCE code challenge
func (m *OAuthManager) verifyCodeChallenge(verifier, challenge string) bool {
	h := sha256.New()
	h.Write([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	return computed == challenge
}

// sendError redirects back with OAuth error
func (m *OAuthManager) sendError(w http.ResponseWriter, redirectURI, state, errorCode, errorDesc string) {
	if redirectURI == "" {
		http.Error(w, fmt.Sprintf("%s: %s", errorCode, errorDesc), http.StatusBadRequest)
		return
	}

	redirectURL := fmt.Sprintf("%s?error=%s&error_description=%s&state=%s",
		redirectURI,
		url.QueryEscape(errorCode),
		url.QueryEscape(errorDesc),
		url.QueryEscape(state))

	w.Header().Set("Location", redirectURL)
	w.WriteHeader(http.StatusFound)
}

// sendTokenError sends OAuth token error response
func (m *OAuthManager) sendTokenError(w http.ResponseWriter, errorCode, errorDesc string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{
		"error":             errorCode,
		"error_description": errorDesc,
	})
}

// cleanupExpiredStates removes expired OAuth states
func (m *OAuthManager) cleanupExpiredStates() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		// Clean expired states
		m.states.Range(func(key, value interface{}) bool {
			state := value.(*OAuthState)
			if now.Sub(state.CreatedAt) > 10*time.Minute {
				m.states.Delete(key)
				if state.AuthCode != "" {
					m.codes.Delete(state.AuthCode)
				}
			}
			return true
		})

		// Clean expired sessions
		m.codes.Range(func(key, value interface{}) bool {
			state := value.(*OAuthState)
			if !state.ExpiresAt.IsZero() && now.After(state.ExpiresAt) {
				m.codes.Delete(key)
			}
			return true
		})
	}
}

// generateRandomString generates a cryptographically secure random string
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
