# User Guide

This guide provides step-by-step instructions for common tasks in the WireGuard network management system.

## Table of Contents

1. [Groups Management](#groups-management)
2. [Policies Management](#policies-management)
3. [Routes Management](#routes-management)
4. [DNS Mappings](#dns-mappings)
5. [Default Groups Configuration](#default-groups-configuration)
6. [Common Workflows](#common-workflows)

---

## Prerequisites

- Administrator account (all operations in this guide require admin privileges)
- API access token
- Network ID for your network

### Getting Your API Token

```bash
# Login and get token
curl -X POST https://your-server/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "your-password"}'

# Save token for later use
export TOKEN="your-token-here"
export API_URL="https://your-server/api/v1"
export NETWORK_ID="your-network-id"
```

---

## Groups Management

Groups organize peers into logical collections for applying policies and routes.

### Creating a Group

**Step 1:** Define your group details

**Step 2:** Create the group via API

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "engineering-team",
    "description": "Engineering team members"
  }'
```

**Step 3:** Save the group ID from the response

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "network_id": "660e8400-e29b-41d4-a716-446655440000",
  "name": "engineering-team",
  "description": "Engineering team members",
  "peer_ids": [],
  "policy_ids": [],
  "route_ids": [],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

### Adding Peers to a Group

```bash
# Add a single peer
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"

# Add multiple peers (loop)
for PEER_ID in peer-1 peer-2 peer-3; do
  curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
    -H "Authorization: Bearer $TOKEN"
done
```

### Listing Groups

```bash
# List all groups in a network
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups"
```

### Viewing Group Details

```bash
# Get specific group with all members
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID"
```

### Updating a Group

```bash
# Update name and description
curl -X PUT "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "senior-engineers",
    "description": "Senior engineering team members"
  }'
```

### Removing Peers from a Group

```bash
# Remove a peer from a group
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Deleting a Group

```bash
# Delete a group (peers are not deleted)
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Policies Management

Policies define iptables rules applied on jump peers to control traffic filtering.

### Understanding Policy Rules

Each policy rule has:
- **Direction**: "input" (incoming traffic) or "output" (outgoing traffic)
- **Action**: "allow" or "deny"
- **Target**: IP/CIDR, peer ID, or group ID
- **Target Type**: "cidr", "peer", or "group"

### Creating a Policy from Scratch

**Step 1:** Design your policy rules

**Step 2:** Create the policy

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "allow-web-traffic",
    "description": "Allow HTTP and HTTPS traffic",
    "rules": [
      {
        "direction": "output",
        "action": "allow",
        "target": "0.0.0.0/0",
        "target_type": "cidr",
        "description": "Allow all outbound traffic"
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
```

### Using Policy Templates

The system provides three built-in templates:

**1. Fully Encapsulated** - Allow outbound, deny inbound

```bash
# Get templates
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/policies/templates"

# Create policy from fully-encapsulated template
curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-encapsulated-policy",
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

**2. Isolated** - Deny all traffic

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

**3. Default Network** - Allow traffic within network CIDR

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "network-only",
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

### Attaching a Policy to a Group

```bash
# Attach policy to group
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

This automatically applies the policy's iptables rules on all jump peers for all group members.

### Managing Policy Rules

**Add a rule to existing policy:**

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/policies/$POLICY_ID/rules" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "direction": "input",
    "action": low",
    "target": "192.168.1.0/24",
    "target_type": "cidr",
    "description": "Allow from office network"
  }'
```

**Remove a rule:**

```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/policies/$POLICY_ID/rules/$RULE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Viewing Policies

```bash
# List all policies
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/policies"

# Get specific policy with rules
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/policies/$POLICY_ID"

# Get policies attached to a group
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies"
```

### Detaching a Policy from a Group

```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Deleting a Policy

```bash
# Delete policy (removes from all groups)
curl -X DELETE "$API_URL/networks/$NETWORK_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Routes Management

Routes define external network destinations accessible through jump peers.

### Creating a Route

**Step 1:** Identify your jump peer

```bash
# List peers and find jump peers (is_jump: true)
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/peers"
```

**Step 2:** Create the route

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/routes" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "aws-vpc",
    "description": "AWS VPC network",
    "destination_cidr": "172.31.0.0/16",
    "jump_peer_id": "880e8400-e29b-41d4-a716-446655440000",
    "domain_suffix": "aws.internal"
  }'
```

**Parameters:**
- `destination_cidr`: The external network CIDR (required)
- `jump_peer_id`: The jump peer that routes traffic (required)
- `domain_suffix`: Custom DNS suffix (optional, default: "internal")

### Attaching a Route to a Group

```bash
# Attach route to group
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

This automatically:
1. Adds the route CIDR to AllowedIPs for all group members
2. Configures the jump peer as the gateway
3. Regenerates WireGuard configurations

### Listing Routes

```bash
# List all routes in network
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/routes"

# Get routes attached to a group
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes"
```

### Updating a Route

```bash
curl -X PUT "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "aws-vpc-updated",
    "description": "Updated AWS VPC",
    "destination_cidr": "172.31.0.0/16",
    "jump_peer_id": "880e8400-e29b-41d4-a716-446655440000",
    "domain_suffix": "aws.internal"
  }'
```

Updates trigger automatic WireGuard config regeneration for affected peers.

### Detaching a Route from a Group

```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Deleting a Route

```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## DNS Mappings

DNS mappings provide name resolution for IP addresses within route networks.

### Creating a DNS Mapping

**Step 1:** Ensure you have a route created

**Step 2:** Create DNS mapping (IP must be within route CIDR)

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "database-server",
    "ip_address": "172.31.10.50"
  }'
```

**FQDN Format:** `database-server.aws-vpc.aws.internal`
- `database-server`: The name you specified
- `aws-vpc`: The route name
- `aws.internal`: The route's domain suffix

### Listing DNS Mappings

```bash
# List DNS mappings for a route
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns"

# Get all DNS records for network (peers + routes)
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/dns"
```

### Updating a DNS Mapping

```bash
curl -X PUT "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns/$DNS_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "db-primary",
    "ip_address": "172.31.10.51"
  }'
```

Changes propagate to jump peer DNS servers within 60 seconds.

### Deleting a DNS Mapping

```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns/$DNS_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Testing DNS Resolution

From a peer in the network:

```bash
# Test DNS resolution
nslookup database-server.aws-vpc.aws.internal

# Or using dig
dig database-server.aws-vpc.aws.internal
```

---

## Default Groups Configuration

Default groups are automatically assigned to peers created by non-administrator users.

### Configuring Default Groups

**Step 1:** Create groups for default assignment

```bash
# Create a "standard-users" group
curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "standard-users",
    "description": "Default group for all users"
  }'
```

**Step 2:** Attach policies and routes to the group

```bash
# Attach a policy
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"

# Attach a route
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

**Step 3:** Configure the group as default for the network

```bash
curl -X PUT "$API_URL/networks/$NETWORK_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "default_group_ids": ["'$GROUP_ID'"]
  }'
```

### How Default Groups Work

- **Non-admin creates peer**: Automatically added to all default groups
- **Admin creates peer**: NOT automatically added to default groups
- **Existing peers**: Not affected by default group configuration changes

### Viewing Default Groups

```bash
# Get network details including default groups
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID"
```

### Removing Default Group Configuration

```bash
# Clear default groups
curl -X PUT "$API_URL/networks/$NETWORK_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "default_group_ids": []
  }'
```

---

## Common Workflows

### Workflow 1: Setting Up a New Team

**Goal:** Create a group for the engineering team with internet access and internal network access.

**Steps:**

1. Create the group:
```bash
GROUP_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "engineering", "description": "Engineering team"}' \
  | jq -r '.id')
```

2. Create a policy for internet access:
```bash
POLICY_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "internet-access",
    "rules": [
      {"direction": "output", "action": "allow", "target": "0.0.0.0/0", "target_type": "cidr"},
      {"direction": "input", "action": "allow", "target": "10.0.0.0/24", "target_type": "cidr"}
    ]
  }' | jq -r '.id')
```

3. Create a route for office network:
```bash
ROUTE_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/routes" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "office-network",
    "destination_cidr": "192.168.1.0/24",
    "jump_peer_id": "'$JUMP_PEER_ID'"
  }' | jq -r '.id')
```

4. Attach policy and route to group:
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"

curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

5. Add team members to group:
```bash
for PEER_ID in peer-1 peer-2 peer-3; do
  curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
    -H "Authorization: Bearer $TOKEN"
done
```

### Workflow 2: Isolating a Compromised Peer

**Goal:** Immediately isolate a peer that may be compromised.

**Steps:**

1. Create isolated policy (if not exists):
```bash
ISOLATED_POLICY_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "isolated",
    "rules": [
      {"direction": "input", "action": "deny", "target": "0.0.0.0/0", "target_type": "cidr"},
      {"direction": "output", "action": "deny", "target": "0.0.0.0/0", "target_type": "cidr"}
    ]
  }' | jq -r '.id')
```

2. Create quarantine group:
```bash
QUARANTINE_GROUP_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "quarantine", "description": "Isolated peers"}' \
  | jq -r '.id')
```

3. Attach isolated policy:
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$QUARANTINE_GROUP_ID/policies/$ISOLATED_POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

4. Move peer to quarantine group:
```bash
# Remove from current groups
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$OLD_GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"

# Add to quarantine
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$QUARANTINE_GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Workflow 3: Providing Access to Cloud Resources

**Goal:** Give a group access to AWS VPC with DNS resolution.

**Steps:**

1. Create route to AWS VPC:
```bash
ROUTE_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/routes" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "aws-vpc",
    "destination_cidr": "172.31.0.0/16",
    "jump_peer_id": "'$JUMP_PEER_ID'",
    "domain_suffix": "aws.internal"
  }' | jq -r '.id')
```

2. Create DNS mappings for important servers:
```bash
# Database server
curl -X POST "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "database", "ip_address": "172.31.10.50"}'

# Web server
curl -X POST "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "web", "ip_address": "172.31.20.100"}'
```

3. Attach route to group:
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

4. Test DNS resolution from a peer:
```bash
nslookup database.aws-vpc.aws.internal
nslookup web.aws-vpc.aws.internal
```

### Workflow 4: Multi-Tier Application Security

**Goal:** Set up web, app, and database tiers with appropriate access controls.

**Steps:**

1. Create groups for each tier:
```bash
WEB_GROUP=$(curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name": "web-tier"}' | jq -r '.id')

APP_GROUP=$(curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name": "app-tier"}' | jq -r '.id')

DB_GROUP=$(curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name": "db-tier"}' | jq -r '.id')
```

2. Create policies for each tier:
```bash
# Web tier: Allow from internet, allow to app tier
WEB_POLICY=$(curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "web-policy",
    "rules": [
      {"direction": "input", "action": "allow", "target": "0.0.0.0/0", "target_type": "cidr"},
      {"direction": "output", "action": "allow", "target": "'$APP_GROUP'", "target_type": "group"}
    ]
  }' | jq -r '.id')

# App tier: Allow from web, allow to db
APP_POLICY=$(curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "app-policy",
    "rules": [
      {"direction": "input", "action": "allow", "target": "'$WEB_GROUP'", "target_type": "group"},
      {"direction": "output", "action": "allow", "target": "'$DB_GROUP'", "target_type": "group"}
    ]
  }' | jq -r '.id')

# DB tier: Allow from app only
DB_POLICY=$(curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "db-policy",
    "rules": [
      {"direction": "input", "action": "allow", "target": "'$APP_GROUP'", "target_type": "group"}
    ]
  }' | jq -r '.id')
```

3. Attach policies to groups:
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$WEB_GROUP/policies/$WEB_POLICY" \
  -H "Authorization: Bearer $TOKEN"

curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$APP_GROUP/policies/$APP_POLICY" \
  -H "Authorization: Bearer $TOKEN"

curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$DB_GROUP/policies/$DB_POLICY" \
  -H "Authorization: Bearer $TOKEN"
```

4. Add peers to appropriate tiers:
```bash
# Add web servers
for PEER in web-1 web-2; do
  curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$WEB_GROUP/peers/$PEER" \
    -H "Authorization: Bearer $TOKEN"
done

# Add app servers
for PEER in app-1 app-2 app-3; do
  curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$APP_GROUP/peers/$PEER" \
    -H "Authorization: Bearer $TOKEN"
done

# Add database servers
for PEER in db-1 db-2; do
  curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$DB_GROUP/peers/$PEER" \
    -H "Authorization: Bearer $TOKEN"
done
```

---

## Best Practices

### Group Organization

1. **Use descriptive names**: "engineering-team" not "group1"
2. **Document purpose**: Use the description field
3. **Keep groups focused**: One purpose per group
4. **Plan hierarchy**: Consider nested access patterns

### Policy Design

1. **Start restrictive**: Begin with deny-all, add allows as needed
2. **Use templates**: Leverage built-in templates for common patterns
3. **Document rules**: Add descriptions to each rule
4. **Test incrementally**: Add one rule at a time and test
5. **Order matters**: Rules are applied in order of attachment

### Route Management

1. **Verify CIDRs**: Double-check CIDR notation before creating
2. **Use meaningful names**: "aws-vpc-prod" not "route1"
3. **Document jump peers**: Note which jump peer serves which route
4. **Monitor capacity**: Ensure jump peers can handle traffic

### DNS Mappings

1. **Use consistent naming**: Follow a naming convention
2. **Document IPs**: Keep external documentation of mappings
3. **Verify CIDR**: Ensure IPs are within route CIDR
4. **Test resolution**: Always test DNS after creating mappings

### Default Groups

1. **Keep it simple**: Start with one default group
2. **Baseline access**: Provide minimum necessary access
3. **Review regularly**: Audit default group policies quarterly
4. **Communicate**: Inform users about automatic group assignment

---

## Troubleshooting

### Groups

**Problem:** Can't add peer to group
- **Check:** Peer exists in the network
- **Check:** You have admin privileges
- **Check:** Group exists in the same network

**Problem:** Deleting group fails
- **Check:** No foreign key constraints
- **Check:** Group exists
- **Solution:** Detach policies and routes first

### Policies

**Problem:** Policy not taking effect
- **Check:** Policy is attached to group
- **Check:** Peer is member of group
- **Check:** Jump peer agent is running
- **Solution:** Check iptables on jump peer: `iptables -L -n -v`

**Problem:** Traffic blocked unexpectedly
- **Check:** Policy rule order
- **Check:** Default deny behavior
- **Solution:** Add explicit allow rule

### Routes

**Problem:** Can't reach route destination
- **Check:** Route attached to peer's group
- **Check:** Jump peer is online
- **Check:** WireGuard config includes route CIDR
- **Solution:** Check AllowedIPs: `wg show`

**Problem:** Route creation fails
- **Check:** CIDR format is valid
- **Check:** Jump peer exists and is_jump=true
- **Check:** No duplicate route names

### DNS

**Problem:** DNS not resolving
- **Check:** DNS mapping exists
- **Check:** IP is within route CIDR
- **Check:** Jump peer DNS server is running
- **Solution:** Check DNS server logs on jump peer

**Problem:** Wrong IP returned
- **Check:** DNS mapping is correct
- **Check:** DNS cache (clear with `systemd-resolve --flush-caches`)
- **Solution:** Update DNS mapping

---

## Additional Resources

- [API Reference](./api-reference.md) - Complete API documentation
- [Migration Guide](./migration-guide.md) - Migrating from legacy system
- [Architecture](./architecture.md) - System architecture overview
- [Troubleshooting](./troubleshooting.md) - Detailed troubleshooting guide

---

## Getting Help

If you need assistance:

1. Check this guide and the troubleshooting section
2. Review server logs: `docker logs wireguard-server`
3. Check agent logs: `journalctl -u wireguard-agent`
4. Open an issue on GitHub with:
   - What you're trying to do
   - What you expected to happen
   - What actually happened
   - Relevant logs and error messages

