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

	"github.com/Agusmazzeo/allfunds-mcp/internal/graphql"
)

// OAuthState represents an OAuth authorization state
type OAuthState struct {
	State         string
	CodeChallenge string
	RedirectURI   string
	ClientID      string
	Email         string // Allfunds email
	Password      string // Allfunds password
	CreatedAt     time.Time
	AuthCode      string
	Client        *graphql.Client // Authenticated Allfunds client
	ExpiresAt     time.Time
}

// OAuthClient represents a registered OAuth client
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	Email        string // Allfunds email (from client_id)
	Password     string // Allfunds password (from client_secret)
	RedirectURIs []string
	CreatedAt    time.Time
}

// OAuthManager manages OAuth 2.0 with PKCE flow for Allfunds
type OAuthManager struct {
	states     sync.Map // map[state]*OAuthState
	codes      sync.Map // map[authCode]*OAuthState
	clients    sync.Map // map[clientID]*OAuthClient
	graphqlURL string
}

// NewOAuthManager creates a new OAuth manager
func NewOAuthManager(graphqlURL string) *OAuthManager {
	manager := &OAuthManager{
		graphqlURL: graphqlURL,
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

	// Extract client_id (Allfunds email) - try multiple sources
	var clientID string
	var clientSecret string

	// First try JSON body
	if id, ok := req["client_id"].(string); ok && id != "" {
		clientID = id
	}
	if secret, ok := req["client_secret"].(string); ok && secret != "" {
		clientSecret = secret
	}

	// If not in body, try Basic Auth header (some OAuth clients send credentials this way)
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

	// Generate client_id if still not provided (for dynamic registration)
	if clientID == "" {
		generatedID, err := generateRandomString(24)
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		clientID = generatedID
	}

	// For Allfunds, we REQUIRE client_secret (the password)
	// If not provided now, it must be provided during authorization
	if clientSecret == "" {
		// Allow registration without secret, but mark it
		// Secret will be required during /authorize or /token
		clientSecret = "" // Will be empty, indicating credentials not yet provided
	}

	// Store client with credentials
	client := &OAuthClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Email:        clientID,    // client_id is the email
		Password:     clientSecret, // client_secret is the password
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

	// Try to get client - if not registered, auto-register with empty credentials
	clientObj, exists := m.clients.Load(clientID)
	var client *OAuthClient

	if !exists {
		// Auto-register client on first authorization attempt
		// Credentials will be extracted from Authorization header or query params
		client = &OAuthClient{
			ClientID:     clientID,
			ClientSecret: "", // Will be populated from auth header
			Email:        "",
			Password:     "",
			RedirectURIs: []string{redirectURI}, // Trust the redirect URI on first use
			CreatedAt:    time.Now(),
		}
		m.clients.Store(clientID, client)
	} else {
		client = clientObj.(*OAuthClient)
	}

	// Extract credentials from Authorization header if not already in client
	if client.Email == "" || client.Password == "" {
		// Try Basic Auth
		if username, password, ok := r.BasicAuth(); ok {
			client.Email = username
			client.Password = password
			m.clients.Store(clientID, client) // Update with credentials
		} else {
			// Try Bearer token or other auth methods
			auth := r.Header.Get("Authorization")
			if auth != "" {
				// Could be credentials encoded in other formats
				// For now, we'll check query params as fallback
			}
		}
	}

	// Validate redirect URI
	validRedirect := false
	for _, uri := range client.RedirectURIs {
		if redirectURI == uri {
			validRedirect = true
			break
		}
	}

	// Also allow localhost and Claude redirects for public clients
	if !validRedirect {
		parsedURI, err := url.Parse(redirectURI)
		if err == nil {
			hostname := parsedURI.Hostname()
			// Allow localhost and claude.ai redirects
			if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" || hostname == "claude.ai" {
				validRedirect = true
				// Add to allowed redirect URIs
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

	// Generate authorization code (authentication happens during token exchange)
	authCode, err := generateRandomString(32)
	if err != nil {
		m.sendError(w, redirectURI, state, "server_error", "Failed to generate authorization code")
		return
	}

	// Create OAuth state (client credentials will be provided during token exchange)
	oauthState := &OAuthState{
		State:         state,
		CodeChallenge: codeChallenge,
		RedirectURI:   redirectURI,
		ClientID:      clientID,
		AuthCode:      authCode,
		Email:         client.Email,    // May be empty, will be set during token exchange
		Password:      client.Password, // May be empty, will be set during token exchange
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
			client.Password = clientSecret // client_secret is the Allfunds password
			if client.Email == "" {
				client.Email = clientID // client_id is the Allfunds email
			}
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

	// Get client to extract credentials (client_secret = password)
	clientObj, exists := m.clients.Load(oauthState.ClientID)
	if !exists {
		m.sendTokenError(w, "invalid_client", "Client not found")
		return
	}
	client := clientObj.(*OAuthClient)

	// Extract email and password (client_id and client_secret from Claude Desktop config)
	email := client.Email
	password := client.Password

	// If not set in client, use clientID as email
	if email == "" {
		email = oauthState.ClientID
	}

	// Validate we have credentials
	if password == "" {
		m.sendTokenError(w, "invalid_client", "Client secret (Allfunds password) required. Configure client_secret in Claude Desktop OAuth settings.")
		return
	}

	fmt.Printf("[OAuth] Token: Logging in to Allfunds with email=%s\n", email)

	// Login to Allfunds GraphQL
	ctx := context.Background()
	allfundsClient, err := graphql.NewClient(m.graphqlURL, email, password)
	if err != nil {
		fmt.Printf("[OAuth] Token: Failed to create Allfunds client: %v\n", err)
		m.sendTokenError(w, "server_error", fmt.Sprintf("Failed to create Allfunds client: %v", err))
		return
	}

	if err := allfundsClient.Login(ctx); err != nil {
		fmt.Printf("[OAuth] Token: Allfunds login failed: %v\n", err)
		m.sendTokenError(w, "invalid_grant", fmt.Sprintf("Allfunds login failed: %v. Check your credentials in Claude Desktop OAuth settings.", err))
		return
	}

	fmt.Printf("[OAuth] Token: Allfunds login successful\n")

	// Generate access token (session ID for the authenticated client)
	accessToken, err := generateRandomString(32)
	if err != nil {
		m.sendTokenError(w, "server_error", "Failed to generate access token")
		return
	}

	// Generate refresh token
	refreshToken, err := generateRandomString(32)
	if err != nil {
		m.sendTokenError(w, "server_error", "Failed to generate refresh token")
		return
	}

	// Store session with the authenticated Allfunds client
	sessionState := &OAuthState{
		State:     accessToken,
		ClientID:  oauthState.ClientID,
		Email:     email,
		Password:  password,
		Client:    allfundsClient,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour), // 30 days
		CreatedAt: time.Now(),
	}
	m.codes.Store(accessToken, sessionState)
	m.codes.Store(refreshToken, sessionState)

	// Clean up used code
	m.codes.Delete(code)
	m.states.Delete(oauthState.State)

	fmt.Printf("[OAuth] Token: Issued access_token and refresh_token for client_id=%s\n", oauthState.ClientID)

	// Return tokens
	response := map[string]interface{}{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    2592000, // 30 days
		"refresh_token": refreshToken,
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

	// Re-authenticate with Allfunds to get fresh session
	ctx := context.Background()
	allfundsClient, err := graphql.NewClient(m.graphqlURL, oauthState.Email, oauthState.Password)
	if err != nil {
		m.sendTokenError(w, "server_error", "Failed to create Allfunds client")
		return
	}

	if err := allfundsClient.Login(ctx); err != nil {
		m.sendTokenError(w, "invalid_grant", fmt.Sprintf("Allfunds re-authentication failed: %v", err))
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
		State:     newAccessToken,
		ClientID:  oauthState.ClientID,
		Email:     oauthState.Email,
		Password:  oauthState.Password,
		Client:    allfundsClient,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		CreatedAt: time.Now(),
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

// GetClient retrieves an authenticated Allfunds client by access token
func (m *OAuthManager) GetClient(accessToken string) (*graphql.Client, error) {
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
