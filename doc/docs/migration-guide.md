# Migration Guide: Groups, Policies, and Routes

This guide helps you migrate from the legacy ACL and peer flag system to the new groups, policies, and routes architecture.

## Overview

The WireGuard network management system has undergone a major architectural redesign. This is a **breaking change** with no backward compatibility.

### What's New

- **Groups**: Organize peers into logical collections
- **Policies**: Define iptables rules for traffic filtering on jump peers
- **Routes**: Configure external network access through jump peers
- **DNS Mappings**: Resolve custom domains for route networks
- **Default Groups**: Automatically assign groups to non-admin created peers

### What's Removed

- **ACL System**: Completely removed (tables: `acls`, `acl_rules`)
- **Peer Flags**: `is_isolated` and `full_encapsulation` fields removed
- **Legacy Access Control**: All access control now through policies

---

## Breaking Changes

### 1. ACL System Removal

**What Changed:**
- The `acls` and `acl_rules` database tables have been dropped
- ACL-related API endpoints no longer exist
- ACL configuration in peer and network objects is removed

**Migration Path:**
ACLs must be manually recreated as policies. See [Converting ACLs to Policies](#converting-acls-to-policies) below.

---

### 2. Peer Flag Removal

**What Changed:**
- `is_isolated` field removed from peers
- `full_encapsulation` field removed from peers
- These fields are no longer accepted in API requests

**Migration Path:**
Use policy templates to achieve the same behavior:
- `is_isolated: true` → Use "isolated" policy template
- `full_encapsulation: true` → Use "fully-encapsulated" policy template

See [Converting Peer Flags to Policies](#converting-peer-flags-to-policies) below.

---

### 3. API Changes

**Removed Endpoints:**
```
DELETE /api/v1/networks/:networkId/acls/:aclId
POST   /api/v1/networks/:networkId/acls
GET    /api/v1/networks/:networkId/acls
```

**New Endpoints:**
```
# Groups
POST   /api/v1/networks/:networkId/groups
GET    /api/v1/networks/:networkId/groups
PUT    /api/v1/networks/:networkId/groups/:groupId
DELETE /api/v1/networks/:networkId/groups/:groupId

# Policies
POST   /api/v1/networks/:networkId/policies
GET    /api/v1/networks/:networkId/policies
PUT    /api/v1/networks/:networkId/policies/:policyId
DELETE /api/v1/networks/:networkId/policies/:policyId

# Routes
POST   /api/v1/networks/:networkId/routes
GET    /api/v1/networks/:networkId/routes
PUT    /api/v1/networks/:networkId/routes/:routeId
DELETE /api/v1/networks/:networkId/routes/:routeId

# DNS Mappings
POST   /api/v1/networks/:networkId/routes/:routeId/dns
GET    /api/v1/networks/:networkId/routes/:routeId/dns
```

See the [API Reference](./api-reference.md) for complete documentation.

---

### 4. Peer Model Changes

**Before:**
```json
{
  "id": "peer-1",
  "name": "laptop-1",
  "is_isolated": true,
  "full_encapsulation": false,
  ...
}
```

**After:**
```json
{
  "id": "peer-1",
  "name": "laptop-1",
  "group_ids": ["group-1"],
  ...
}
```

**Impact:**
- API requests containing `is_isolated` or `full_encapsulation` will return HTTP 400
- API responses no longer include these fields
- Peer behavior is now controlled by group membership and attached policies

---

### 5. Network Model Changes

**New Fields:**
```json
{
  "domain_suffix": "internal",
  "default_group_ids": ["group-1", "group-2"]
}
```

**Impact:**
- Networks can now specify custom DNS domain suffixes
- Default groups are automatically assigned to peers created by non-admins

---

## Migration Steps

### Step 1: Backup Your Data

Before upgrading, backup your database:

```bash
# PostgreSQL backup
pg_dump -h localhost -U postgres -d wireguard > backup_$(date +%Y%m%d).sql

# Or using Docker
docker exec postgres pg_dump -U postgres wireguard > backup_$(date +%Y%m%d).sql
```

### Step 2: Document Current Configuration

Export your current ACL and peer configurations:

```bash
# Export ACLs
curl -H "Authorization: Bearer $TOKEN" \
  https://your-server/api/v1/networks/$NETWORK_ID/acls > acls_backup.json

# Export peers with flags
curl -H "Authorization: Bearer $TOKEN" \
  https://your-server/api/v1/networks/$NETWORK_ID/peers > peers_backup.json
```

### Step 3: Upgrade Server

Deploy the new server version:

```bash
# Using Docker
docker pull your-registry/wireguard-server:latest
docker-compose up -d

# Using Kubernetes
kubectl apply -f deployment.yaml
```

The database migration will run automatically on startup.

### Step 4: Recreate Access Control

Follow the conversion guides below to recreate your access control configuration using the new system.

### Step 5: Update Agents

Upgrade all jump peer agents to support the newDNS features:

```bash
# On each jump peer
systemctl stop wireguard-agent
wget https://your-server/downloads/agent-latest
chmod +x agent-latest
mv agent-latest /usr/local/bin/wireguard-agent
systemctl start wireguard-agent
```

### Step 6: Verify Configuration

Test connectivity and verify that policies are working correctly:

```bash
# Check peer connectivity
ping <peer-ip>

# Verify DNS resolution
nslookup server.route.internal

# Check iptables rules on jump peers
iptables -L -n -v
```

---

## Converting ACLs to Policies

### Understanding the Mapping

**Legacy ACL:**
- Applied at the network level
- Controlled peer-to-peer communication
- Simple allow/deny rules

**New Policies:**
- Applied to groups
- Generate iptables rules on jump peers
- Support input/output direction
- Support CIDR, peer, and group targets

### Example Conversion

**Legacy ACL:**
```json
{
  "name": "allow-internal",
  "rules": [
    {
      "action": "allow",
      "source": "10.0.0.0/24",
      "destination": "10.0.0.0/24"
    }
  ]
}
```

**New Policy:**
```json
{
  "name": "allow-internal",
  "description": "Allow traffic within internal network",
  "rules": [
    {
      "direction": "input",
      "action": "allow",
      "target": "10.0.0.0/24",
      "target_type": "cidr",
      "description": "Allow inbound from internal network"
    },
    {
      "direction": "output",
      "action": "allow",
      "target": "10.0.0.0/24",
      "target_type": "cidr",
      "description": "Allow outbound to internal network"
    }
  ]
}
```

### Conversion Script

Use this script to help convert ACLs to policies:

```bash
#!/bin/bash

NETWORK_ID="your-network-id"
TOKEN="your-admin-token"
API_URL="https://your-server/api/v1"

# Create policy from ACL
create_policy() {
  local acl_name=$1
  local rules=$2
  
  curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"$acl_name\",
      \"description\": \"Converted from legacy ACL\",
      \"rules\": $rules
    }"
}

# Example: Convert allow-all ACL
create_policy "allow-all" '[
  {
    "direction": "input",
    "action": "allow",
    "target": "0.0.0.0/0",
    "target_type": "cidr",
    "description": "Allow all inbound"
  },
  {
    "direction": "output",
    "action": "allow",
    "target": "0.0.0.0/0",
    "target_type": "cidr",
    "description": "Allow all outbound"
  }
]'
```

---

## Converting Peer Flags to Policies

### Using Policy Templates

The system provides three built-in policy templates that replace the legacy peer flags:

#### 1. Isolated Peers

**Legacy:**
```json
{
  "is_isolated": true
}
```

**New Approach:**
1. Get the "isolated" template:
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/policies/templates"
```

2. Create a policy from the template:
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "isolated-policy",
    "description": "Deny all traffic",
    "rules": [
      {
        "direction": "input",
        "action": "deny",
        "target": "0.0.0.0/0",
        "target_type": "cidr",
        "description": "Deny all inbound"
      },
      {
        "direction": "output",
        "action": "deny",
        "target": "0.0.0.0/0",
        "target_type": "cidr",
        "description": "Deny all outbound"
      }
    ]
  }'
```

3. Create a group and attach the policy:
```bash
# Create group
GROUP_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "isolated-peers"}' | jq -r '.id')

# Attach policy to group
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"

# Add peer to group
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

#### 2. Fully Encapsulated Peers

**Legacy:**
```json
{
  "full_encapsulation": true
}
```

**New Approach:**
Use the "fully-encapsulated" template:
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "fully-encapsulated",
    "description": "Allow outbound, deny inbound",
    "rules": [
      {
        "direction": "output",
        "action": "allow",
        "target": "0.0.0.0/0",
        "target_type": "cidr",
        "description": "Allow all outbound"
      },
      {
        "direction": "input",
        "action": "deny",
        "target": "0.0.0.0/0",
        "target_type": "cidr",
        "description": "Deny all inbound"
      }
    ]
  }'
```

#### 3. Default Network Access

**Legacy:**
```json
{
  "is_isolated": false,
  "full_encapsulation": false
}
```

**New Approach:**
Use the "default-network" template:
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "default-network",
    "description": "Allow traffic within network",
    "rules": [
      {
        "direction": "input",
        "action": "allow",
        "target": "10.0.0.0/24",
        "target_type": "cidr",
        "description": "Allow inbound from network"
      },
      {
        "direction": "output",
        "action": "allow",
        "target": "10.0.0.0/24",
        "target_type": "cidr",
        "description": "Allow outbound to network"
      }
    ]
  }'
```

### Bulk Conversion Script

Convert all peers with flags to groups and policies:

```bash
#!/bin/bash

NETWORK_ID="your-network-id"
TOKEN="your-admin-token"
API_URL="https://your-server/api/v1"

# Create policies from templates
ISOLATED_POLICY_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "isolated", "rules": [...]}' | jq -r '.id')

ENCAPSULATED_POLICY_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "fully-encapsulated", "rules": [...]}' | jq -r '.id')

# Create groups
ISOLATED_GROUP_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "isolated-peers"}' | jq -r '.id')

ENCAPSULATED_GROUP_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "encapsulated-peers"}' | jq -r '.id')

# Attach policies to groups
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$ISOLATED_GROUP_ID/policies/$ISOLATED_POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"

curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$ENCAPSULATED_GROUP_ID/policies/$ENCAPSULATED_POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"

# Process each peer from backup
jq -c '.[]' peers_backup.json | while read peer; do
  PEER_ID=$(echo $peer | jq -r '.id')
  IS_ISOLATED=$(echo $peer | jq -r '.is_isolated')
  FULL_ENCAP=$(echo $peer | jq -r '.full_encapsulation')
  
  if [ "$IS_ISOLATED" = "true" ]; then
    echo "Adding $PEER_ID to isolated group"
    curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$ISOLATED_GROUP_ID/peers/$PEER_ID" \
      -H "Authorization: Bearer $TOKEN"
  elif [ "$FULL_ENCAP" = "true" ]; then
    echo "Adding $PEER_ID to encapsulated group"
    curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$ENCAPSULATED_GROUP_ID/peers/$PEER_ID" \
      -H "Authorization: Bearer $TOKEN"
  fi
done
```

---

## New Concepts Explained

### Groups

Groups are collections of peers that share common characteristics. They serve as the attachment point for policies and routes.

**Key Features:**
- Admin-only management
- Peers can belong to multiple groups
- Policies and routes are attached to groups, not individual peers
- Deleting a group doesn't delete the peers

**Use Cases:**
- Organize peers by team (engineering, sales, support)
- Organize by function (databases, web servers, clients)
- Organize by security level (trusted, untrusted, DMZ)

**Example:**
```bash
# Create engineering team group
curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "engineering",
    "description": "Engineering team members"
  }'

# Add peers to group
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Policies

Policies define iptables rules that are applied on jump peers to control traffic filtering.

**Key Features:**
- Admin-only management
- Attached to groups (not individual peers)
- Support input and output directions
- Support allow and deny actions
- Support CIDR, peer, and group targets
- Applied in order of attachment

**How It Works:**
1. Create a policy with rules
2. Attach the policy to a group
3. System generates iptables rules on jump peers
4. Rules are applied to all traffic for group members

**Example:**
```bash
# Create policy allowing web traffic
curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "allow-web",
    "description": "Allow HTTP/HTTPS traffic",
    "rules": [
      {
        "direction": "output",
        "action": "allow",
        "target": "0.0.0.0/0",
        "target_type": "cidr",
        "description": "Allow all outbound"
      },
      {
        "direction": "input",
        "action": "allow",
        "target": "10.0.0.0/24",
        "target_type": "cidr",
        "description": "Allow inbound from network"
      }
    ]
  }'

# Attach to group
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Routes

Routes define external network destinations that are accessible through jump peers. They are added to the WireGuard AllowedIPs configuration.

**Key Features:**
- Admin-only management
- Attached to groups (not individual peers)
- Specify destination CIDR and jump peer
- Support custom DNS domain suffixes
- Automatically update WireGuard configurations

**How It Works:**
1. Create a route with destination CIDR and jump peer
2. Attach the route to a group
3. System adds the CIDR to AllowedIPs for all group members
4. Traffic to that CIDR is routed through the jump peer

**Example:**
```bash
# Create route to AWS VPC
curl -X POST "$API_URL/networks/$NETWORK_ID/routes" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "aws-vpc",
    "description": "AWS VPC network",
    "destination_cidr": "172.31.0.0/16",
    "jump_peer_id": "'$JUMP_PEER_ID'",
    "domain_suffix": "aws.internal"
  }'

# Attach to group
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### DNS Mappings

DNS mappings provide name resolution for IP addresses within route networks.

**Key Features:**
- Admin-only management
- Associated with routes
- IP must be within route's CIDR
- FQDN format: `name.route_name.domain_suffix`
- Propagated to jump peer DNS servers

**Example:**
```bash
# Create DNS mapping for database server
curl -X POST "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "database",
    "ip_address": "172.31.10.50"
  }'

# Resolves as: database.aws-vpc.aws.internal
```

### Default Groups

Default groups are automatically assigned to peers created by non-administrator users.

**Key Features:**
- Admin-only configuration
- Applied only to non-admin created peers
- Admin-created peers are not auto-assigned
- Useful for applying baseline policies to all users

**Example:**
```bash
# Configure default groups for network
curl -X PUT "$API_URL/networks/$NETWORK_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "default_group_ids": ["'$GROUP_ID_1'", "'$GROUP_ID_2'"]
  }'

# When a non-admin creates a peer, it's automatically added to these groups
```

---

## Common Migration Scenarios

### Scenario 1: Simple Network with Isolated Peers

**Before:**
- Network with 10 peers
- 3 peers with `is_isolated: true`
- 7 peers with default settings

**After:**
1. Create "isolated" policy from template
2. Create "isolated-peers" group
3. Attach policy to group
4. Add the 3 isolated peers to the group

### Scenario 2: Network with ACL Rules

**Before:**
- Network with ACL allowing internal traffic
- ACL denying external traffic

**After:**
1. Create "internal-only" policy with rules:
   - Allow input from network CIDR
   - Allow output to network CIDR
   - Deny all other traffic
2. Create "internal-users" group
3. Attach policy to group
4. Add all peers to the group

### Scenario 3: Jump Peer with External Access

**Before:**
- Jump peer providing internet access
- Regular peers with `full_encapsulation: true`

**After:**
1. Create route for internet access (0.0.0.0/0)
2. Create "fully-encapsulated" policy
3. Create "internet-users" group
4. Attach policy and route to group
5. Add regular peers to group

### Scenario 4: Multi-Tier Application

**Before:**
- Web tier: 3 peers
- App tier: 5 peers
- DB tier: 2 peers (isolated)
- ACLs controlling tier-to-tier communication

**After:**
1. Create groups: "web-tier", "app-tier", "db-tier"
2. Create policies:
   - "web-policy": Allow from internet, allow to app tier
   - "app-policy": Allow from web tier, allow to db tier
   - "db-policy": Allow from app tier only
3. Attach policies to respective groups
4. Add peers to their tier groups

---

## Troubleshooting

### Issue: Peers Can't Communicate After Migration

**Cause:** No policies attached to groups, default deny behavior.

**Solution:**
1. Check if peers are in groups: `GET /api/v1/networks/:networkId/peers/:peerId`
2. Check if groups have policies: `GET /api/v1/networks/:networkId/groups/:groupId/policies`
3. Create and attach appropriate policies

### Issue: DNS Not Resolving Route Domains

**Cause:** Jump peer agent not updated or DNS mappings not created.

**Solution:**
1. Verify agent version: `wireguard-agent --version`
2. Check DNS mappings: `GET /api/v1/networks/:networkId/routes/:routeId/dns`
3. Verify DNS server on jump peer: `systemctl status wireguard-agent`

### Issue: Routes Not Working

**Cause:** Route not attached to group or jump peer not configured.

**Solution:**
1. Verify route attachment: `GET /api/v1/networks/:networkId/groups/:groupId/routes`
2. Check jump peer configuration: `wg show`
3. Verify AllowedIPs includes route CIDR

### Issue: API Returns 403 Forbidden

**Cause:** Non-administrator user attempting admin-only operation.

**Solution:**
1. Verify user role: `GET /api/v1/users/me`
2. Use administrator account for group/policy/route operations
3. Contact administrator to perform the operation

---

## Rollback Procedure

If you need to rollback to the previous version:

### Step 1: Stop New Server

```bash
docker-compose down
# or
kubectl delete deployment wireguard-server
```

### Step 2: Restore Database

```bash
# Restore from backup
psql -h localhost -U postgres -d wireguard < backup_YYYYMMDD.sql

# Or using Docker
docker exec -i postgres psql -U postgres wireguard < backup_YYYYMMDD.sql
```

### Step 3: Deploy Previous Version

```bash
# Using Docker
docker pull your-registry/wireguard-server:previous-version
docker-compose up -d

# Using Kubernetes
kubectl apply -f deployment-previous.yaml
```

### Step 4: Verify Functionality

Test that the previous version is working correctly with the restored database.

---

## Support

If you encounter issues during migration:

1. Check the [Troubleshooting Guide](./troubleshooting.md)
2. Review server logs: `docker logs wireguard-server`
3. Check agent logs on jump peers: `journalctl -u wireguard-agent`
4. Open an issue on GitHub with:
   - Server version
   - Database migration logs
   - Error messages
   - Steps to reproduce

---

## Additional Resources

- [API Reference](./api-reference.md) - Complete API documentation
- [User Guide](./user-guide.md) - Step-by-step guides for common tasks
- [Architecture](./architecture.md) - System architecture overview
- [Troubleshooting](./troubleshooting.md) - Common issues and solutions

