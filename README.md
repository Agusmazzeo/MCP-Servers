# MCP-Servers

Multi-service MCP (Model Context Protocol) server repository with shared skeleton architecture.

## Quick Start

```bash
# Build ZenCRM service
make build-zencrm

# Run
./bin/zencrm-mcp -mode=stdio
```

See [docs/creating-services.md](docs/creating-services.md) to add a new service.

## Features

- **Shared Infrastructure**: Reusable MCP server components
- **Service Isolation**: Each service has its own logic
- **Easy Scaling**: Add new services quickly
- **Type Safe**: Go implementation with interfaces
- **Well Documented**: Comprehensive guides

## Documentation

- **[docs/repository-overview.md](docs/repository-overview.md)** - Complete overview
- **[docs/architecture.md](docs/architecture.md)** - System design
- **[docs/creating-services.md](docs/creating-services.md)** - Service development guide

## Build Commands

### Local Development
```bash
# Build specific service
make build-zencrm
make build-allfunds

# Build all services
make build-all

# Install dependencies
make install-deps

# Run tests
make test

# Clean
make clean
```

### Docker

```bash
# Build and run with docker-compose
docker-compose up --build

# Build individual services
docker build -f services/zencrm/Dockerfile -t zencrm-mcp .
docker build -f services/allfunds/Dockerfile -t allfunds-mcp .

# Run individual containers
docker run -p 8080:8080 -e ZENCRM_API_URL=$ZENCRM_API_URL -e ZENCRM_API_KEY=$ZENCRM_API_KEY zencrm-mcp
docker run -p 8081:8081 -e GRAPHQL_URL=$GRAPHQL_URL -e EMAIL=$EMAIL -e PASSWORD=$PASSWORD allfunds-mcp
```

## Services

### ZenCRM ✅
CRM management with 100+ tools - Production ready
- HTTP/SSE transport
- Port: 8080

### Allfunds ✅
Fund platform via GraphQL - Production ready
- OAuth 2.0 + PKCE authentication
- HTTP/SSE transport
- Port: 8081

