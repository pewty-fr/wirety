# Groups Management Guide

This guide provides detailed information about managing groups in the WireGuard network management system.

## What are Groups?

Groups are logical collections of peers that share common characteristics, policies, or access requirements. They serve as the primary organizational unit for applying network policies and routes.

## Key Concepts

### Group Membership
- Peers can belong to multiple groups simultaneously
- Group membership is managed by administrators only
- Adding/removing peers from groups is non-destructive (peers are not deleted)

### Group Attachments
Groups can have:
- **Policies**: Traffic filtering rules applied on jump peers
- **Routes**: External network destinations added to peer configurations

### Automatic Application
When a peer joins a group:
- All attached policies are automatically applied
- All attached routes are added to the peer's WireGuard configuration
- Changes take effect within seconds via WebSocket notifications

## Creating Groups

### Basic Group Creation

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "engineering-team",
    "description": "Engineering team members with full network access"
  }'
```

### Naming Best Practices

**Good Names:**
- `engineering-team`
- `sales-department`
- `database-servers`
- `trusted-devices`
- `guest-network`

**Avoid:**
- `group1`, `group2` (not descriptive)
- `temp` (unclear purpose)
- `test` (ambiguous)

### Description Guidelines

Write clear descriptions that explain:
- Purpose of the group
- Who should be in it
- What access it provides

**Example:**
```json
{
  "name": "remote-developers",
  "description": "Remote development team with access to staging environment and office VPN"
}
```

## Managing Group Membership

### Adding Peers

**Single Peer:**
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

**Multiple Peers:**
```bash
#!/bin/bash
PEERS=("peer-1" "peer-2" "peer-3" "peer-4")

for PEER_ID in "${PEERS[@]}"; do
  echo "Adding $PEER_ID to group..."
  curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
    -H "Authorization: Bearer $TOKEN"
done
```

### Removing Peers

```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

**Important:** Removing a peer from a group:
- Removes attached policies from the peer
- Removes attached routes from the peer's configuration
- Does NOT delete the peer itself

### Viewing Group Members

```bash
# Get group details with member list
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID" \
  | jq '.peer_ids'
```

## Attaching Policies to Groups

### Attach a Policy

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### View Attached Policies

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies"
```

### Detach a Policy

```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Policy Order Matters

When multiple policies are attached to a group:
1. Rules are applied in the order policies were attached
2. First matching rule wins
3. Default deny applies if no rules match

**Example:**
```bash
# Attach policies in order
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$ALLOW_POLICY" \
  -H "Authorization: Bearer $TOKEN"

curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$DENY_POLICY" \
  -H "Authorization: Bearer $TOKEN"
```

## Attaching Routes to Groups

### Attach a Route

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### View Attached Routes

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes"
```

### Detach a Route

```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

## Common Group Patterns

### Pattern 1: Department-Based Groups

Organize by organizational structure:

```bash
# Create department groups
curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "engineering", "description": "Engineering department"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "sales", "description": "Sales department"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "support", "description": "Customer support"}'
```

### Pattern 2: Role-Based Groups

Organize by job function:

```bash
# Create role groups
curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "developers", "description": "Software developers"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "admins", "description": "System administrators"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "viewers", "description": "Read-only access"}'
```

### Pattern 3: Environment-Based Groups

Organize by environment access:

```bash
# Create environment groups
curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "production-access", "description": "Production environment access"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "staging-access", "description": "Staging environment access"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "development-access", "description": "Development environment access"}'
```

### Pattern 4: Security Level Groups

Organize by trust level:

```bash
# Create security level groups
curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "trusted", "description": "Fully trusted devices"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "standard", "description": "Standard security devices"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "restricted", "description": "Restricted access devices"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "quarantine", "description": "Isolated/compromised devices"}'
```

## Advanced Group Management

### Bulk Operations Script

```bash
#!/bin/bash
# bulk-group-operations.sh

API_URL="https://your-server/api/v1"
TOKEN="your-token"
NETWORK_ID="your-network-id"

# Function to create group
create_group() {
  local name=$1
  local description=$2
  
  curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"name\": \"$name\", \"description\": \"$description\"}" \
    | jq -r '.id'
}

# Function to add peers to group
add_peers_to_group() {
  local group_id=$1
  shift
  local peers=("$@")
  
  for peer_id in "${peers[@]}"; do
    curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$group_id/peers/$peer_id" \
      -H "Authorization: Bearer $TOKEN"
  done
}

# Example usage
ENGINEERING_GROUP=$(create_group "engineering" "Engineering team")
add_peers_to_group "$ENGINEERING_GROUP" "peer-1" "peer-2" "peer-3"
```

### Group Audit Script

```bash
#!/bin/bash
# audit-groups.sh

# List all groups with member counts
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups" \
  | jq -r '.[] | "\(.name): \(.peer_ids | length) members"'

# Detailed group report
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups" \
  | jq -r '.[] | "Group: \(.name)\nMembers: \(.peer_ids | length)\nPolicies: \(.policy_ids | length)\nRoutes: \(.route_ids | length)\n"'
```

### Group Cleanup Script

```bash
#!/bin/bash
# cleanup-empty-groups.sh

# Find and delete groups with no members
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups" \
  | jq -r '.[] | select(.peer_ids | length == 0) | .id' \
  | while read group_id; do
      echo "Deleting empty group: $group_id"
      curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$group_id" \
        -H "Authorization: Bearer $TOKEN"
    done
```

## Troubleshooting

### Problem: Can't Create Group

**Symptoms:**
- HTTP 400 Bad Request
- Error: "group name already exists"

**Solutions:**
1. Check for duplicate names:
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups" \
  | jq -r '.[].name'
```

2. Use a different name or update existing group

### Problem: Can't Add Peer to Group

**Symptoms:**
- HTTP 404 Not Found
- Peer not appearing in group

**Solutions:**
1. Verify peer exists:
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/peers/$PEER_ID"
```

2. Verify group exists:
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID"
```

3. Check network ID matches

### Problem: Policies Not Applied After Adding to Group

**Symptoms:**
- Peer added to group but traffic not filtered
- iptables rules not present on jump peer

**Solutions:**
1. Verify policy is attached to group:
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies"
```

2. Check jump peer agent status:
```bash
systemctl status wireguard-agent
```

3. Check iptables on jump peer:
```bash
iptables -L -n -v
```

4. Check WebSocket connection:
```bash
journalctl -u wireguard-agent | grep -i websocket
```

### Problem: Routes Not Applied After Adding to Group

**Symptoms:**
- Peer added to group but can't reach route destination
- AllowedIPs not updated

**Solutions:**
1. Verify route is attached to group:
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes"
```

2. Check WireGuard configuration:
```bash
wg show
```

3. Verify AllowedIPs includes route CIDR

4. Check jump peer is online and reachable

## Best Practices

### 1. Plan Your Group Structure

Before creating groups, plan:
- How will you organize peers?
- What access patterns do you need?
- How will groups scale as you grow?

### 2. Use Consistent Naming

Establish naming conventions:
- Use lowercase with hyphens: `engineering-team`
- Include purpose: `prod-database-access`
- Avoid abbreviations unless standard

### 3. Document Group Purpose

Always include descriptions:
- What is the group for?
- Who should be in it?
- What access does it provide?

### 4. Regular Audits

Schedule regular reviews:
- Monthly: Review group memberships
- Quarterly: Audit attached policies and routes
- Annually: Restructure if needed

### 5. Principle of Least Privilege

- Start with minimal access
- Add permissions as needed
- Remove unused groups
- Review and revoke unnecessary access

### 6. Use Default Groups Wisely

- Keep default groups simple
- Provide baseline access only
- Don't over-provision
- Review default group policies regularly

### 7. Monitor Group Changes

- Log all group modifications
- Track who makes changes
- Review audit logs regularly
- Alert on suspicious changes

## Related Documentation

- [Policies Management Guide](./policies-management.md)
- [Routes Management Guide](./routes-management.md)
- [Default Groups Configuration](./default-groups.md)
- [API Reference](../api-reference.md)
