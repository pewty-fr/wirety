---
id: mcp
title: MCP Server
sidebar_position: 9
---

Wirety embeds a [Model Context Protocol (MCP)](https://modelcontextprotocol.io) server directly in the main binary. It exposes Wirety's capabilities as AI-callable tools, letting Claude (or any MCP-compatible assistant) explore and manage your networks.

## Endpoint

```
GET/POST /mcp
```

Transport: **Streamable HTTP** (MCP 2025-03-26 spec). The same server binary serves both the REST API and the MCP endpoint — no extra process needed.

## Authentication

MCP uses the same API tokens as the REST API. Create one from your profile in the UI (Profile → API Tokens → New Token), then pass it as a header:

```
Authorization: Bearer wirety_<64-hex-chars>
```

Permissions are enforced per-token — an admin token can call admin-only tools; a regular user token cannot.

## Available Tools

### Users
| Tool | Description | Admin only |
|------|-------------|-----------|
| `get_current_user` | Get the authenticated user profile | No |
| `list_users` | List all users | Yes |

### Networks
| Tool | Description | Admin only |
|------|-------------|-----------|
| `list_networks` | List accessible WireGuard networks | No |
| `get_network` | Get network details by ID | No |
| `create_network` | Create a new network | Yes |
| `update_network` | Update network name/DNS | Yes |
| `delete_network` | Delete a network | Yes |

### Peers
| Tool | Description | Admin only |
|------|-------------|-----------|
| `list_peers` | List peers in a network | No |
| `get_peer` | Get peer details | No |
| `create_peer` | Create a new peer | No |
| `update_peer` | Rename a peer | No |
| `delete_peer` | Delete a peer | No |
| `get_peer_config` | Get WireGuard config file for a peer | No |

### Groups *(requires DB)*
| Tool | Description | Admin only |
|------|-------------|-----------|
| `list_groups` | List groups in a network | No |
| `create_group` | Create a new group | Yes |
| `update_group` | Update a group's name, description, or priority | Yes |

### Policies *(requires DB)*
| Tool | Description | Admin only |
|------|-------------|-----------|
| `list_policies` | List policies in a network | No |
| `create_policy` | Create a new policy with rules | Yes |
| `update_policy` | Update a policy's name or description | Yes |

### Routes *(requires DB)*
| Tool | Description | Admin only |
|------|-------------|-----------|
| `list_routes` | List routes in a network | No |
| `create_route` | Create a route (destination CIDR via jump peer) | Yes |
| `update_route` | Update a route's configuration | Yes |

Groups, policies, and routes tools are only registered when the database backend is enabled (`DB_ENABLED=true`).

## Claude Code Setup

Add to `~/.claude/settings.json` (user-level, all projects) or `.mcp.json` (project-level):

```json
{
  "mcpServers": {
    "wirety": {
      "type": "http",
      "url": "http://localhost:8080/mcp",
      "headers": {
        "Authorization": "Bearer wirety_<your-token>"
      }
    }
  }
}
```

## Claude Desktop Setup

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS):

```json
{
  "mcpServers": {
    "wirety": {
      "type": "http",
      "url": "http://localhost:8080/mcp",
      "headers": {
        "Authorization": "Bearer wirety_<your-token>"
      }
    }
  }
}
```

Restart Claude Desktop after editing the config.

## Troubleshooting

| Problem | Cause | Fix |
|---------|-------|-----|
| "not valid MCP server configurations" in Claude Desktop | Missing `"type": "http"` | Add `"type": "http"` to the server config |
| 401 Unauthorized | Invalid or expired token | Re-create the token in the UI |
| Tools missing (groups, policies, routes) | DB not enabled | Set `DB_ENABLED=true` and configure `DB_DSN` |
| MCP works via curl but not Claude | Wrong transport | Ensure server was rebuilt after the Streamable HTTP migration |
