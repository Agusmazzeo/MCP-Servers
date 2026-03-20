package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Agusmazzeo/allfunds-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HTTPTransport provides HTTP/SSE transport for MCP protocol
type HTTPTransport struct {
	port     int
	server   *mcp.Server
	toolDefs []*tools.ToolDefinition
	executor ToolExecutor
}

// ToolExecutor interface for service-specific tool execution
type ToolExecutor interface {
	ExecuteTool(ctx context.Context, toolDef *tools.ToolDefinition, args map[string]interface{}) (string, error)
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(port int, server *mcp.Server, toolDefs []*tools.ToolDefinition, executor ToolExecutor) *HTTPTransport {
	return &HTTPTransport{
		port:     port,
		server:   server,
		toolDefs: toolDefs,
		executor: executor,
	}
}

// Start starts the HTTP server
func (t *HTTPTransport) Start() error {
	mux := http.NewServeMux()

	// MCP protocol endpoints
	mux.HandleFunc("/message", t.handleMessage)
	mux.HandleFunc("/sse", t.handleSSE)
	mux.HandleFunc("/", t.handleSSE)

	// Utility endpoints
	mux.HandleFunc("/health", t.handleHealth)
	mux.HandleFunc("/tools", t.handleListTools)

	// Apply middleware
	handler := corsMiddleware(mux)

	addr := fmt.Sprintf(":%d", t.port)
	return http.ListenAndServe(addr, handler)
}

// handleMessage processes MCP messages via HTTP POST
func (t *HTTPTransport) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var message map[string]interface{}
	if err := json.Unmarshal(body, &message); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	method, _ := message["method"].(string)
	var response interface{}

	switch method {
	case "tools/list":
		response = t.handleToolsList(message)
	case "tools/call":
		response = t.handleToolCall(message)
	case "initialize":
		response = t.handleInitialize(message)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleToolsList returns the list of available tools
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
func (t *HTTPTransport) handleToolCall(message map[string]interface{}) map[string]interface{} {
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

	// Execute tool
	ctx := context.Background()
	result, err := t.executor.ExecuteTool(ctx, toolDef, arguments)
	if err != nil {
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

	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      message["id"],
		"result": map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": result,
				},
			},
		},
	}
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
				"name":    "mcp-server",
				"version": "1.0.0",
			},
		},
	}
}

// handleSSE manages Server-Sent Events connection
func (t *HTTPTransport) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		body, _ := io.ReadAll(r.Body)
		var message map[string]interface{}
		json.Unmarshal(body, &message)

		method, _ := message["method"].(string)
		var response interface{}

		switch method {
		case "tools/list":
			response = t.handleToolsList(message)
		case "tools/call":
			response = t.handleToolCall(message)
		case "initialize":
			response = t.handleInitialize(message)
		default:
			response = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      message["id"],
				"error":   map[string]interface{}{"code": -32601, "message": "Method not found"},
			}
		}

		responseJSON, _ := json.Marshal(response)
		fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(responseJSON))
		flusher.Flush()
		return
	}

	// GET request - keep-alive connection
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

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

// handleHealth returns server health status
func (t *HTTPTransport) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// handleListTools returns available tools
func (t *HTTPTransport) handleListTools(w http.ResponseWriter, r *http.Request) {
	toolsList := make([]map[string]interface{}, len(t.toolDefs))
	for i, toolDef := range t.toolDefs {
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

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ExtractJWT extracts JWT from Authorization header
func ExtractJWT(r *http.Request) string {
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
