---
id: deployment
title: Deployment Guide
sidebar_position: 6
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
    LOG_LEVEL: "info"    # trace|debug|info|warn|error|fatal
    LOG_FORMAT: "json"   # json recommended for log aggregators (Loki, Datadog, etc.)

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
      LOG_LEVEL: "info"     # trace|debug|info|warn|error|fatal
      LOG_FORMAT: "json"    # json is recommended when logs are ingested by a collector
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
curl -fsSL https://github.com/pewty-fr/wirety/releases/latest/download/wirety-server-linux-amd64 -o /usr/local/bin/wirety-server
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
Environment="LOG_LEVEL=info"
Environment="LOG_FORMAT=json"
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
# Detect architecture
ARCH=$(uname -m)
case $ARCH in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
esac

# Download the agent binary
curl -fsSL "https://github.com/pewty-fr/wirety/releases/latest/download/wirety-agent-linux-${ARCH}" \
  -o /usr/local/bin/wirety-agent
chmod +x /usr/local/bin/wirety-agent
```

### Systemd Service

The following unit file provides a production-ready configuration with security hardening and automatic restarts.

```ini
[Unit]
Description=Wirety Agent VPN Service
Documentation=https://github.com/pewty-fr/wirety
After=network-online.target systemd-modules-load.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/wirety-agent \
  --token <ENROLLMENT_TOKEN> \
  --server https://wirety.example.com \
  --portal-url https://wirety.example.com/captive-portal
Restart=on-failure
RestartSec=5
TimeoutStopSec=30

# Logging
StandardOutput=append:/var/log/wirety/wirety.log
StandardError=append:/var/log/wirety/wirety.log

# Runtime directories (created by systemd before start)
RuntimeDirectory=wireguard
RuntimeDirectoryMode=0755
LogsDirectory=wirety
LogsDirectoryMode=0750

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/etc/wireguard
PrivateTmp=yes
PrivateDevices=no
ProtectKernelTunables=no
ProtectKernelModules=no
ProtectControlGroups=yes
RestrictAddressFamilies=AF_INET AF_INET6 AF_NETLINK AF_UNIX
RestrictNamespaces=yes
LockPersonality=yes
MemoryDenyWriteExecute=yes
RestrictRealtime=yes

# Resource limits
LimitNOFILE=65536
LimitNPROC=512

[Install]
WantedBy=multi-user.target
```

For **reverse-proxy setups** where the server is accessed by IP (no DNS), add `--server-host` and optionally `--skip-tls-verify` if the proxy uses a self-signed certificate:

```ini
ExecStart=/usr/local/bin/wirety-agent \
  --token <ENROLLMENT_TOKEN> \
  --server https://10.0.0.1 \
  --server-host wirety.internal \
  --portal-url https://wirety.internal/captive-portal \
  --skip-tls-verify
```

Enable and start the service:

```bash
systemctl daemon-reload
systemctl enable --now wirety
```

View logs:

```bash
journalctl -u wirety -f
```

### Cloud-Init (Automated Jump Peer Provisioning)

The following cloud-init config fully provisions a jump peer VM from scratch — kernel modules, sysctl tuning, security hardening, automatic updates, and Wirety agent installation. Substitute the `${token}`, `${server}`, and `${host}` variables before use.

```yaml
#cloud-config
package_update: true
package_upgrade: true
package_reboot_if_required: true

timezone: Europe/Paris
locale: en_US.UTF-8

packages:
  # Base utilities
  - curl
  - wget
  - ca-certificates
  - git
  - jq
  # WireGuard
  - wireguard
  - wireguard-tools
  # Network diagnostics
  - net-tools
  - bind9-dnsutils
  - iputils-ping
  - traceroute
  - tcpdump
  # System monitoring
  - htop
  - lsof
  # Log management
  - logrotate
  # Security
  - fail2ban
  - unattended-upgrades
  # Time sync
  - chrony

write_files:
  # Kernel modules required by Wirety iptables rules
  - path: /etc/modules-load.d/wirety.conf
    content: |
      # Required for connection tracking (ESTABLISHED/RELATED rules)
      nf_conntrack
      # xtables compatibility layer — allows xt_string to work with iptables-nft
      nft_compat
      # Required for iptables string matching (SNI / Host-header vhost isolation)
      xt_string

  # Sysctl tuning: IP forwarding, security hardening, TCP performance
  - path: /etc/sysctl.d/99-wirety.conf
    content: |
      # IP forwarding for WireGuard VPN
      net.ipv4.ip_forward = 1
      net.ipv6.conf.all.forwarding = 1

      # Reverse path filtering
      net.ipv4.conf.all.rp_filter = 1
      net.ipv4.conf.default.rp_filter = 1

      # Disable ICMP redirects
      net.ipv4.conf.all.accept_redirects = 0
      net.ipv4.conf.default.accept_redirects = 0
      net.ipv4.conf.all.send_redirects = 0
      net.ipv6.conf.all.accept_redirects = 0

      # Ignore broadcast pings
      net.ipv4.icmp_echo_ignore_broadcasts = 1

      # TCP tuning
      net.core.somaxconn = 65535
      net.ipv4.tcp_rmem = 4096 87380 16777216
      net.ipv4.tcp_wmem = 4096 65536 16777216

      # Increase file descriptor limit
      fs.file-max = 1000000

  - path: /etc/security/limits.d/99-wirety.conf
    content: |
      * soft nofile 65536
      * hard nofile 65536
      root soft nofile 65536
      root hard nofile 65536

  # Log rotation for Wirety logs
  - path: /etc/logrotate.d/wirety
    content: |
      /var/log/wirety/*.log {
        daily
        rotate 14
        compress
        delaycompress
        missingok
        notifempty
        create 0640 root root
      }

  # fail2ban: protect SSH
  - path: /etc/fail2ban/jail.local
    content: |
      [DEFAULT]
      bantime  = 1h
      findtime = 10m
      maxretry = 5

      [sshd]
      enabled = true

  # Unattended security upgrades
  - path: /etc/apt/apt.conf.d/20auto-upgrades
    content: |
      APT::Periodic::Update-Package-Lists "1";
      APT::Periodic::Unattended-Upgrade "1";
      APT::Periodic::AutocleanInterval "7";

  - path: /etc/apt/apt.conf.d/50unattended-upgrades
    content: |
      Unattended-Upgrade::Allowed-Origins {
        "${distro_id}:${distro_codename}-security";
      };
      Unattended-Upgrade::AutoFixInterruptedDpkg "true";
      Unattended-Upgrade::MinimalSteps "true";
      Unattended-Upgrade::Remove-Unused-Dependencies "true";
      Unattended-Upgrade::Automatic-Reboot "false";

  # Wirety systemd unit
  - path: /etc/systemd/system/wirety.service
    permissions: '0644'
    content: |
      [Unit]
      Description=Wirety Agent VPN Service
      Documentation=https://github.com/pewty-fr/wirety
      After=network-online.target systemd-modules-load.service
      Wants=network-online.target

      [Service]
      Type=simple
      ExecStart=/usr/local/bin/wirety-agent \
        --token ${token} \
        --server ${server} \
        --server-host ${host} \
        --portal-url https://${host}/captive-portal \
        --skip-tls-verify
      Restart=on-failure
      RestartSec=5
      TimeoutStopSec=30
      StandardOutput=append:/var/log/wirety/wirety.log
      StandardError=append:/var/log/wirety/wirety.log
      RuntimeDirectory=wireguard
      RuntimeDirectoryMode=0755
      LogsDirectory=wirety
      LogsDirectoryMode=0750
      NoNewPrivileges=yes
      ProtectSystem=strict
      ProtectHome=yes
      ReadWritePaths=/etc/wireguard
      PrivateTmp=yes
      PrivateDevices=no
      ProtectKernelTunables=no
      ProtectKernelModules=no
      ProtectControlGroups=yes
      RestrictAddressFamilies=AF_INET AF_INET6 AF_NETLINK AF_UNIX
      RestrictNamespaces=yes
      LockPersonality=yes
      MemoryDenyWriteExecute=yes
      RestrictRealtime=yes
      LimitNOFILE=65536
      LimitNPROC=512

      [Install]
      WantedBy=multi-user.target

  # Install script: downloads the agent binary and configures the system
  - path: /usr/local/bin/install-wirety.sh
    permissions: '0755'
    content: |
      #!/bin/bash
      set -euo pipefail

      WIRETY_VERSION="1.0.0"
      ARCH=$(uname -m)
      case $ARCH in
        x86_64)  ARCH="amd64" ;;
        aarch64) ARCH="arm64" ;;
        *)
          echo "Unsupported architecture: $ARCH" >&2
          exit 1
          ;;
      esac

      echo "Downloading wirety-agent v${WIRETY_VERSION} for ${ARCH}..."
      curl -fsSL \
        "https://github.com/pewty-fr/wirety/releases/download/wirety-agent%2Fv${WIRETY_VERSION}/wirety-agent-linux-${ARCH}" \
        -o /usr/local/bin/wirety-agent
      chmod +x /usr/local/bin/wirety-agent

      systemctl daemon-reload
      systemctl enable --now wirety
      systemctl enable --now fail2ban

runcmd:
  - modprobe xt_string || true
  - modprobe nf_conntrack || true
  - sysctl --system
  - /usr/local/bin/install-wirety.sh
  - echo "Wirety installation completed at $(date -u)" >> /var/log/cloud-init-output.log
```

:::caution `--skip-tls-verify`
The `--skip-tls-verify` flag disables TLS certificate validation for the connection between the agent and the Wirety server. Only use it when the server is behind a reverse proxy with a self-signed certificate that the agent cannot verify (e.g. internal CA). Never use it in production environments with publicly signed certificates.
:::

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

  tasks:
    - name: Install WireGuard
      apt:
        name: wireguard
        state: present

    - name: Download Wirety agent
      get_url:
        url: "https://github.com/pewty-fr/wirety/releases/latest/download/wirety-agent-linux-{{ 'arm64' if ansible_architecture == 'aarch64' else 'amd64' }}"
        dest: /usr/local/bin/wirety-agent
        mode: '0755'

    - name: Create systemd service
      template:
        src: wirety.service.j2
        dest: /etc/systemd/system/wirety.service

    - name: Enable and start agent
      systemd:
        name: wirety
        enabled: yes
        state: started
        daemon_reload: yes
```

## Environment Configuration

### Server Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| HTTP_PORT | Server port | `8080` | No |
| AUTH_ENABLED | Enable OIDC auth | `false` | No |
| AUTH_ISSUER_URL | OIDC provider URL | — | If auth enabled |
| AUTH_CLIENT_ID | OIDC client ID | — | If auth enabled |
| AUTH_CLIENT_SECRET | OIDC client secret | — | If auth enabled |
| AUTH_JWKS_CACHE_TTL | JWKS cache duration (seconds) | `3600` | No |
| COOKIE_SECURE | Set `Secure` flag on session cookie — set to `false` only for local HTTP dev | `true` | No |
| LOG_LEVEL | Log verbosity: `trace`\|`debug`\|`info`\|`warn`\|`error`\|`fatal` | `info` | No |
| LOG_FORMAT | Log output format: `text`\|`json` | `text` | No |
| AUDIT_LOG | Emit JSON audit events to stdout | `false` | No |

### Agent Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| SERVER_URL | Wirety server URL | — | Yes |
| TOKEN | Enrollment token | — | Yes |
| WG_CONFIG_PATH | WireGuard config file path | — | No |
| WG_APPLY_METHOD | Apply method: `wg-quick`\|`syncconf` | `syncconf` | No |
| NAT_INTERFACES | Comma-separated NAT interfaces | auto-detect | No |
| CAPTIVE_PORTAL_URL | Captive portal page URL | `<SERVER_URL>/captive-portal` | No |
| SERVER_HOST | Override HTTP Host header (reverse-proxy setups) | — | No |
| SKIP_TLS_VERIFY | Disable TLS verification | `false` | No |
| LOG_LEVEL | Log verbosity: `trace`\|`debug`\|`info`\|`warn`\|`error`\|`fatal` | `info` | No |
| LOG_FORMAT | Log output format: `text`\|`json` | `text` | No |
| AUDIT_LOG | Emit JSON audit events to stdout | `false` | No |

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

- [OIDC Configuration](/guides/oidc.md)
- [Incident Management](/incidents.md)
- [Network Configuration](/network.md)
