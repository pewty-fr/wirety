# Wirety Agent

The Wirety Agent is a WireGuard management agent that connects to the Wirety server to manage VPN configurations, DNS, and network policies.

## Installation

### Binary Installation

Download the latest binary from the [releases page](https://github.com/pewty/wirety/releases) for your platform:

- `wirety-agent-linux-amd64` - Linux x86_64
- `wirety-agent-linux-arm64` - Linux ARM64
- `wirety-agent-darwin-amd64` - macOS Intel
- `wirety-agent-darwin-arm64` - macOS Apple Silicon
- `wirety-agent-windows-amd64.exe` - Windows x86_64

### Docker Installation

The agent is available as a Docker image for containerized deployments:

```bash
docker pull rg.fr-par.scw.cloud/wirety/agent:latest
```

## Usage

### Binary Usage

```bash
./wirety-agent --server-url https://your-server.com --token your-enrollment-token
```

### Docker Usage

```bash
docker run -d \
  --name wirety-agent \
  --cap-add NET_ADMIN \
  --cap-add NET_RAW \
  --cap-add SYS_MODULE \
  -e SERVER_URL=https://your-server.com \
  -e TOKEN=your-enrollment-token \
  -v /etc/wireguard:/etc/wireguard \
  rg.fr-par.scw.cloud/wirety/agent:latest
```

### Kubernetes Deployment

The agent can be deployed in Kubernetes using either a Deployment or DaemonSet:

#### Deployment (Single Instance)
```bash
kubectl apply -f k8s/deployment.yaml
```

#### DaemonSet (One Per Node)
```bash
kubectl apply -f k8s/daemonset.yaml
```

Before deploying, update the secret in the manifest with your server URL and enrollment token:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: wirety-agent-config
type: Opaque
stringData:
  server-url: "https://your-wirety-server.com"
  token: "your-enrollment-token-here"
```

## Configuration

The agent can be configured using environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `SERVER_URL` | Wirety server URL | Required |
| `TOKEN` | Enrollment token | Required |
| `WG_CONFIG_PATH` | WireGuard config directory | `/etc/wireguard` |
| `WG_APPLY_METHOD` | WireGuard apply method | `syncconf` |
| `NAT_INTERFACE` | NAT interface (auto-detected if empty) | Auto-detect |
| `HTTP_PROXY_PORT` | HTTP proxy port | `3128` |
| `HTTPS_PROXY_PORT` | HTTPS proxy port | `3129` |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |

## Building

### Build Binary

```bash
make build
```

### Build for All Platforms

```bash
make build-all
```

### Build Docker Image

```bash
make docker-build
```

### Build Multi-Architecture Docker Image

```bash
make docker-build-multiarch
```

## Requirements

### System Requirements

- Linux kernel with WireGuard support
- iptables/netfilter support
- Root privileges (for network configuration)

### Docker Requirements

- Docker with BuildKit support
- Multi-architecture build support (for cross-platform images)

### Kubernetes Requirements

- Kubernetes 1.20+
- Nodes with WireGuard kernel module support
- Privileged containers support (for DaemonSet deployment)

## Security

The Docker image includes security best practices:

- Runs as non-root user (except in DaemonSet mode)
- Minimal attack surface with Alpine Linux base
- Health checks for monitoring
- Proper signal handling for graceful shutdown
- Read-only root filesystem where possible

## Troubleshooting

### Check Agent Status

```bash
# Binary
./wirety-agent --help

# Docker
docker logs wirety-agent

# Kubernetes
kubectl logs -l app=wirety-agent
```

### Verify WireGuard Interface

```bash
# Check if WireGuard interface is up
ip link show wg0

# Check WireGuard status
wg show
```

### Health Check

The Docker image includes a health check script:

```bash
# Manual health check
docker exec wirety-agent /app/healthcheck.sh
```
