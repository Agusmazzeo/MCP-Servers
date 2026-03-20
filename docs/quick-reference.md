# Quick Reference

## Build Commands

```bash
# Build ZenCRM
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

## Run Services

### ZenCRM
```bash
./bin/zencrm-mcp -mode=stdio
```

### Configuration
Create `.env` in service directory:
```bash
cd services/zencrm
cp .env.example .env
# Edit .env with your settings
```

## Claude Desktop Setup

Edit `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "zencrm": {
      "command": "/path/to/bin/zencrm-mcp",
      "args": ["-mode=stdio"],
      "env": {
        "API_BASE_URL": "http://localhost:8000",
        "FRONTEND_BASE_URL": "http://localhost:3000"
      }
    }
  }
}
```

## Creating a New Service

```bash
# 1. Create directory
mkdir -p services/myservice/{cmd,internal/{config,handlers,apiclient},tools}

# 2. Create files
cd services/myservice
# - go.mod
# - internal/config/config.go
# - internal/apiclient/client.go
# - internal/handlers/factory.go
# - tools/tools.json
# - cmd/main.go
# - .env.example

# 3. Add to workspace
echo "    ./services/myservice" >> ../../go.work

# 4. Build
cd ../..
go build -o bin/myservice-mcp ./services/myservice/cmd
```

See [creating-services.md](creating-services.md) for complete guide.

## Directory Structure

```
MCP-Servers/
├── shared/              # Shared infrastructure
├── services/
│   ├── zencrm/         # Production ready
│   └── allfunds/       # In progress
├── bin/                # Built binaries
├── docs/               # Documentation
├── go.work             # Workspace
└── Makefile            # Build automation
```

## Documentation

- [repository-overview.md](repository-overview.md) - What & why
- [architecture.md](architecture.md) - How it works
- [creating-services.md](creating-services.md) - Build new services
- [quick-reference.md](quick-reference.md) - This file

## Common Tasks

### Update shared package
```bash
cd shared
go mod tidy
cd ../services/zencrm
go mod tidy
```

### Test a service
```bash
cd services/zencrm
go test ./...
```

### Check all builds
```bash
make build-all
```

## Troubleshooting

**Build fails**: Run `make tidy` first

**Module errors**: Check `go.work` includes your service

**Tools not found**: Verify `tools/tools.json` path is correct

**Runtime errors**: Check `.env` has required variables
