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

```bash
# Build specific service
make build-zencrm

# Build all services
make build-all

# Install dependencies
make install-deps

# Run tests
make test

# Clean
make clean
```

## Services

### ZenCRM ✅
CRM management with 100+ tools - Production ready

### Allfunds 🚧
Fund platform via GraphQL - In progress

