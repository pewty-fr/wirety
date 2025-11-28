# Groups, Policies, and Routes Overview

This document provides an overview of the new groups, policies, and routes architecture in the WireGuard network management system.

## Quick Links

- **[API Reference](./api-reference.md)** - Complete API documentation for all endpoints
- **[Migration Guide](./migration-guide.md)** - Migrate from legacy ACL and peer flag system
- **[User Guide](./user-guide.md)** - Step-by-step guides for common tasks
- **[Groups Management Guide](./guides/groups-management.md)** - Detailed groups management

## What's New

The system has been redesigned with a modern, flexible architecture:

### Groups
Organize peers into logical collections. Groups serve as the attachment point for policies and routes.

**Key Features:**
- Admin-only management
- Peers can belong to multiple groups
- Non-destructive operations (deleting a group doesn't delete peers)

**Learn More:** [Groups Management Guide](./guides/groups-management.md)

### Policies
Define iptables rules applied on jump peers for traffic filtering. Policies are attached to groups.

**Key Features:**
- Support input/output direction
- Support allow/deny actions
- Target by CIDR, peer, or group
- Built-in templates for common patterns

**Learn More:** [User Guide - Policies](./user-guide.md#policies-management)

### Routes
Define external network destinations accessible through jump peers. Routes are added to WireGuard AllowedIPs.

**Key Features:**
- Specify destination CIDR and jump peer
- Attach to groups for automatic configuration
- Support custom DNS domain suffixes

**Learn More:** [User Guide - Routes](./user-guide.md#routes-management)

### DNS Mappings
Provide name resolution for IP addresses within route networks.

**Key Features:**
- Associate names with IPs in route CIDRs
- FQDN format: `name.route.domain`
- Propagate to jump peer DNS servers

**Learn More:** [User Guide - DNS](./user-guide.md#dns-mappings)

### Default Groups
Automatically assign groups to peers created by non-administrator users.

**Key Features:**
- Admin-only configuration
- Applied only to non-admin created peers
- Useful for baseline policies

**Learn More:** [User Guide - Default Groups](./user-guide.md#default-groups-configuration)

## Breaking Changes

This is a **major breaking change** with no backward compatibility:

### Removed
- ❌ ACL system (tables: `acls`, `acl_rules`)
- ❌ Peer flags: `is_isolated`, `full_encapsulation`
- ❌ Legacy access control endpoints

### Migration Required
All existing ACLs and peer flags must be manually recreated using the new system.

**See:** [Migration Guide](./migration-guide.md)

## Getting Started

### For New Users

1. Read the [User Guide](./user-guide.md) introduction
2. Follow the [Common Workflows](./user-guide.md#common-workflows)
3. Explore the [API Reference](./api-reference.md)

### For Existing Users

1. **Backup your data** - See [Migration Guide - Step 1](./migration-guide.md#step-1-backup-your-data)
2. **Review breaking changes** - See [Migration Guide - Breaking Changes](./migration-guide.md#breaking-changes)
3. **Follow migration steps** - See [Migration Guide - Migration Steps](./migration-guide.md#migration-steps)
4. **Convert ACLs to policies** - See [Converting ACLs to Policies](./migration-guide.md#converting-acls-to-policies)
5. **Convert peer flags** - See [Converting Peer Flags to Policies](./migration-guide.md#converting-peer-flags-to-policies)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Network                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                       Groups                            │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │ │
│  │  │   Group 1    │  │   Group 2    │  │   Group 3    │ │ │
│  │  │              │  │              │  │              │ │ │
│  │  │ Peers: 3     │  │ Peers: 5     │  │ Peers: 2     │ │ │
│  │  │ Policies: 2  │  │ Policies: 1  │  │ Policies: 3  │ │ │
│  │  │ Routes: 1    │  │ Routes: 2    │  │ Routes: 0    │ │ │
│  │  └──────────────┘  └──────────────┘  └──────────────┘ │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                      Policies                           │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │ │
│  │  │  Policy 1    │  │  Policy 2    │  │  Policy 3    │ │ │
│  │  │              │  │              │  │              │ │ │
│  │  │ Rules: 3     │  │ Rules: 2     │  │ Rules: 5     │ │ │
│  │  │ Groups: 2    │  │ Groups: 1    │  │ Groups: 1    │ │ │
│  │  └──────────────┘  └──────────────┘  └──────────────┘ │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                       Routes                            │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │ │
│  │  │   Route 1    │  │   Route 2    │  │   Route 3    │ │ │
│  │  │              │  │              │  │              │ │ │
│  │  │ CIDR: /16    │  │ CIDR: /24    │  │ CIDR: /8     │ │ │
│  │  │ Jump: peer-1 │  │ Jump: peer-1 │  │ Jump: peer-2 │ │ │
│  │  │ DNS: 3       │  │ DNS: 1       │  │ DNS: 0       │ │ │
│  │  │ Groups: 2    │  │ Groups: 1    │  │ Groups: 1    │ │ │
│  │  └──────────────┘  └──────────────┘  └──────────────┘ │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Common Use Cases

### 1. Organize by Department
Create groups for each department and apply appropriate policies and routes.

**Example:** Engineering, Sales, Support groups with different access levels.

**Guide:** [User Guide - Workflow 1](./user-guide.md#workflow-1-setting-up-a-new-team)

### 2. Multi-Tier Application Security
Set up web, app, and database tiers with controlled communication.

**Example:** Web tier can access app tier, app tier can access database tier.

**Guide:** [User Guide - Workflow 4](./user-guide.md#workflow-4-multi-tier-application-security)

### 3. Cloud Resource Access
Provide access to cloud networks (AWS, Azure, GCP) through jump peers.

**Example:** Route to AWS VPC with DNS resolution for internal services.

**Guide:** [User Guide - Workflow 3](./user-guide.md#workflow-3-providing-access-to-cloud-resources)

### 4. Incident Response
Quickly isolate compromised peers by moving them to a quarantine group.

**Example:** Move suspicious peer to isolated group with deny-all policy.

**Guide:** [User Guide - Workflow 2](./user-guide.md#workflow-2-isolating-a-compromised-peer)

### 5. Default User Access
Configure baseline access for all non-admin created peers.

**Example:** All users get access to office network and internet by default.

**Guide:** [User Guide - Default Groups](./user-guide.md#default-groups-configuration)

## API Overview

### Groups Endpoints
```
POST   /api/v1/networks/:networkId/groups
GET    /api/v1/networks/:networkId/groups
GET    /api/v1/networks/:networkId/groups/:groupId
PUT    /api/v1/networks/:networkId/groups/:groupId
DELETE /api/v1/networks/:networkId/groups/:groupId
POST   /api/v1/networks/:networkId/groups/:groupId/peers/:peerId
DELETE /api/v1/networks/:networkId/groups/:groupId/peers/:peerId
```

### Policies Endpoints
```
POST   /api/v1/networks/:networkId/policies
GET    /api/v1/networks/:networkId/policies
GET    /api/v1/networks/:networkId/policies/:policyId
PUT    /api/v1/networks/:networkId/policies/:policyId
DELETE /api/v1/networks/:networkId/policies/:policyId
POST   /api/v1/networks/:networkId/policies/:policyId/rules
DELETE /api/v1/networks/:networkId/policies/:policyId/rules/:ruleId
GET    /api/v1/networks/:networkId/policies/templates
POST   /api/v1/networks/:networkId/groups/:groupId/policies/:policyId
DELETE /api/v1/networks/:networkId/groups/:groupId/policies/:policyId
GET    /api/v1/networks/:networkId/groups/:groupId/policies
```

### Routes Endpoints
```
POST   /api/v1/networks/:networkId/routes
GET    /api/v1/networks/:networkId/routes
GET    /api/v1/networks/:networkId/routes/:routeId
PUT    /api/v1/networks/:networkId/routes/:routeId
DELETE /api/v1/networks/:networkId/routes/:routeId
POST   /api/v1/networks/:networkId/groups/:groupId/routes/:routeId
DELETE /api/v1/networks/:networkId/groups/:groupId/routes/:routeId
GET    /api/v1/networks/:networkId/groups/:groupId/routes
```

### DNS Endpoints
```
POST   /api/v1/networks/:networkId/routes/:routeId/dns
GET    /api/v1/networks/:networkId/routes/:routeId/dns
PUT    /api/v1/networks/:networkId/routes/:routeId/dns/:dnsId
DELETE /api/v1/networks/:networkId/routes/:routeId/dns/:dnsId
GET    /api/v1/networks/:networkId/dns
```

**Full Details:** [API Reference](./api-reference.md)

## Security Considerations

### Admin-Only Operations
All group, policy, route, and DNS operations require administrator privileges.

### Authorization
- Non-admin users receive HTTP 403 for admin operations
- Authorization checked at both API and service layers

### Audit Logging
All administrative operations are logged for security auditing.

### Default Deny
Traffic is denied by default unless explicitly allowed by policies.

### Principle of Least Privilege
Start with minimal access and add permissions as needed.

## Performance Considerations

### Scalability
- Groups: Thousands per network
- Policies: Hundreds per network
- Routes: Hundreds per network
- Peers per group: Thousands

### Configuration Updates
- Policy changes: Applied within seconds via WebSocket
- Route changes: WireGuard configs regenerated automatically
- DNS changes: Propagated to jump peers within 60 seconds

### Database Optimization
- Indexed foreign keys for fast lookups
- Connection pooling for concurrent requests
- Batch operations for bulk changes

## Troubleshooting

### Quick Diagnostics

**Check group membership:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID"
```

**Check attached policies:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies"
```

**Check attached routes:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes"
```

**Check iptables on jump peer:**
```bash
iptables -L -n -v
```

**Check WireGuard config:**
```bash
wg show
```

**Full Guide:** [Troubleshooting](./troubleshooting.md)

## Support

### Documentation
- [API Reference](./api-reference.md)
- [Migration Guide](./migration-guide.md)
- [User Guide](./user-guide.md)
- [Troubleshooting](./troubleshooting.md)

### Getting Help
1. Check documentation and troubleshooting guides
2. Review server logs: `docker logs wireguard-server`
3. Check agent logs: `journalctl -u wireguard-agent`
4. Open GitHub issue with details

## Version Information

This documentation covers the groups, policies, and routes architecture introduced in version 2.0.

**Previous Version:** Legacy ACL and peer flag system (deprecated)
**Current Version:** Groups, policies, and routes architecture
**Migration Required:** Yes - see [Migration Guide](./migration-guide.md)
