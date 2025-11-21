---
id: deployment
title: Deployment Guide
sidebar_position: 8
---

This guide covers deploying Wirety in production environments.

## Deployment Options

Wirety can be deployed using:
- **Kubernetes with Helm** (recommended for production)
- **Docker Compose** (suitable for testing and small deployments)
- **Binary deployment** (advanced users)

## Kubernetes Deployment with Helm

### Prerequisites
- Kubernetes cluster (1.24+)
- Helm 3.x
- Ingress controller (nginx, traefik, etc.)
- Storage class for persistent volumes (optional)

### Installation

1. **Add Helm repository** (coming soon)
```bash
# Helm chart will be published to a repository
# For now, use local chart
cd helm
```

2. **Create values file**
```yaml
# values-production.yaml
server:
  replicaCount: 2
  resources:
    requests:
      memory: "256Mi"
      cpu: "250m"
    limits:
      memory: "512Mi"
      cpu: "500m"
  env:
    HTTP_PORT: "8080"
    AUTH_ENABLED: "true"
    AUTH_ISSUER_URL: "https://auth.example.com/realms/wirety"
    AUTH_CLIENT_ID: "wirety-client"
    AUTH_CLIENT_SECRET: "your-secret"
    AUTH_JWKS_CACHE_TTL: "3600"

frontend:
  replicaCount: 2
  resources:
    requests:
      memory: "128Mi"
      cpu: "100m"
    limits:
      memory: "256Mi"
      cpu: "200m"

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: wirety.example.com
      paths:
        - path: /api
          pathType: Prefix
          backend: server
        - path: /
          pathType: Prefix
          backend: frontend
  tls:
    - secretName: wirety-tls
      hosts:
        - wirety.example.com

persistence:
  enabled: false  # Use external database instead
  # storageClass: "standard"
  # size: 10Gi
```

3. **Install the chart**
```bash
helm install wirety ./helm -f values-production.yaml --namespace wirety --create-namespace
```

4. **Verify deployment**
```bash
kubectl get pods -n wirety
kubectl get ingress -n wirety
kubectl logs -n wirety -l app=wirety-server
```

### Upgrading

```bash
helm upgrade wirety ./helm -f values-production.yaml --namespace wirety
```

### Uninstalling

```bash
helm uninstall wirety --namespace wirety
```

## Docker Compose Deployment

Suitable for testing and small-scale deployments.

### Setup

1. **Create docker-compose.yml**
```yaml
version: '3.8'

services:
  server:
    image: rg.fr-par.scw.cloud/wirety/server:latest
    ports:
      - "8080:8080"
    environment:
      HTTP_PORT: "8080"
      AUTH_ENABLED: "false"
    volumes:
      - ./data:/data
    restart: unless-stopped

  frontend:
    image: rg.fr-par.scw.cloud/wirety/frontend:latest
    ports:
      - "80:80"
    environment:
      VITE_API_URL: "http://localhost:8080"
    depends_on:
      - server
    restart: unless-stopped

networks:
  default:
    driver: bridge
```

2. **Start services**
```bash
docker-compose up -d
```

3. **Check logs**
```bash
docker-compose logs -f
```

## Binary Deployment

### Server

1. **Download the binary**
```bash
curl -fsSL https://github.com/pewty/wirety/releases/latest/download/wirety-server-linux-amd64 -o /usr/local/bin/wirety-server
chmod +x /usr/local/bin/wirety-server
```

2. **Create systemd service**
```bash
cat > /etc/systemd/system/wirety-server.service <<EOF
[Unit]
Description=Wirety Server
After=network.target

[Service]
Type=simple
User=wirety
Environment="HTTP_PORT=8080"
Environment="AUTH_ENABLED=false"
ExecStart=/usr/local/bin/wirety-server
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF
```

3. **Enable and start**
```bash
systemctl daemon-reload
systemctl enable wirety-server
systemctl start wirety-server
```

### Frontend

Serve the built frontend using nginx or any web server:

```nginx
server {
    listen 80;
    server_name wirety.example.com;

    root /var/www/wirety/frontend;
    index index.html;

    location /api {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
    }

    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

## Agent Deployment

### Manual Installation

```bash
# Download agent
curl -fsSL https://github.com/pewty/wirety/releases/latest/download/wirety-agent-linux-amd64 -o /usr/local/bin/wirety-agent
chmod +x /usr/local/bin/wirety-agent

# Create systemd service
cat > /etc/systemd/system/wirety-agent.service <<EOF
[Unit]
Description=Wirety Agent
After=network.target

[Service]
Type=simple
Environment="SERVER_URL=https://wirety.example.com"
Environment="TOKEN=your-enrollment-token"
Environment="WG_INTERFACE=wg0"
ExecStart=/usr/local/bin/wirety-agent
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable wirety-agent
systemctl start wirety-agent
```

### Ansible Playbook

```yaml
# playbooks/wirety-agent.yml
---
- name: Deploy Wirety Agent
  hosts: wirety_peers
  become: yes
  
  vars:
    wirety_server_url: "https://wirety.example.com"
    wirety_token: "{{ lookup('env', 'WIRETY_TOKEN') }}"
    wirety_interface: "wg0"
    
  tasks:
    - name: Install WireGuard
      apt:
        name: wireguard
        state: present
        
    - name: Download Wirety agent
      get_url:
        url: https://github.com/pewty/wirety/releases/latest/download/wirety-agent-linux-amd64
        dest: /usr/local/bin/wirety-agent
        mode: '0755'
        
    - name: Create systemd service
      template:
        src: wirety-agent.service.j2
        dest: /etc/systemd/system/wirety-agent.service
        
    - name: Enable and start agent
      systemd:
        name: wirety-agent
        enabled: yes
        state: started
        daemon_reload: yes
```

## Environment Configuration

### Server Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| HTTP_PORT | Server port | 8080 | No |
| AUTH_ENABLED | Enable OIDC auth | false | No |
| AUTH_ISSUER_URL | OIDC provider URL | - | If auth enabled |
| AUTH_CLIENT_ID | OIDC client ID | - | If auth enabled |
| AUTH_CLIENT_SECRET | OIDC client secret | - | If auth enabled |
| AUTH_JWKS_CACHE_TTL | JWKS cache duration (seconds) | 3600 | No |

### Agent Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| SERVER_URL | Wirety server URL | - | Yes |
| TOKEN | Enrollment token | - | Yes |
| WG_INTERFACE | WireGuard interface name | wg0 | No |
| WG_CONFIG_PATH | Config file path | /etc/wireguard/wg0.conf | No |
| WG_APPLY_METHOD | Apply method (wg-quick/syncconf) | wg-quick | No |
| NAT_INTERFACE | NAT interface for jump peers | eth0 | No |

## Monitoring

### Health Checks

**Server health endpoint:**
```bash
curl http://localhost:8080/health
```

**Agent status:**
```bash
systemctl status wirety-agent
wg show wg0
```

### Metrics (Future)

Prometheus metrics will be exposed at `/metrics`:
- Peer count
- Active connections
- Incident count
- IPAM utilization

## Backup and Recovery

### Server Data

If using in-memory storage (default), data is lost on restart. For production:
- Implement persistent storage backend
- Regular backups of peer configurations
- Export network configurations

### Configuration Backup

```bash
# Export peer configs
curl -H "Authorization: Bearer $TOKEN" \
  https://wirety.example.com/api/v1/peers > peers-backup.json

# Export networks
curl -H "Authorization: Bearer $TOKEN" \
  https://wirety.example.com/api/v1/networks > networks-backup.json
```

## Security Considerations

1. **TLS/HTTPS**: Always use HTTPS in production
2. **Authentication**: Enable OIDC authentication
3. **Network policies**: Restrict access to server API
4. **Secrets management**: Use Kubernetes secrets or vault
5. **Token rotation**: Regularly rotate enrollment tokens
6. **Firewall rules**: Restrict WireGuard ports
7. **Incident monitoring**: Set up alerts for security incidents

## Troubleshooting

### Server not starting
```bash
kubectl logs -n wirety -l app=wirety-server
# Check environment variables
# Verify OIDC configuration
```

### Agent enrollment fails
```bash
journalctl -u wirety-agent -f
# Verify token validity
# Check server URL accessibility
# Ensure WireGuard is installed
```

### Connectivity issues
```bash
# Check WireGuard status
wg show wg0

# Test peer connectivity
ping <peer-ip>

# Check allowed IPs
wg show wg0 allowed-ips

# Verify routing
ip route | grep wg0
```

## Performance Tuning

### Server
- Increase replicas for high availability
- Use horizontal pod autoscaling
- Configure resource limits appropriately

### WireGuard
- Adjust MTU for optimal performance
- Use persistent keepalive for NAT traversal
- Monitor bandwidth and connection limits

## Next Steps

- [OIDC Configuration](./guides/oidc)
- [Incident Management](./incidents)
- [Network Configuration](./network)
