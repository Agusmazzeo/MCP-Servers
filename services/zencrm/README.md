# ZenCRM MCP Server

MCP server for the ZenCRM API, providing 100+ tools for managing clients, contacts, interactions, users, teams, and more.

## Features

- **100+ Tools**: Comprehensive CRM management capabilities
- **OAuth 2.0 Support**: Automatic authentication with persistent sessions
- **TOON Format**: Efficient data transfer (70-80% token reduction)
- **Export Generation**: CSV/XLSX export for large datasets
- **Frontend Integration**: Direct links to CRM frontend
- **Observability**: Structured logging and tracing support

## Installation

```bash
# Build the server
cd services/zencrm
go build -o ../../bin/zencrm-mcp ./cmd

# Or use the Makefile from root
cd ../..
make build-zencrm
```

## Configuration

Copy `.env.example` to `.env` and configure:

```env
# ZenCRM API Configuration
API_BASE_URL=http://localhost:8000
FRONTEND_BASE_URL=http://localhost:3000

# Optional: JWT Token for stdio mode
# JWT_TOKEN=your_jwt_token_here

# HTTP Configuration
HTTP_TIMEOUT=60

# Observability
ENVIRONMENT=development
LOG_LEVEL=info
```

## Usage

### Stdio Mode (Claude Desktop)

```bash
./bin/zencrm-mcp -mode=stdio
```

Configure in Claude Desktop's `claude_desktop_config.json`:

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

### HTTP Mode (Coming Soon)

```bash
./bin/zencrm-mcp -mode=http -port=8080
```

## Available Tools

The ZenCRM MCP server provides tools for:

- **Authentication**: `login`
- **Clients**: `list_clients`, `get_client`, `create_client`, `update_client`, `delete_client`
- **Contacts**: `list_contacts`, `get_contact`, `create_contact`, `update_contact`, `delete_contact`
- **Interactions**: `list_interactions`, `get_interaction`, `create_interaction`, `update_interaction`
- **Users**: `list_users`, `get_user`, `create_user`, `update_user`
- **Teams**: `list_teams`, `get_team`, `create_team`, `update_team`
- **Accounts**: `list_accounts`, `get_account`, `create_account`, `update_account`
- **Reminders**: `list_reminders`, `create_reminder`, `update_reminder`, `delete_reminder`
- **Campaigns**: `list_campaigns`, `create_campaign`, `update_campaign`, `delete_campaign`
- **Metadata**: Various `list_*_types`, `list_*_statuses` tools
- **Exports**: `generate_*_export_url` tools
- **Search**: `semantic_search` and `orchestrate` for AI-powered queries

See `tools/tools.json` for the complete list with detailed schemas.

## Authentication

### OAuth Mode (Recommended)

When OAuth is configured in Claude Desktop, authentication is automatic. No need to call the login tool.

### Manual Mode

For stdio mode without OAuth:

```
Call the login tool with email and password
```

## Development

### Project Structure

```
zencrm/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── config/              # Configuration
│   ├── apiclient/           # ZenCRM API client
│   │   ├── client.go        # HTTP client & execution
│   │   ├── routes.go        # Route definitions
│   │   ├── helpers.go       # Helper functions
│   │   └── errors.go        # Error handling
│   ├── handlers/            # Tool handlers
│   │   └── factory.go       # Handler factory
│   └── metadata/            # Frontend URL generation
│       └── enhancer.go      # Metadata enhancer
├── tools/
│   └── tools.json           # Tool definitions
└── go.mod
```

### Adding New Tools

1. Add tool definition to `tools/tools.json`
2. Add route configuration to `internal/apiclient/routes.go`
3. (Optional) Add metadata generation to `internal/metadata/enhancer.go`
4. Rebuild and test

## License

See main repository LICENSE file.
