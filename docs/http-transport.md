# HTTP/SSE Transport

The MCP servers support HTTP transport with Server-Sent Events (SSE) for persistent connections.

## Overview

HTTP mode allows MCP servers to run as persistent services that can handle multiple client connections, unlike stdio mode which is designed for single-process communication.

## Features

- **SSE Connection**: GET `/sse` establishes a long-lived connection with ping/keepalive
- **Message Handling**: POST `/sse` processes MCP JSON-RPC messages
- **Health Check**: GET `/health` returns server status
- **CORS Support**: Allows cross-origin requests for web clients
- **OAuth Discovery**: RFC 9728/8414 compliant discovery endpoints for Claude Desktop

## Starting in HTTP Mode

```bash
# ZenCRM on port 8080
./bin/zencrm-mcp -mode=http -port=8080

# Allfunds on port 8081
./bin/allfunds-mcp -mode=http -port=8081
```

## Endpoints

### `GET /sse`
Establishes an SSE connection. Sends periodic ping events to keep connection alive.

**Response:**
```
Content-Type: text/event-stream

event: endpoint
data: /sse

: ping
```

### `POST /sse`
Processes MCP JSON-RPC messages and returns responses as SSE events.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list",
  "params": {}
}
```

**Response:**
```
Content-Type: text/event-stream

event: message
data: {"jsonrpc":"2.0","id":1,"result":{"tools":[...]}}
```

### `GET /health`
Returns server health status.

**Response:**
```json
{
  "status": "healthy",
  "server": "zencrm",
  "version": "1.0.0"
}
```

### OAuth Discovery Endpoints

#### `GET /.well-known/oauth-protected-resource`
Returns OAuth 2.0 Protected Resource Metadata (RFC 9728).

**Response:**
```json
{
  "resource": "http://localhost:8080",
  "authorization_servers": ["http://localhost:8080"],
  "bearer_methods_supported": ["header"],
  "scopes_supported": []
}
```

#### `GET /.well-known/oauth-authorization-server`
Returns OAuth 2.0 Authorization Server Metadata (RFC 8414).

**Response:**
```json
{
  "issuer": "http://localhost:8080",
  "authorization_endpoint": "http://localhost:8080/authorize",
  "token_endpoint": "http://localhost:8080/token",
  "registration_endpoint": "http://localhost:8080/register",
  "response_types_supported": ["code"],
  "grant_types_supported": ["authorization_code", "refresh_token"],
  "code_challenge_methods_supported": ["S256"],
  "token_endpoint_auth_methods_supported": ["none"]
}
```

When accessing `/sse` without authentication:
- Returns `401 Unauthorized`
- Includes `WWW-Authenticate: Bearer resource_metadata="..."` header
- Allows Claude Desktop to discover OAuth capabilities

## VS Code Debugging

Use the provided launch configurations:

- **ZenCRM MCP Server (HTTP/SSE) - Port 8080**
- **Allfunds MCP Server (HTTP/SSE) - Port 8081**

## OAuth Flow Implementation

The shared transport provides OAuth **discovery** endpoints required by Claude Desktop. The full OAuth flow requires service-specific implementation:

**Required Endpoints:**
- `/register` - Dynamic Client Registration (RFC 7591)
- `/authorize` - Authorization request with PKCE (S256)
- `/token` - Token exchange (authorization_code + refresh_token grants)
- `/callback` - Post-login callback handler

**Reference Implementation:**
See `/Users/E67244/Documents/Criteria/ZenCRM-MCP/internal/transport/` for a complete OAuth 2.0 + PKCE implementation that:
- Handles dynamic client registration
- Redirects to login page for authorization
- Exchanges auth codes for JWT access tokens
- Supports refresh tokens for long-lived sessions
- Stores sessions in backend API

## Current Limitations

The shared HTTP transport provides:
- ✅ SSE connection management with keepalive
- ✅ OAuth discovery endpoints (RFC 9728/8414)
- ✅ CORS support
- ✅ Health checks

Services must implement:
- ⚠️ OAuth flow endpoints (register, authorize, token, callback)
- ⚠️ MCP message handling (initialize, tools/list, tools/call)
- ⚠️ Authentication and session management

**For immediate full MCP functionality, use stdio mode.**

## Extending HTTP Transport

Services can extend the base SSE transport by:

1. Implementing custom message handlers in their service layer
2. Adding authentication/authorization middleware
3. Adding service-specific endpoints
4. Implementing OAuth flows (see ZenCRM-MCP for example)

The base transport (`shared/pkg/transport/sse.go`) provides reusable SSE utilities:
- `SetSSEHeaders(w)` - Set SSE response headers
- `WriteSSEMessage(w, data)` - Send SSE message event
- `WriteSSEError(w, error)` - Send SSE error event
- `HandleConnection(w, r)` - Maintain SSE connection with keepalive
