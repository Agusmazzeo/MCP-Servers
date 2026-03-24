package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Agusmazzeo/1816-mcp/internal/client"
	"github.com/Agusmazzeo/1816-mcp/internal/handlers"
	"github.com/Agusmazzeo/1816-mcp/internal/tools"
	"github.com/Agusmazzeo/1816-mcp/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServerConfig holds MCP server configuration
type ServerConfig struct {
	Name    string
	Version string
}

// Server wraps an MCP server with lifecycle management
type Server struct {
	mcp          *mcp.Server
	config       *ServerConfig
	apiURL       string
	tokenURL     string
	clientIDConfig string
	oauthManager *transport.OAuthManager
	toolDefs     []*tools.ToolDefinition
}

// NewServer creates a new MCP server
func NewServer(config *ServerConfig, apiURL, tokenURL, clientIDConfig string) *Server {
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    config.Name,
		Version: config.Version,
	}, nil)

	return &Server{
		mcp:          mcpServer,
		config:       config,
		apiURL:       apiURL,
		tokenURL:     tokenURL,
		clientIDConfig: clientIDConfig,
		oauthManager: transport.NewOAuthManager(apiURL, tokenURL, clientIDConfig),
	}
}

// RegisterTools registers tools with the server
func (s *Server) RegisterTools(factory tools.HandlerFactory, toolDefs []*tools.ToolDefinition, includeLogin bool) {
	s.toolDefs = toolDefs
	tools.RegisterTools(s.mcp, factory, toolDefs, includeLogin)
}

// SetToolDefinitions sets tool definitions without registering them (for HTTP mode)
func (s *Server) SetToolDefinitions(toolDefs []*tools.ToolDefinition) {
	s.toolDefs = toolDefs
}

// RunStdio runs the server over stdio transport
func (s *Server) RunStdio() error {
	log.Println("========================================")
	log.Printf("%s MCP Server Starting", s.config.Name)
	log.Printf("Version: %s", s.config.Version)
	log.Println("Mode: stdio")
	log.Println("========================================")
	log.Println("Starting server over stdio transport")
	log.Println("Ready to accept MCP requests!")
	log.Println("========================================")

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, stopping server...")
		cancel()
	}()

	// Ensure stdout is unbuffered for immediate MCP protocol responses
	os.Stdout.Sync()

	if err := s.mcp.Run(ctx, &mcp.StdioTransport{}); err != nil && err != context.Canceled {
		log.Printf("Server error: %v", err)
		return err
	}

	log.Println("Server stopped gracefully")
	return nil
}

// RunHTTP runs the server over HTTP with SSE transport
func (s *Server) RunHTTP(port int) error {
	log.Println("========================================")
	log.Printf("%s MCP Server Starting", s.config.Name)
	log.Printf("Version: %s", s.config.Version)
	log.Printf("Mode: HTTP (SSE)")
	log.Printf("Port: %d", port)
	log.Println("========================================")

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, stopping server...")
		cancel()
	}()

	mux := http.NewServeMux()

	// OAuth discovery endpoints (RFC 8414 & RFC 9728) - MUST be first for proper routing
	mux.HandleFunc("/.well-known/oauth-protected-resource", s.handleOAuthProtectedResourceMetadata)
	mux.HandleFunc("/.well-known/oauth-authorization-server", s.handleOAuthAuthorizationServerMetadata)

	// OAuth 2.0 endpoints
	mux.HandleFunc("/register", s.oauthManager.HandleRegister)
	mux.HandleFunc("/authorize", s.oauthManager.HandleAuthorize)
	mux.HandleFunc("/token", s.oauthManager.HandleToken)

	// SSE endpoint - handles GET (connection) and POST (messages)
	mux.HandleFunc("/sse", s.handleSSE)
	mux.HandleFunc("/", s.handleSSE)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","server":"%s","version":"%s"}`, s.config.Name, s.config.Version)
	})

	// Tools list endpoint
	mux.HandleFunc("/tools", s.handleListTools)

	// Apply CORS middleware
	handler := s.corsMiddleware(mux)

	addr := fmt.Sprintf(":%d", port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Run server in goroutine
	errChan := make(chan error, 1)
	go func() {
		log.Printf("HTTP server listening on http://localhost%s", addr)
		log.Println("SSE endpoint: /sse")
		log.Println("Health check: /health")
		log.Println("========================================")
		log.Println("Ready to accept MCP requests!")
		log.Println("========================================")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case <-ctx.Done():
		log.Println("Shutting down HTTP server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
			return err
		}
		log.Println("Server stopped gracefully")
		return nil

	case err := <-errChan:
		log.Printf("HTTP server error: %v", err)
		return err
	}
}

// handleSSE handles Server-Sent Events endpoint for MCP protocol
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Handle POST requests (MCP messages)
	if r.Method == http.MethodPost {
		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Read and parse message
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Failed to read request body: %v", err)
			fmt.Fprintf(w, "event: error\ndata: {\"error\":\"Failed to read request\"}\n\n")
			flusher.Flush()
			return
		}

		var message map[string]interface{}
		if err := json.Unmarshal(body, &message); err != nil {
			log.Printf("Failed to parse JSON: %v", err)
			fmt.Fprintf(w, "event: error\ndata: {\"error\":\"Invalid JSON\"}\n\n")
			flusher.Flush()
			return
		}

		method, _ := message["method"].(string)

		// Handle different MCP methods
		var response interface{}

		// Allow initialize and tools/list without authentication
		if method != "initialize" && method != "tools/list" {
			// Extract and validate access token
			accessToken := extractBearerToken(r)
			if accessToken == "" {
				errorResponse := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      message["id"],
					"error": map[string]interface{}{
						"code":    -32001,
						"message": "Authentication required",
					},
				}
				responseJSON, _ := json.Marshal(errorResponse)
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(responseJSON))
				flusher.Flush()
				return
			}

			// Validate token with OAuth manager and get authenticated client
			allfundsClient, err := s.oauthManager.GetClient(accessToken)
			if err != nil {
				errorResponse := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      message["id"],
					"error": map[string]interface{}{
						"code":    -32001,
						"message": fmt.Sprintf("Invalid or expired token: %v", err),
					},
				}
				responseJSON, _ := json.Marshal(errorResponse)
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(responseJSON))
				flusher.Flush()
				return
			}

			// Handle authenticated methods
			if method == "tools/call" {
				response = s.handleToolCall(message, allfundsClient)
			}
		}

		// Handle methods that don't require authentication
		if response == nil {
			switch method {
			case "initialize":
				response = s.handleInitialize(message)
			case "tools/list":
				response = s.handleToolsList(message)
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
		}

		// Send response
		responseJSON, _ := json.Marshal(response)
		fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(responseJSON))
		flusher.Flush()
		return
	}

	// Handle GET requests (SSE connection)
	if r.Method == http.MethodGet {
		// Check for Bearer token
		accessToken := extractBearerToken(r)
		if accessToken == "" {
			s.sendOAuthUnauthorized(w, r, 0)
			return
		}

		// Validate token
		_, err := s.oauthManager.GetClient(accessToken)
		if err != nil {
			s.sendOAuthUnauthorized(w, r, 0)
			return
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Send connection event
		fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
		flusher.Flush()

		// Keep connection alive
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				fmt.Fprintf(w, "event: ping\ndata: {\"timestamp\":%d}\n\n", time.Now().Unix())
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

// handleInitialize handles MCP initialize request
func (s *Server) handleInitialize(message map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      message["id"],
		"result": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    s.config.Name,
				"version": s.config.Version,
			},
		},
	}
}

// handleToolsList returns list of available tools
func (s *Server) handleToolsList(message map[string]interface{}) map[string]interface{} {
	mcpTools := make([]map[string]interface{}, len(s.toolDefs))

	for i, toolDef := range s.toolDefs {
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

// handleToolCall executes a tool with authenticated client
func (s *Server) handleToolCall(message map[string]interface{}, allfundsClient *client.Client) map[string]interface{} {
	log.Printf("[ToolCall] Received tool call request")

	params, ok := message["params"].(map[string]interface{})
	if !ok {
		log.Printf("[ToolCall] ERROR: Invalid params in message")
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
	log.Printf("[ToolCall] Tool: %s, Arguments: %+v", toolName, arguments)

	// Find tool definition
	var toolDef *tools.ToolDefinition
	for _, td := range s.toolDefs {
		if td.Name == toolName {
			toolDef = td
			break
		}
	}

	if toolDef == nil {
		log.Printf("[ToolCall] ERROR: Tool not found: %s (available tools: %d)", toolName, len(s.toolDefs))
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      message["id"],
			"error": map[string]interface{}{
				"code":    -32602,
				"message": fmt.Sprintf("Tool not found: %s", toolName),
			},
		}
	}
	log.Printf("[ToolCall] Tool definition found: %s", toolName)

	// Create handler factory with authenticated client
	log.Printf("[ToolCall] Creating handler factory with authenticated client")
	factory := handlers.NewHandlerFactory(allfundsClient)
	handler := factory.CreateHandler(toolName)

	// Execute tool handler
	log.Printf("[ToolCall] Executing tool handler for: %s", toolName)
	ctx := context.Background()
	result, _, err := handler(ctx, nil, arguments)
	if err != nil {
		log.Printf("[ToolCall] ERROR: Tool execution failed: tool=%s error=%v", toolName, err)
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      message["id"],
			"error": map[string]interface{}{
				"code":    -32603,
				"message": err.Error(),
				"tool":    toolName,
			},
		}
	}

	// Check if result is an error
	if result.IsError {
		errorText := ""
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
				errorText = textContent.Text
			}
		}
		log.Printf("[ToolCall] ERROR: Tool returned error result: tool=%s error=%s", toolName, errorText)
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      message["id"],
			"error": map[string]interface{}{
				"code":    -32603,
				"message": errorText,
				"tool":    toolName,
			},
		}
	}

	// Extract text from result
	resultText := ""
	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
			resultText = textContent.Text
		}
	}

	log.Printf("[ToolCall] SUCCESS: Tool executed successfully: %s (result length: %d chars)", toolName, len(resultText))

	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      message["id"],
		"result": map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": resultText,
				},
			},
		},
	}
}

// handleListTools returns available tools (public endpoint for discovery)
func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	toolsList := make([]map[string]interface{}, len(s.toolDefs))

	for i, toolDef := range s.toolDefs {
		toolsList[i] = map[string]interface{}{
			"name":        toolDef.Name,
			"description": toolDef.Description,
		}
	}

	response := map[string]interface{}{
		"tools": toolsList,
		"count": len(toolsList),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

// handleOAuthProtectedResourceMetadata implements RFC 9728
// Returns OAuth 2.0 Protected Resource Metadata for Claude Desktop discovery
func (s *Server) handleOAuthProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
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
// Returns OAuth Authorization Server Metadata for Claude Desktop discovery
func (s *Server) handleOAuthAuthorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	baseURL := getBaseURL(r)

	metadata := map[string]interface{}{
		"issuer":                                baseURL,
		"authorization_endpoint":                fmt.Sprintf("%s/authorize", baseURL),
		"token_endpoint":                        fmt.Sprintf("%s/token", baseURL),
		"registration_endpoint":                 fmt.Sprintf("%s/register", baseURL),
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metadata)
}

// sendOAuthUnauthorized sends 401 with WWW-Authenticate header for OAuth discovery
func (s *Server) sendOAuthUnauthorized(w http.ResponseWriter, r *http.Request, port int) {
	baseURL := getBaseURL(r)
	discoveryURL := fmt.Sprintf("%s/.well-known/oauth-protected-resource", baseURL)

	w.Header().Set("WWW-Authenticate", fmt.Sprintf("Bearer resource_metadata=\"%s\"", discoveryURL))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   "unauthorized",
		"message": "Authentication required. Please authenticate using OAuth.",
	})
}

// getBaseURL constructs the base URL from the request, detecting HTTPS from headers
func getBaseURL(r *http.Request) string {
	scheme := "http"

	// Check X-Forwarded-Proto header (set by reverse proxies like Cloudflare, nginx)
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS != nil {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

// corsMiddleware adds CORS headers
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
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

// GetMCPServer returns the underlying MCP server
func (s *Server) GetMCPServer() *mcp.Server {
	return s.mcp
}
