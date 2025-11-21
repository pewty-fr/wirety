# Wirety Helm Chart

This Helm chart deploys Wirety Server and Frontend to Kubernetes.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- Ingress controller (nginx recommended)
- cert-manager (optional, for TLS)

## Installation

### Quick Start

```bash
# Add helm repo (once published)
helm repo add wirety https://charts.wirety.example
helm repo update

# Install with default values
helm install wirety wirety/wirety

# Or install from source
helm install wirety ./helm
```

### Custom Installation

Create a `values.yaml` file:

```yaml
ingress:
  enabled: true
  className: nginx
  hosts:
    - host: wirety.yourdomain.com
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
        - wirety.yourdomain.com

server:
  env:
    AUTH_ENABLED: "true"
    AUTH_ISSUER_URL: "https://keycloak.example.com/realms/wirety"
    AUTH_CLIENT_ID: "wirety-client"
    AUTH_CLIENT_SECRET: "your-secret-here"

  resources:
    limits:
      cpu: 1000m
      memory: 1Gi
    requests:
      cpu: 200m
      memory: 256Mi
```

Then install:

```bash
helm install wirety ./helm -f values.yaml
```

## Configuration

### Server Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `server.image.repository` | Server image repository | `ghcr.io/pewty/wirety-server` |
| `server.image.tag` | Server image tag | `Chart.AppVersion` |
| `server.env.HTTP_PORT` | HTTP port | `8080` |
| `server.env.AUTH_ENABLED` | Enable OIDC auth | `false` |
| `server.env.AUTH_ISSUER_URL` | OIDC issuer URL | `""` |
| `server.env.AUTH_CLIENT_ID` | OIDC client ID | `""` |
| `server.env.AUTH_CLIENT_SECRET` | OIDC client secret | `""` |
| `server.resources` | Resource limits/requests | See values.yaml |

### Frontend Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `frontend.image.repository` | Frontend image repository | `ghcr.io/pewty/wirety-front` |
| `frontend.image.tag` | Frontend image tag | `Chart.AppVersion` |
| `frontend.resources` | Resource limits/requests | See values.yaml |

### Ingress Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ingress.enabled` | Enable ingress | `true` |
| `ingress.className` | Ingress class | `nginx` |
| `ingress.hosts` | Ingress hosts configuration | See values.yaml |
| `ingress.tls` | TLS configuration | See values.yaml |

## Upgrading

```bash
helm upgrade wirety ./helm -f values.yaml
```

## Uninstalling

```bash
helm uninstall wirety
```

## Using Secrets for Sensitive Data

It's recommended to store sensitive data (like OIDC client secret) in Kubernetes secrets:

```bash
# Create secret
kubectl create secret generic wirety-server-secrets \
  --from-literal=AUTH_CLIENT_SECRET=your-secret-here

# Update values.yaml
server:
  envFrom:
    - secretRef:
        name: wirety-server-secrets
```

## Troubleshooting

### Check pods status
```bash
kubectl get pods -l app.kubernetes.io/name=wirety
```

### View server logs
```bash
kubectl logs -l app.kubernetes.io/component=server
```

### View frontend logs
```bash
kubectl logs -l app.kubernetes.io/component=frontend
```

### Test connectivity
```bash
kubectl port-forward svc/wirety-server 8080:8080
curl http://localhost:8080/api/v1/health
```

## Development

To test chart locally:

```bash
# Lint the chart
helm lint ./helm

# Dry-run install
helm install wirety ./helm --dry-run --debug

# Template output
helm template wirety ./helm > output.yaml
```
