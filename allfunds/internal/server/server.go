package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Agusmazzeo/allfunds-mcp/internal/tools"
	"github.com/Agusmazzeo/allfunds-mcp/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServerConfig holds MCP server configuration
type ServerConfig struct {
	Name    string
	Version string
}

// Server wraps an MCP server with lifecycle management
type Server struct {
	mcp    *mcp.Server
	config *ServerConfig
}

// NewServer creates a new MCP server
func NewServer(config *ServerConfig) *Server {
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    config.Name,
		Version: config.Version,
	}, nil)

	return &Server{
		mcp:    mcpServer,
		config: config,
	}
}

// RegisterTools registers tools with the server
func (s *Server) RegisterTools(factory tools.HandlerFactory, toolDefs []*tools.ToolDefinition, includeLogin bool) {
	tools.RegisterTools(s.mcp, factory, toolDefs, includeLogin)
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

	// Create SSE handler
	sseHandler := transport.NewSSEHandler()

	mux := http.NewServeMux()

	// OAuth discovery endpoints (required for Claude Desktop OAuth)
	mux.HandleFunc("/.well-known/oauth-protected-resource", s.handleOAuthProtectedResourceMetadata)
	mux.HandleFunc("/.well-known/oauth-authorization-server", s.handleOAuthAuthorizationServerMetadata)

	// SSE endpoint - handles GET (connection) and POST (messages)
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Check for authentication - return 401 with OAuth discovery if not authenticated
			auth := r.Header.Get("Authorization")
			if auth == "" {
				s.sendOAuthUnauthorized(w, r, port)
				return
			}
			sseHandler.HandleConnection(w, r)
		} else if r.Method == http.MethodPost {
			s.handleSSEMessage(w, r)
		} else if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Check for authentication - return 401 with OAuth discovery if not authenticated
			auth := r.Header.Get("Authorization")
			if auth == "" {
				s.sendOAuthUnauthorized(w, r, port)
				return
			}
			sseHandler.HandleConnection(w, r)
		} else if r.Method == http.MethodPost {
			s.handleSSEMessage(w, r)
		} else if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","server":"%s","version":"%s"}`, s.config.Name, s.config.Version)
	})

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

// handleSSEMessage handles MCP messages over SSE POST
func (s *Server) handleSSEMessage(w http.ResponseWriter, r *http.Request) {
	transport.SetSSEHeaders(w)

	// Note: Full MCP message handling will be implemented per-service
	// For now, return a helpful message
	response := `{
		"jsonrpc": "2.0",
		"error": {
			"code": -32601,
			"message": "MCP message handling not yet implemented in shared server. Use stdio mode for full functionality."
		}
	}`

	transport.WriteSSEMessage(w, response)
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
		"message": "Authentication required. Use OAuth to authenticate.",
		"hint":    fmt.Sprintf("OAuth flow not yet implemented. See http://localhost:%d/health for server status.", port),
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
