package tools

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandlerFactory creates tool handlers for a given service
type HandlerFactory interface {
	CreateHandler(toolName string) func(context.Context, *mcp.CallToolRequest, map[string]interface{}) (*mcp.CallToolResult, any, error)
	CreateLoginHandler() func(context.Context, *mcp.CallToolRequest, map[string]interface{}) (*mcp.CallToolResult, any, error)
}

// RegisterTools registers all tools with the MCP server
func RegisterTools(server *mcp.Server, factory HandlerFactory, toolDefs []*ToolDefinition, includeLogin bool) {
	log.Printf("Registering %d tools with MCP server...", len(toolDefs))

	// Register the login tool if needed (optional for services)
	if includeLogin {
		registerLoginTool(server, factory)
	}

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

	loginMsg := ""
	if includeLogin {
		loginMsg = " plus login tool"
	}
	log.Printf("Successfully registered all %d tools%s!", len(toolDefs), loginMsg)
}

// registerLoginTool registers the special login authentication tool
func registerLoginTool(server *mcp.Server, factory HandlerFactory) {
	loginTool := &mcp.Tool{
		Name: "login",
		Description: `Authenticate with the service. This tool has TWO authentication modes:

🔐 OAUTH MODE (Recommended - Automatic):
   When OAuth is configured in Claude Desktop settings, authentication is AUTOMATIC.
   You DO NOT need to call this tool - all tools work immediately!

   How it works:
   1. User sets up OAuth once in Claude Desktop (browser login)
   2. OAuth creates a session with client_id (persistent storage)
   3. MCP server automatically retrieves your JWT token using the session
   4. All tools work automatically with this token

   Benefits:
   - No credentials needed in chat
   - Survives server restarts (stored persistently)
   - Secure OAuth 2.0 flow with PKCE
   - All tools work immediately after OAuth setup

📧 MANUAL MODE (Fallback):
   Only use this if OAuth is NOT configured (stdio mode or testing).
   Requires email and password to authenticate.

   When to use:
   - Running in stdio mode without OAuth
   - OAuth not configured in Claude Desktop
   - Testing/debugging purposes

⚠️  If you see "authentication required" errors with OAuth configured, something is wrong.
    The OAuth session should provide automatic authentication.`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"email": map[string]interface{}{
					"type":        "string",
					"description": "The user's email address for login (only for non-OAuth mode)",
				},
				"password": map[string]interface{}{
					"type":        "string",
					"description": "The user's password for login (only for non-OAuth mode)",
				},
			},
			"required": []string{"email", "password"},
		},
	}

	handler := factory.CreateLoginHandler()
	mcp.AddTool(server, loginTool, handler)
	log.Printf("  ✓ Registered special tool: login")
}
