# Quick Reference

Quick reference for common operations in the WireGuard network management system.

## Environment Setup

```bash
export TOKEN="your-admin-token"
export API_URL="https://your-server/api/v1"
export NETWORK_ID="your-network-id"
```

## Groups

### Create Group
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "group-name", "description": "Description"}'
```

### List Groups
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups"
```

### Add Peer to Group
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Remove Peer from Group
```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Delete Group
```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID" \
  -H "Authorization: Bearer $TOKEN"
```

## Policies

### Create Policy
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "policy-name",
    "description": "Description",
    "rules": [
      {
        "direction": "output",
        "action": "allow",
        "target": "0.0.0.0/0",
        "target_type": "cidr",
        "description": "Allow all outbound"
      }
    ]
  }'
```

### Get Policy Templates
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/policies/templates"
```

### Attach Policy to Group
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Detach Policy from Group
```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Add Rule to Policy
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/policies/$POLICY_ID/rules" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "direction": "input",
    "action": "allow",
    "target": "10.0.0.0/24",
    "target_type": "cidr",
    "description": "Allow from network"
  }'
```

## Routes

### Create Route
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/routes" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "route-name",
    "description": "Description",
    "destination_cidr": "172.31.0.0/16",
    "jump_peer_id": "jump-peer-id",
    "domain_suffix": "internal"
  }'
```

### List Routes
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/routes"
```

### Attach Route to Group
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Detach Route from Group
```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

## DNS Mappings

### Create DNS Mapping
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "server-name",
    "ip_address": "172.31.10.50"
  }'
```

### List DNS Mappings for Route
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns"
```

### Get All Network DNS Records
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/dns"
```

## Default Groups

### Configure Default Groups
```bash
curl -X PUT "$API_URL/networks/$NETWORK_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "default_group_ids": ["group-id-1", "group-id-2"]
  }'
```

## Common Patterns

### Fully Encapsulated Policy
```json
{
  "name": "fully-encapsulated",
  "rules": [
    {
      "direction": "output",
      "action": "allow",
      "target": "0.0.0.0/0",
      "target_type": "cidr"
    },
    {
      "direction": "input",
      "action": "deny",
      "target": "0.0.0.0/0",
      "target_type": "cidr"
    }
  ]
}
```

### Isolated Policy
```json
{
  "name": "isolated",
  "rules": [
    {
      "direction": "input",
      "action": "deny",
      "target": "0.0.0.0/0",
      "target_type": "cidr"
    },
    {
      "direction": "output",
      "action": "deny",
      "target": "0.0.0.0/0",
      "target_type": "cidr"
    }
  ]
}
```

### Network-Only Policy
```json
{
  "name": "network-only",
  "rules": [
    {
      "direction": "input",
      "action": "allow",
      "target": "10.0.0.0/24",
      "target_type": "cidr"
    },
    {
      "direction": "output",
      "action": "allow",
      "target": "10.0.0.0/24",
      "target_type": "cidr"
    }
  ]
}
```

## Diagnostics

### Check Group Details
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID" | jq
```

### Check Group Policies
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies" | jq
```

### Check Group Routes
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes" | jq
```

### Check iptables on Jump Peer
```bash
ssh jump-peer "sudo iptables -L -n -v"
```

### Check WireGuard Config
```bash
ssh peer "sudo wg show"
```

### Check DNS Resolution
```bash
ssh peer "nslookup server.route.internal"
```

### Check Agent Status
```bash
ssh jump-peer "sudo systemctl status wireguard-agent"
```

### Check Agent Logs
```bash
ssh jump-peer "sudo journalctl -u wireguard-agent -f"
```

## Bulk Operations

### Add Multiple Peers to Group
```bash
for PEER_ID in peer-1 peer-2 peer-3; do
  curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
    -H "Authorization: Bearer $TOKEN"
done
```

### Create Multiple DNS Mappings
```bash
declare -A servers=(
  ["web"]="172.31.10.10"
  ["app"]="172.31.10.20"
  ["db"]="172.31.10.30"
)

for name in "${!servers[@]}"; do
  curl -X POST "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"name\": \"$name\", \"ip_address\": \"${servers[$name]}\"}"
done
```

## Error Codes

- `200 OK` - Success
- `201 Created` - Resource created
- `204 No Content` - Success (no response body)
- `400 Bad Request` - Invalid request
- `403 Forbidden` - Not authorized (admin required)
- `404 Not Found` - Resource not found
- `409 Conflict` - Duplicate name or constraint violation
- `500 Internal Server Error` - Server error

## Policy Rule Fields

- **direction**: `input` or `output`
- **action**: `allow` or `deny`
- **target**: IP/CIDR, peer ID, or group ID
- **target_type**: `cidr`, `peer`, or `group`

## DNS FQDN Format

`name.route-name.domain-suffix`

Example: `database.aws-vpc.aws.internal`

## Related Documentation

- [API Reference](./api-reference.md)
- [User Guide](./user-guide.md)
- [Migration Guide](./migration-guide.md)
- [Groups Management](./guides/groups-management.md)
