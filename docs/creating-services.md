# Creating a New MCP Service

Guide for adding a new service to the MCP-Servers repository.

## Prerequisites

- Understanding of the target API
- Go 1.24.11 or higher
- Familiarity with MCP protocol (optional)

## Quick Start

### 1. Create Service Directory

```bash
mkdir -p services/myservice/{cmd,internal/{config,handlers,apiclient},tools}
```

### 2. Create `go.mod`

File: `services/myservice/go.mod`

```go
module github.com/Criteria/MCP-Servers/services/myservice

go 1.24.11

require (
    github.com/Criteria/MCP-Servers/shared v0.0.0
    github.com/modelcontextprotocol/go-sdk v1.4.0
)

replace github.com/Criteria/MCP-Servers/shared => ../../shared
```

### 3. Create Configuration

File: `services/myservice/internal/config/config.go`

```go
package config

import (
    sharedconfig "github.com/Criteria/MCP-Servers/shared/pkg/config"
    "github.com/joho/godotenv"
)

type Config struct {
    APIURL   string
    APIKey   string
    // Add service-specific fields
}

func LoadConfig() (*Config, error) {
    _ = godotenv.Load()

    apiURL, err := sharedconfig.RequireEnv("MY_SERVICE_API_URL")
    if err != nil {
        return nil, err
    }

    apiKey, err := sharedconfig.RequireEnv("MY_SERVICE_API_KEY")
    if err != nil {
        return nil, err
    }

    return &Config{
        APIURL: apiURL,
        APIKey: apiKey,
    }, nil
}
```

### 4. Create API Client

File: `services/myservice/internal/apiclient/client.go`

```go
package apiclient

import (
    "context"
    "net/http"
    "time"
)

type Client struct {
    httpClient *http.Client
    apiURL     string
    apiKey     string
}

func NewClient(apiURL, apiKey string) *Client {
    return &Client{
        httpClient: &http.Client{Timeout: 30 * time.Second},
        apiURL:     apiURL,
        apiKey:     apiKey,
    }
}

func (c *Client) Execute(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error) {
    // Implement your API calls here
    return nil, nil
}
```

### 5. Create Handler Factory

File: `services/myservice/internal/handlers/factory.go`

```go
package handlers

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/Criteria/MCP-Servers/services/myservice/internal/apiclient"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

type HandlerFactory struct {
    client *apiclient.Client
}

func NewHandlerFactory(client *apiclient.Client) *HandlerFactory {
    return &HandlerFactory{client: client}
}

func (f *HandlerFactory) CreateHandler(toolName string) func(context.Context, *mcp.CallToolRequest, map[string]interface{}) (*mcp.CallToolResult, any, error) {
    return func(ctx context.Context, req *mcp.CallToolRequest, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
        result, err := f.client.Execute(ctx, toolName, args)
        if err != nil {
            return &mcp.CallToolResult{
                IsError: true,
                Content: []mcp.Content{
                    &mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
                },
            }, nil, nil
        }

        data, _ := json.MarshalIndent(result, "", "  ")
        return &mcp.CallToolResult{
            Content: []mcp.Content{
                &mcp.TextContent{Text: string(data)},
            },
        }, nil, nil
    }
}

func (f *HandlerFactory) CreateLoginHandler() func(context.Context, *mcp.CallToolRequest, map[string]interface{}) (*mcp.CallToolResult, any, error) {
    // Implement if your service needs authentication
    return func(ctx context.Context, req *mcp.CallToolRequest, args map[string]interface{}) (*mcp.CallToolResult, any, error) {
        return &mcp.CallToolResult{
            Content: []mcp.Content{
                &mcp.TextContent{Text: "✓ Authenticated"},
            },
        }, nil, nil
    }
}
```

### 6. Define Tools

File: `services/myservice/tools/tools.json`

```json
[
  {
    "name": "my_tool",
    "description": "Description of what this tool does",
    "input_schema": {
      "type": "object",
      "properties": {
        "param1": {
          "type": "string",
          "description": "Description of param1"
        },
        "param2": {
          "type": "number",
          "description": "Description of param2"
        }
      },
      "required": ["param1"]
    }
  }
]
```

### 7. Create Main Entry Point

File: `services/myservice/cmd/main.go`

```go
package main

import (
    "flag"
    "log"
    "os"

    "github.com/Criteria/MCP-Servers/services/myservice/internal/apiclient"
    "github.com/Criteria/MCP-Servers/services/myservice/internal/config"
    "github.com/Criteria/MCP-Servers/services/myservice/internal/handlers"
    "github.com/Criteria/MCP-Servers/shared/pkg/server"
    "github.com/Criteria/MCP-Servers/shared/pkg/tools"
)

func main() {
    log.SetOutput(os.Stderr)

    mode := flag.String("mode", "stdio", "Transport mode: stdio or http")
    flag.Parse()

    // Load config
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Create API client
    client := apiclient.NewClient(cfg.APIURL, cfg.APIKey)

    // Load tools
    toolDefs, err := tools.LoadTools("tools/tools.json")
    if err != nil {
        log.Fatalf("Failed to load tools: %v", err)
    }

    // Create MCP server
    mcpServer := server.NewServer(&server.ServerConfig{
        Name:    "myservice",
        Version: "1.0.0",
    })

    // Register tools
    handlerFactory := handlers.NewHandlerFactory(client)
    mcpServer.RegisterTools(handlerFactory, toolDefs, true)

    // Run
    if *mode == "stdio" {
        if err := mcpServer.RunStdio(); err != nil {
            log.Fatalf("Server error: %v", err)
        }
    }
}
```

### 8. Create `.env.example`

File: `services/myservice/.env.example`

```env
MY_SERVICE_API_URL=https://api.example.com
MY_SERVICE_API_KEY=your_api_key_here
```

### 9. Add to Workspace

```bash
echo "    ./services/myservice" >> go.work
```

### 10. Add to Makefile

```makefile
build-myservice: ## Build MyService MCP server
	@echo "Building MyService MCP server..."
	cd services/myservice && go build -o ../../bin/myservice-mcp ./cmd
	@echo "✓ Built: bin/myservice-mcp"
```

### 11. Build and Test

```bash
# Tidy modules
cd services/myservice && go mod tidy

# Build
cd ../..
make build-myservice

# Test
./bin/myservice-mcp -mode=stdio
```

## Best Practices

1. **Configuration**: Use environment variables for all settings
2. **Error Handling**: Provide clear, actionable error messages
3. **Logging**: Use `log.SetOutput(os.Stderr)` to avoid interfering with MCP protocol
4. **Tool Naming**: Use clear, descriptive names (e.g., `list_items`, `create_item`)
5. **Documentation**: Document tool purpose and parameters clearly

## Examples

See existing services:
- **ZenCRM**: REST API example (`services/zencrm/`)
- **Allfunds**: GraphQL API example (`services/allfunds/`)

## API Types

### REST API
Use `net/http` client directly (see ZenCRM example).

### GraphQL API
Create GraphQL client with mutation/query support (see Allfunds example).

### Other APIs
Adapt the pattern to your API type.

## Testing

```bash
# Unit tests
cd services/myservice
go test ./...

# Integration test with Claude Desktop
# 1. Build binary
# 2. Configure in claude_desktop_config.json
# 3. Test with queries
```

## Troubleshooting

**Build fails**: Check `go.mod` has correct module path and replace directive.

**Tools not found**: Verify `tools/tools.json` exists and is valid JSON.

**Handler not called**: Check tool name matches exactly in JSON and handler switch.

**API errors**: Check configuration and credentials in `.env`.
