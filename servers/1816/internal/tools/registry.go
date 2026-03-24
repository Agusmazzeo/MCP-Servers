package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolHandler is a function that handles a tool call
type ToolHandler func(ctx context.Context, req *mcp.CallToolRequest, args map[string]interface{}) (interface{}, map[string]interface{}, error)

// ToolRegistry manages tool handlers
type ToolRegistry struct {
	handlers map[string]ToolHandler
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		handlers: make(map[string]ToolHandler),
	}
}

// Register registers a tool handler
func (r *ToolRegistry) Register(name string, handler ToolHandler) {
	r.handlers[name] = handler
}

// Get retrieves a tool handler
func (r *ToolRegistry) Get(name string) (ToolHandler, bool) {
	handler, ok := r.handlers[name]
	return handler, ok
}
