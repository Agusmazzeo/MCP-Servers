package tools

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandlerFactory creates tool handlers for a given service
type HandlerFactory interface {
	CreateHandler(toolName string) func(context.Context, *mcp.CallToolRequest, map[string]interface{}) (*mcp.CallToolResult, any, error)
}

// RegisterTools registers all tools with the MCP server
func RegisterTools(server *mcp.Server, factory HandlerFactory, toolDefs []ToolDefinition, includeLogin bool) {
	log.Printf("Registering %d tools with MCP server...", len(toolDefs))

	// Register all API tools
	for _, toolDef := range toolDefs {
		// Create MCP tool from definition
		tool := &mcp.Tool{
			Name:        toolDef.Name,
			Description: toolDef.Description,
			InputSchema: toolDef.InputSchema,
		}

		// Create handler for this tool
		handler := factory.CreateHandler(toolDef.Name)

		// Register with MCP server
		mcp.AddTool(server, tool, handler)

		log.Printf("  ✓ Registered tool: %s", toolDef.Name)
	}

	log.Printf("Successfully registered all %d tools!", len(toolDefs))
}
