---
id: troubleshooting
title: Troubleshooting
sidebar_position: 9
---

Common issues and solutions when working with Wirety.

## Server Issues

### Server fails to start

**Symptoms:**
- Pod/container crashes immediately
- Error logs show configuration issues

**Solutions:**

1. **Check environment variables**
```bash
kubectl logs -n wirety <server-pod-name>
# or
docker logs wirety-server
```

2. **Verify OIDC configuration**
```bash
# If AUTH_ENABLED=true, ensure all auth variables are set:
# - AUTH_ISSUER_URL
# - AUTH_CLIENT_ID
# - AUTH_CLIENT_SECRET
```

3. **Check port availability**
```bash
# Ensure HTTP_PORT is not already in use
netstat -tlnp | grep 8080
```

### Authentication not working

**Symptoms:**
- Login redirects fail
- 401 Unauthorized errors
- JWKS fetch errors

**Solutions:**

1. **Verify OIDC provider is accessible**
```bash
curl https://auth.example.com/realms/wirety/.well-known/openid-configuration
```

2. **Check redirect URIs**
   - Ensure `https://wirety.example.com/api/v1/auth/callback` is registered in OIDC provider

3. **Verify clock synchronization**
```bash
# Token validation fails if clocks are skewed
timedatectl status
ntpdate -q pool.ntp.org
```

4. **Check JWKS cache**
   - Restart server if JWKS is stale
   - Adjust `AUTH_JWKS_CACHE_TTL` if needed

### WebSocket connection fails

**Symptoms:**
- Agents don't receive real-time updates
- Frontend shows disconnection warnings

**Solutions:**

1. **Check reverse proxy configuration**
```nginx
# Nginx needs WebSocket upgrade headers
location /api {
    proxy_pass http://backend;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
```

2. **Verify firewall rules**
   - Ensure WebSocket connections are not blocked

## Agent Issues

### Agent enrollment fails

**Symptoms:**
- Error: "enrollment failed"
- 401/403 HTTP errors
- Connection timeout

**Solutions:**

1. **Verify token validity**
   - Generate new token from UI
   - Check token hasn't expired or been revoked

2. **Check server URL accessibility**
```bash
curl -v https://wirety.example.com/api/v1/health
```

3. **Verify WireGuard is installed**
```bash
which wg
modprobe wireguard
```

4. **Check agent logs**
```bash
journalctl -u wirety-agent -f
# or
./wirety-agent -server https://wirety.example.com -token <token>
```

### Agent config not updating

**Symptoms:**
- Changes in UI don't reflect on agent
- WireGuard config remains old

**Solutions:**

1. **Check agent is running**
```bash
systemctl status wirety-agent
ps aux | grep wirety-agent
```

2. **Verify WebSocket connection**
   - Agent should reconnect after network issues
   - Check logs for WebSocket errors

3. **Manual config refresh**
```bash
# Restart agent to force refresh
systemctl restart wirety-agent
```

4. **Check ACL blocking**
   - Peer may be blocked due to incident
   - Resolve incident in UI

### WireGuard interface issues

**Symptoms:**
- `wg0` interface not created
- Interface exists but no connectivity

**Solutions:**

1. **Check WireGuard module**
```bash
lsmod | grep wireguard
modprobe wireguard
```

2. **Verify permissions**
```bash
# Agent needs CAP_NET_ADMIN or root
sudo -u wirety-agent wg show
```

3. **Check config syntax**
```bash
wg-quick up /etc/wireguard/wg0.conf
# Check for syntax errors
```

4. **Verify routing**
```bash
ip route | grep wg0
# Should see routes for allowed IPs
```

## Connectivity Issues

### Cannot ping other peers

**Symptoms:**
- Ping timeouts between peers
- `wg show` shows no handshake

**Solutions:**

1. **Verify peer is not isolated**
   - Check `is_isolated` flag in UI
   - Isolated peers can only reach jump peers

2. **Check allowed IPs**
```bash
wg show wg0 allowed-ips
# Should include target peer's IP
```

3. **Verify endpoint reachability**
```bash
# Test UDP connectivity to peer endpoint
nc -u -v <peer-ip> <peer-port>
```

4. **Check NAT/firewall**
   - Ensure WireGuard port is open (default: 51820)
   - Configure port forwarding if behind NAT

5. **Persistent keepalive**
   - For NAT traversal, add persistent keepalive:
   ```
   PersistentKeepalive = 25
   ```

### Jump peer routing not working

**Symptoms:**
- Full encapsulation enabled but traffic not routed
- Can reach jump peer but not internet

**Solutions:**

1. **Verify NAT is configured**
```bash
# On jump peer
iptables -t nat -L POSTROUTING -n -v
# Should see MASQUERADE rule for wg0
```

2. **Enable IP forwarding**
```bash
# On jump peer
sysctl net.ipv4.ip_forward
# Should be 1
echo 1 > /proc/sys/net/ipv4/ip_forward
```

3. **Check additional allowed IPs**
   - Verify `0.0.0.0/0` is in AllowedIPs for full encapsulation

4. **Verify default route**
```bash
# On regular peer with full encapsulation
ip route
# Default route should point to wg0
```

### Session conflict incidents

**Symptoms:**
- Peer automatically blocked
- Incident type: "session conflict"

**Solutions:**

1. **Check for duplicate agents**
   - Ensure only one agent instance per peer
   - Stop duplicate agents

2. **Review enrollment process**
   - Don't reuse tokens across multiple hosts
   - Each host needs unique peer entry

3. **Resolve incident**
   - Once duplicate is removed, resolve incident in UI
   - Peer will receive config updates again

## IPAM Issues

### IP allocation fails

**Symptoms:**
- Error: "no available IPs"
- Peer creation fails

**Solutions:**

1. **Check network capacity**
   - View network in UI to see capacity
   - Expand CIDR if needed (only if no static peers)

2. **Delete unused peers**
   - Free up IPs by removing old peers

### Cannot change network CIDR

**Symptoms:**
- Error: "static peers exist"

**Solutions:**

1. **Convert static peers to dynamic**
   - Or remove static peers before CIDR change
   - Static peers require manual reconfiguration

2. **Create new network**
   - Alternative: create new network with desired CIDR
   - Migrate peers gradually

## Performance Issues

### High latency

**Symptoms:**
- Slow response times
- Packet loss

**Solutions:**

1. **Check MTU settings**
```bash
# Typical WireGuard MTU
ip link set wg0 mtu 1420
```

2. **Optimize routing**
   - Use direct peer connections where possible
   - Minimize hops through jump peers

3. **Check bandwidth**
```bash
# Monitor WireGuard traffic
watch -n 1 wg show wg0 transfer
```

### Agent high CPU usage

**Symptoms:**
- Agent consuming excessive CPU
- System slowdown

**Solutions:**

1. **Check polling interval**
   - Agent polls for updates
   - Verify no excessive reconnection loops

2. **Review logs**
```bash
journalctl -u wirety-agent | grep -i error
```

3. **Update to latest version**
   - Bug fixes may improve performance

## Database/Storage Issues

### In-memory data lost on restart

**Symptoms:**
- All peers/networks gone after server restart

**Solution:**
- This is expected with default in-memory storage
- Implement persistent storage backend (future feature)
- Export configurations regularly as backup

## Security Issues

### Endpoint changes triggering incidents

**Symptoms:**
- Multiple endpoint change incidents
- Peer blocked automatically

**Solutions:**

1. **Use static endpoints for jump peers**
   - Configure fixed public IP
   - Use DNS with short TTL if IP changes

2. **Review detection thresholds**
   - 30 minutes for shared config detection
   - 10 changes/day for suspicious activity

3. **Resolve legitimate incidents**
   - If legitimate (e.g., network changes), resolve in UI

## Logging and Debugging

### Enable verbose logging

**Server:**
```bash
# Add environment variable
DEBUG=true
LOG_LEVEL=debug
```

**Agent:**
```bash
# Run in foreground with verbose output
wirety-agent -server https://wirety.example.com -token <token> -v
```

### Collect diagnostic information

```bash
# WireGuard status
wg show all dump

# Agent status
systemctl status wirety-agent
journalctl -u wirety-agent -n 100

# Network connectivity
ip route
ip addr
iptables -t nat -L -n -v

# DNS resolution
nslookup wirety.example.com

# Server logs
kubectl logs -n wirety <server-pod> --tail=100
```

## Getting Help

If issues persist:

1. **Check GitHub Issues**
   - Search for similar problems
   - Review closed issues for solutions

2. **Open an Issue**
   - Provide clear description
   - Include relevant logs
   - Specify versions and environment

3. **Include Diagnostics**
   ```bash
   # Gather relevant information
   wg show > wg-status.txt
   journalctl -u wirety-agent > agent-logs.txt
   kubectl logs -n wirety <server-pod> > server-logs.txt
   ```

## Related Documentation

- [Agent Configuration](./agent)
- [Server Configuration](./server)
- [Network Setup](./network)
- [OIDC Guide](./guides/oidc)
