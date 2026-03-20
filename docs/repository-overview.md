# MCP-Servers Repository Overview

Multi-service MCP (Model Context Protocol) server repository with shared skeleton architecture.

## What Is This?

A framework for building MCP servers that allows:
- **Shared infrastructure**: Common code for all services
- **Service-specific logic**: Each service implements its own business logic
- **Easy scaling**: Add new services quickly
- **Consistent patterns**: Follow established architecture

## Repository Structure

```
MCP-Servers/
├── shared/              # Reusable MCP infrastructure
├── services/            # Individual service implementations
│   ├── zencrm/         # ✅ Production ready
│   └── allfunds/       # 🚧 In progress
├── bin/                # Built binaries
├── docs/               # Documentation
├── go.work             # Go workspace
└── Makefile            # Build automation
```

## Current Services

### ZenCRM ✅
MCP server for ZenCRM API - 100+ tools for CRM management.

**Features**:
- Client/contact/interaction management
- User and team tools
- Campaign and reminder management
- Export generation
- Semantic search

**Status**: Production ready
**Build**: `make build-zencrm`

### Allfunds 🚧
MCP server for Allfunds Connect platform via GraphQL.

**Features** (planned):
- Fund search and filtering
- Performance data
- Document management
- Portfolio analysis

**Status**: Foundation ready (~25% complete)

## Key Features

### Shared Skeleton
- MCP server initialization
- Tool loading from JSON
- Tool registration framework
- Transport layer (stdio/HTTP)
- Configuration utilities

### Service Pattern
Each service:
1. Implements `HandlerFactory` interface
2. Defines tools in JSON
3. Provides API client
4. Has own configuration

### Build System
- Go workspace for multi-module development
- Makefile for automated builds
- Modular dependencies

## Quick Start

### Using ZenCRM

```bash
# Build
make build-zencrm

# Run
./bin/zencrm-mcp -mode=stdio
```

Configure in Claude Desktop:
```json
{
  "mcpServers": {
    "zencrm": {
      "command": "/path/to/bin/zencrm-mcp",
      "args": ["-mode=stdio"],
      "env": {
        "API_BASE_URL": "http://localhost:8000"
      }
    }
  }
}
```

### Creating a New Service

See: [creating-services.md](creating-services.md)

## Documentation

- [architecture.md](architecture.md) - System design
- [creating-services.md](creating-services.md) - How to add new services
- [repository-overview.md](repository-overview.md) - This file

## Development

```bash
# Install dependencies
make install-deps

# Build all services
make build-all

# Run tests
make test

# Clean
make clean
```

## Architecture Benefits

- **Reusable**: Common infrastructure shared
- **Scalable**: Easy to add services
- **Maintainable**: Changes in one place
- **Testable**: Isolated components
- **Documented**: Clear patterns and guides
