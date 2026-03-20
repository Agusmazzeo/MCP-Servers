# Allfunds MCP Server

MCP server for Allfunds Connect providing 19 tools for fund research and analysis.

## Features

- **19 Tools**: Search funds, get performance, ratios, documents, portfolio data, and more
- **GraphQL Integration**: Direct integration with Allfunds Connect GraphQL API
- **OAuth Authentication**: Automatic login using Allfunds credentials via OAuth flow
- **Session Management**: Persistent sessions with automatic re-authentication
- **Two Modes**: stdio (for Claude Desktop) and HTTP (for persistent server)

## Quick Start

### 1. Configure Environment

Create `.env` file in the repository root:

```bash
# Required for Allfunds
ALLFUNDS_EMAIL=your@email.com
ALLFUNDS_PASSWORD=your_password

# Optional (defaults shown)
GRAPHQL_URL=https://app.allfunds.com/graphql
```

### 2. Build

```bash
make build-allfunds
```

### 3. Run

**stdio mode (for Claude Desktop):**
```bash
cd services/allfunds
../../bin/allfunds-mcp -mode=stdio
```

**HTTP mode (for remote/persistent connections):**
```bash
cd services/allfunds
../../bin/allfunds-mcp -mode=http -port=8081
```

## Claude Desktop Configuration

### stdio Mode (Simple)

Add to Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "allfunds": {
      "command": "/Users/YOUR_USERNAME/Documents/Criteria/MCP-Servers/bin/allfunds-mcp",
      "args": ["-mode=stdio"],
      "cwd": "/Users/YOUR_USERNAME/Documents/Criteria/MCP-Servers/services/allfunds",
      "env": {
        "GRAPHQL_URL": "https://app.allfunds.com/graphql",
        "ALLFUNDS_EMAIL": "your@email.com",
        "ALLFUNDS_PASSWORD": "your_password"
      }
    }
  }
}
```

### HTTP Mode with OAuth (Advanced)

Add to Claude Desktop config:

```json
{
  "mcpServers": {
    "allfunds": {
      "url": "http://localhost:8081/sse",
      "oauth": {
        "client_id": "your@email.com",
        "client_secret": "your_password"
      }
    }
  }
}
```

**How it works:**
1. Claude Desktop discovers OAuth endpoints via `.well-known` URLs
2. Registers as OAuth client using your Allfunds credentials
3. On authorization request, server automatically logs in via GraphQL `LogIn` mutation
4. Returns access token that maps to authenticated Allfunds GraphQL client
5. All tool calls use the pre-authenticated client

**Start the server:**
```bash
cd services/allfunds
../../bin/allfunds-mcp -mode=http -port=8081
```

## Available Tools

### Fund Search & Discovery
- `search_funds` - Search funds by name, ISIN, or ticker
- `get_similar_funds` - Find alternative/competing funds
- `get_watchlist_funds` - View funds in a watchlist
- `list_watchlists` - List your watchlists
- `get_pinned_watchlist` - Get your pinned watchlist

### Fund Details
- `get_fund_detail` - Complete fund information (costs, AUM, ESG, etc.)
- `get_fund_performance` - YTD, annual returns, cumulative performance
- `get_fund_ratios` - Sharpe, volatility, drawdown, beta, alpha
- `get_fund_documents` - KIID, prospectus, reports
- `get_fund_share_classes` - All share classes for a fund
- `get_fund_managers` - Portfolio managers
- `get_fund_portfolio` - Asset allocation, sectors, countries, top holdings
- `get_fund_nav_history` - Historical NAV data
- `get_fund_nav_on_date` - NAV on specific date

### Analysis & Comparison
- `compare_funds_performance` - Compare performance of multiple funds

### News & Insights
- `get_fund_insights` - Articles and news from Allfunds Connect

### Utilities
- `get_current_user` - Your Allfunds account info
- `login_status` - Check authentication status
- `download_document` - Download fund documents

## OAuth Flow Architecture

### How It Works

1. **Registration** (`/register`):
   - Claude Desktop sends `client_id` (Allfunds email) and `client_secret` (Allfunds password)
   - Server stores credentials for this OAuth client

2. **Authorization** (`/authorize`):
   - Server uses stored credentials to call GraphQL `LogIn` mutation
   - Allfunds returns authenticated session with CSRF token
   - Server generates authorization code and returns to Claude Desktop

3. **Token Exchange** (`/token`):
   - Claude Desktop exchanges auth code for access token
   - Access token = session ID mapping to authenticated GraphQL client
   - Server returns access_token + refresh_token

4. **Tool Calls**:
   - Client uses `Authorization: Bearer <access_token>` header
   - Server looks up pre-authenticated GraphQL client by token
   - Executes tool handler with authenticated client
   - Returns results

5. **Refresh**:
   - When access token expires, Claude Desktop uses refresh token
   - Server re-authenticates with Allfunds GraphQL
   - Returns new access_token + refresh_token

### Session Management

- Sessions stored in memory: `access_token → OAuthState{Client, Email, Password}`
- Each OAuthState contains authenticated Allfunds GraphQL client
- 30 day TTL with automatic cleanup
- Refresh tokens trigger new GraphQL login

## Development

### VS Code Debugging

Use the launch configurations:
- **Allfunds MCP Server (stdio)** - Debug stdio mode
- **Allfunds MCP Server (HTTP/SSE) - Port 8081** - Debug HTTP mode with OAuth

### Project Structure

```
services/allfunds/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   └── config.go        # Environment configuration
│   ├── graphql/
│   │   ├── client.go        # GraphQL client with auth
│   │   └── queries.go       # All GraphQL queries
│   ├── handlers/
│   │   └── factory.go       # Tool handlers (19 tools)
│   └── transport/
│       ├── http.go          # HTTP/SSE transport
│       └── oauth.go         # OAuth 2.0 + PKCE flow
├── tools/
│   └── tools.json           # Tool definitions (MCP schema)
└── README.md
```

## Troubleshooting

### "ALLFUNDS_EMAIL environment variable is required"

Set environment variables in `.env` file or pass via Claude Desktop config.

### OAuth errors in Claude Desktop

1. Check that server is running: `curl http://localhost:8081/health`
2. Verify credentials are correct in `client_id` and `client_secret`
3. Check server logs for GraphQL authentication errors
4. Test GraphQL login manually:
```bash
curl -X POST https://app.allfunds.com/graphql \
  -H "Content-Type: application/json" \
  -d '{"query":"mutation { log_in(email:\"your@email.com\", password:\"yourpass\") { csrf_token errors } }"}'
```

### "authentication required" in tools

Access token expired. Claude Desktop will automatically use refresh token to get new credentials.

### Tools return errors

Check server logs (`stderr`) for detailed error messages from Allfunds GraphQL API.
