package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Agusmazzeo/MCP-Servers/services/zencrm/internal/apiclient"
	"github.com/Agusmazzeo/MCP-Servers/services/zencrm/internal/metadata"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandlerFactory creates tool handlers for ZenCRM
type HandlerFactory struct {
	client          *apiclient.Client
	frontendBaseURL string
}

// NewHandlerFactory creates a new handler factory
func NewHandlerFactory(client *apiclient.Client, frontendBaseURL string) *HandlerFactory {
	return &HandlerFactory{
		client:          client,
		frontendBaseURL: frontendBaseURL,
	}
}

// CreateHandler creates a handler function for a specific tool
func (f *HandlerFactory) CreateHandler(toolName string) func(context.Context, *mcp.CallToolRequest, map[string]interface{}) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
		// Execute API call through client
		result, err := f.client.Execute(ctx, toolName, args)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Error executing tool: %v", err)},
				},
			}, nil, nil
		}

		// Check if execution failed
		if !result.Success {
			return &mcp.CallToolResult{
				IsError: result.IsFatalError,
				Content: []mcp.Content{
					&mcp.TextContent{Text: result.Error},
				},
			}, nil, nil
		}

		// Generate metadata with frontend URLs
		metadataMap := metadata.GenerateMetadata(toolName, args, result.Data, f.frontendBaseURL)
		if metadataMap != nil && len(metadataMap) > 0 {
			result.Metadata = metadataMap
		}

		// Format MCP response
		return formatMCPResult(result), nil, nil
	}
}

// CreateLoginHandler creates the login tool handler
func (f *HandlerFactory) CreateLoginHandler() func(context.Context, *mcp.CallToolRequest, map[string]interface{}) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
		// Extract email and password from arguments
		email, emailOk := args["email"].(string)
		password, passwordOk := args["password"].(string)

		if !emailOk || !passwordOk || email == "" || password == "" {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Error: email and password are required"},
				},
			}, nil, nil
		}

		// Call the login method on the client
		err := f.client.Login(ctx, email, password)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Login failed: %v", err)},
				},
			}, nil, nil
		}

		// Success!
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "✓ Successfully authenticated with ZenCRM. You can now use other tools."},
			},
			IsError: false,
		}, nil, nil
	}
}

// formatMCPResult converts ExecuteResult to MCP CallToolResult
func formatMCPResult(result *apiclient.ExecuteResult) *mcp.CallToolResult {
	// Format data and metadata as readable text
	text := formatResultText(result.Data, result.Metadata)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
		IsError: false,
	}
}

// formatResultText formats data and metadata as readable text
func formatResultText(data interface{}, metadataMap map[string]interface{}) string {
	var builder strings.Builder

	fmt.Fprintf(os.Stderr, "[DEBUG formatResultText] Data type: %T\n", data)

	// Format the data
	switch v := data.(type) {
	case string:
		// TOON format or raw text - return as-is
		fmt.Fprintf(os.Stderr, "[DEBUG formatResultText] String data, length: %d\n", len(v))
		builder.WriteString(v)
	case map[string]interface{}:
		// JSON object - pretty print
		jsonData, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			builder.WriteString(fmt.Sprintf("%v", v))
		} else {
			builder.WriteString(string(jsonData))
		}
	default:
		// Other types - convert to JSON
		jsonData, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			builder.WriteString(fmt.Sprintf("%v", v))
		} else {
			builder.WriteString(string(jsonData))
		}
	}

	// Add metadata if present
	if len(metadataMap) > 0 {
		builder.WriteString("\n\n---\nMetadata:\n")
		if url, ok := metadataMap["frontend_url"].(string); ok {
			builder.WriteString(fmt.Sprintf("🔗 Frontend URL: %s\n", url))
		}
		if desc, ok := metadataMap["url_description"].(string); ok {
			builder.WriteString(fmt.Sprintf("   %s\n", desc))
		}
	}

	return builder.String()
}
