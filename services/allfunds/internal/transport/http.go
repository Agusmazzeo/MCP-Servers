package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Criteria/MCP-Servers/services/allfunds/internal/graphql"
	"github.com/Criteria/MCP-Servers/services/allfunds/internal/handlers"
	"github.com/Criteria/MCP-Servers/shared/pkg/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HTTPTransport provides HTTP/SSE transport for Allfunds MCP
type HTTPTransport struct {
	port         int
	toolDefs     []*tools.ToolDefinition
	oauthManager *OAuthManager
	graphqlURL   string
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(port int, toolDefs []*tools.ToolDefinition, graphqlURL string) *HTTPTransport {
	return &HTTPTransport{
		port:         port,
		toolDefs:     toolDefs,
		oauthManager: NewOAuthManager(graphqlURL),
		graphqlURL:   graphqlURL,
	}
}

// Start starts the HTTP server
func (t *HTTPTransport) Start() error {
	mux := http.NewServeMux()

	// OAuth discovery endpoints
	mux.HandleFunc("/.well-known/oauth-protected-resource", t.handleOAuthProtectedResourceMetadata)
	mux.HandleFunc("/.well-known/oauth-authorization-server", t.handleOAuthAuthorizationServerMetadata)

	// OAuth endpoints
	mux.HandleFunc("/register", t.oauthManager.HandleRegister)
	mux.HandleFunc("/authorize", t.oauthManager.HandleAuthorize)
	mux.HandleFunc("/token", t.oauthManager.HandleToken)

	// MCP protocol endpoints
	mux.HandleFunc("/sse", t.handleSSE)
	mux.HandleFunc("/", t.handleSSE)

	// Utility endpoints
	mux.HandleFunc("/health", t.handleHealth)
	mux.HandleFunc("/tools", t.handleListTools)

	// Apply middleware
	handler := t.corsMiddleware(mux)

	addr := fmt.Sprintf(":%d", t.port)
	return http.ListenAndServe(addr, handler)
}

// handleSSE manages Server-Sent Events connection
func (t *HTTPTransport) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Handle POST requests - process MCP messages
	if r.Method == http.MethodPost {
		t.handleSSEMessage(w, r)
		return
	}

	// Handle GET requests - establish SSE connection
	if r.Method == http.MethodGet {
		// Check authentication
		accessToken := extractBearerToken(r)
		if accessToken == "" {
			t.sendOAuthUnauthorized(w, r)
			return
		}

		// Validate token
		if _, err := t.oauthManager.GetClient(accessToken); err != nil {
			t.sendOAuthUnauthorized(w, r)
			return
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Send connection established
		fmt.Fprintf(w, "event: endpoint\ndata: /sse\n\n")
		flusher.Flush()

		// Keep alive with pings
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				fmt.Fprintf(w, ": ping\n\n")
				flusher.Flush()
			}
		}
	}

	// Handle OPTIONS
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleSSEMessage processes MCP messages via SSE POST
func (t *HTTPTransport) handleSSEMessage(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: {\"error\":\"Failed to read request\"}\n\n")
		flusher.Flush()
		return
	}

	var message map[string]interface{}
	if err := json.Unmarshal(body, &message); err != nil {
		fmt.Fprintf(w, "event: error\ndata: {\"error\":\"Invalid JSON\"}\n\n")
		flusher.Flush()
		return
	}

	method, _ := message["method"].(string)

	// Allow initialize and tools/list without authentication
	var client *graphql.Client
	if method != "initialize" && method != "tools/list" {
		accessToken := extractBearerToken(r)
		if accessToken == "" {
			t.sendOAuthUnauthorizedSSE(w, message, flusher)
			return
		}

		// Get authenticated Allfunds client
		var err error
		client, err = t.oauthManager.GetClient(accessToken)
		if err != nil {
			t.sendOAuthUnauthorizedSSE(w, message, flusher)
			return
		}
	}

	// Process message
	var response interface{}
	switch method {
	case "initialize":
		response = t.handleInitialize(message)
	case "tools/list":
		response = t.handleToolsList(message)
	case "tools/call":
		response = t.handleToolCall(message, client)
	default:
		response = map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      message["id"],
			"error": map[string]interface{}{
				"code":    -32601,
				"message": fmt.Sprintf("Method not found: %s", method),
			},
		}
	}

	// Send response as SSE event
	responseJSON, err := json.Marshal(response)
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: {\"error\":\"Internal error\"}\n\n")
		flusher.Flush()
		return
	}

	fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(responseJSON))
	flusher.Flush()
}

// handleInitialize handles MCP initialization
func (t *HTTPTransport) handleInitialize(message map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      message["id"],
		"result": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":        "allfunds-mcp",
				"version":     "1.0.0",
				"description": fmt.Sprintf("MCP server for Allfunds Connect - provides %d tools for fund research and analysis", len(t.toolDefs)),
			},
		},
	}
}

// handleToolsList returns available tools
func (t *HTTPTransport) handleToolsList(message map[string]interface{}) map[string]interface{} {
	mcpTools := make([]map[string]interface{}, len(t.toolDefs))

	for i, toolDef := range t.toolDefs {
		mcpTools[i] = map[string]interface{}{
			"name":        toolDef.Name,
			"description": toolDef.Description,
			"inputSchema": toolDef.InputSchema,
		}
	}

	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      message["id"],
		"result": map[string]interface{}{
			"tools": mcpTools,
		},
	}
}

// handleToolCall executes a tool
func (t *HTTPTransport) handleToolCall(message map[string]interface{}, client *graphql.Client) map[string]interface{} {
	params, ok := message["params"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      message["id"],
			"error": map[string]interface{}{
				"code":    -32602,
				"message": "Invalid params",
			},
		}
	}

	toolName, _ := params["name"].(string)
	arguments, _ := params["arguments"].(map[string]interface{})

	// Find tool definition
	var toolDef *tools.ToolDefinition
	for _, td := range t.toolDefs {
		if td.Name == toolName {
			toolDef = td
			break
		}
	}

	if toolDef == nil {
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      message["id"],
			"error": map[string]interface{}{
				"code":    -32602,
				"message": fmt.Sprintf("Tool not found: %s", toolName),
			},
		}
	}

	// Create handler factory with authenticated client
	handlerFactory := handlers.NewHandlerFactory(client)

	// Get handler for this tool
	var handler func(context.Context, *mcp.CallToolRequest, map[string]interface{}) (*mcp.CallToolResult, any, error)
	if toolName == "login" {
		handler = handlerFactory.CreateLoginHandler()
	} else {
		handler = handlerFactory.CreateHandler(toolName)
	}

	// Execute tool
	ctx := context.Background()
	if arguments == nil {
		arguments = make(map[string]interface{})
	}

	result, _, err := handler(ctx, nil, arguments)
	if err != nil {
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      message["id"],
			"error": map[string]interface{}{
				"code":    -32603,
				"message": fmt.Sprintf("Tool execution error: %v", err),
			},
		}
	}

	// Check if result indicates an error
	if result.IsError {
		errorText := ""
		for _, content := range result.Content {
			if textContent, ok := content.(*mcp.TextContent); ok {
				errorText = textContent.Text
				break
			}
		}
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      message["id"],
			"error": map[string]interface{}{
				"code":    -32603,
				"message": errorText,
			},
		}
	}

	// Convert MCP result to JSON-RPC response
	contentArray := make([]map[string]interface{}, 0, len(result.Content))
	for _, content := range result.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			contentArray = append(contentArray, map[string]interface{}{
				"type": "text",
				"text": textContent.Text,
			})
		}
	}

	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      message["id"],
		"result": map[string]interface{}{
			"content": contentArray,
		},
	}
}

// handleHealth returns server health
func (t *HTTPTransport) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// handleListTools returns available tools
func (t *HTTPTransport) handleListTools(w http.ResponseWriter, r *http.Request) {
	toolList := make([]map[string]interface{}, len(t.toolDefs))

	for i, toolDef := range t.toolDefs {
		toolList[i] = map[string]interface{}{
			"name":        toolDef.Name,
			"description": toolDef.Description,
		}
	}

	response := map[string]interface{}{
		"tools": toolList,
		"count": len(toolList),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleOAuthProtectedResourceMetadata implements RFC 9728
func (t *HTTPTransport) handleOAuthProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	baseURL := getBaseURL(r)

	metadata := map[string]interface{}{
		"resource":                 baseURL,
		"authorization_servers":    []string{baseURL},
		"bearer_methods_supported": []string{"header"},
		"scopes_supported":         []string{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metadata)
}

// handleOAuthAuthorizationServerMetadata implements RFC 8414
func (t *HTTPTransport) handleOAuthAuthorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	baseURL := getBaseURL(r)

	metadata := map[string]interface{}{
		"issuer":                                baseURL,
		"authorization_endpoint":                fmt.Sprintf("%s/authorize", baseURL),
		"token_endpoint":                        fmt.Sprintf("%s/token", baseURL),
		"registration_endpoint":                 fmt.Sprintf("%s/register", baseURL),
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metadata)
}

// sendOAuthUnauthorized sends 401 with WWW-Authenticate header
func (t *HTTPTransport) sendOAuthUnauthorized(w http.ResponseWriter, r *http.Request) {
	baseURL := getBaseURL(r)
	discoveryURL := fmt.Sprintf("%s/.well-known/oauth-protected-resource", baseURL)

	w.Header().Set("WWW-Authenticate", fmt.Sprintf("Bearer resource_metadata=\"%s\"", discoveryURL))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   "unauthorized",
		"message": "Authentication required. Use OAuth to authenticate with your Allfunds credentials.",
	})
}

// sendOAuthUnauthorizedSSE sends OAuth unauthorized via SSE
func (t *HTTPTransport) sendOAuthUnauthorizedSSE(w http.ResponseWriter, message map[string]interface{}, flusher http.Flusher) {
	discoveryURL := fmt.Sprintf("http://localhost:%d/.well-known/oauth-protected-resource", t.port)

	w.Header().Set("WWW-Authenticate", fmt.Sprintf("Bearer resource_metadata=\"%s\"", discoveryURL))

	errorResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      message["id"],
		"error": map[string]interface{}{
			"code":    -32001,
			"message": "Authentication required. Please authenticate using OAuth with your Allfunds credentials.",
		},
	}

	responseJSON, _ := json.Marshal(errorResponse)
	fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(responseJSON))
	flusher.Flush()
}

// extractBearerToken extracts Bearer token from Authorization header
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	parts := strings.Split(auth, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}

	return parts[1]
}

// getBaseURL constructs the base URL from the request
func getBaseURL(r *http.Request) string {
	scheme := "http"

	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS != nil {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

// corsMiddleware adds CORS headers
func (t *HTTPTransport) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
