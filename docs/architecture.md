# MCP-Servers Architecture

## Overview

Multi-service MCP (Model Context Protocol) server framework with shared skeleton approach.

## Design Principles

1. **Separation of Concerns**: Shared infrastructure vs service-specific logic
2. **Interface-Based Design**: Services implement standard interfaces
3. **Modular Architecture**: Each service is independently buildable
4. **Go Workspace**: Seamless cross-module development

## Directory Structure

```
MCP-Servers/
├── shared/                    # Shared skeleton package
│   ├── pkg/
│   │   ├── server/           # MCP server lifecycle
│   │   ├── transport/        # stdio & HTTP transport
│   │   ├── tools/            # Tool framework
│   │   └── config/           # Base configuration
│   └── go.mod
│
├── services/                  # Service implementations
│   ├── zencrm/               # ZenCRM service
│   └── allfunds/             # Allfunds service (in progress)
│
├── bin/                       # Built binaries
├── docs/                      # Documentation
├── go.work                    # Go workspace
├── Makefile                   # Build automation
└── README.md
```

## Component Architecture

### Shared Package

#### Server Package (`shared/pkg/server`)
- Creates and configures MCP server
- Handles graceful shutdown
- Provides stdio transport runner

#### Tools Package (`shared/pkg/tools`)
- Loads tool definitions from JSON
- Registers tools with MCP server
- Defines handler factory interface

**Key Interface**:
```go
type HandlerFactory interface {
    CreateHandler(toolName string) func(...)
    CreateLoginHandler() func(...)
}
```

#### Transport Package (`shared/pkg/transport`)
- HTTP/SSE transport (optional)
- Tool execution orchestration

#### Config Package (`shared/pkg/config`)
- Environment variable loading
- Base configuration structure

### Service Package

Each service implements:

1. **Configuration** (`internal/config`)
2. **API Client** (`internal/apiclient` or `internal/graphql`)
3. **Handlers** (`internal/handlers`) - implements `HandlerFactory`
4. **Tools** (`tools/tools.json`)
5. **Main** (`cmd/main.go`)

## Data Flow

```
User Request
   ↓
MCP Protocol (stdio/HTTP)
   ↓
MCP Server (shared/pkg/server)
   ↓
Tool Registry (shared/pkg/tools)
   ↓
Handler Factory (service/internal/handlers)
   ↓
API Client (service-specific)
   ↓
External API
   ↓
Response Formatting
   ↓
MCP Protocol Response
```

## Extension Points

### Adding a New Service

1. Create service directory
2. Implement `HandlerFactory` interface
3. Define tools in `tools.json`
4. Create `main.go`
5. Add to workspace

See: [creating-services.md](creating-services.md)

## Interface Contracts

### HandlerFactory

```go
type HandlerFactory interface {
    CreateHandler(toolName string) ToolHandler
    CreateLoginHandler() ToolHandler
}
```

Returns function matching MCP tool handler signature.

## Configuration Strategy

All configuration via environment variables:
- Shared: `HTTP_TIMEOUT`, `ENVIRONMENT`, `LOG_LEVEL`
- Service-specific: `API_BASE_URL`, `GRAPHQL_URL`, credentials

## Build System

- **Go Workspace**: Single `go.work` file
- **Makefile**: `build-zencrm`, `build-allfunds`, etc.
- **Module Dependencies**: Local replacements for shared package
