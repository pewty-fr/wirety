---
id: deployment
title: Guide de Déploiement
sidebar_position: 8
---

Ce guide couvre le déploiement de Wirety dans des environnements de test et de production.

## Options de Déploiement
Wirety peut être déployé via :
- **Kubernetes avec Helm** (recommandé production)
- **Docker Compose** (POC / petites installations)
- **Binaires** (utilisateurs avancés, intégrations personnalisées)

## Déploiement Kubernetes (Helm)

### Prérequis
- Cluster Kubernetes (>=1.24)
- Helm 3.x
- Contrôleur Ingress (nginx, traefik...)
- DNS pointant vers l'Ingress
- (Optionnel) Classe de stockage si persistance activée

### Installation
1. Cloner ou récupérer le chart (OCI) :
```bash
# Chart local (exemple)
cd helm
```
2. Créer un fichier de valeurs :
```yaml
# values-production.yaml
server:
  replicaCount: 2
  env:
    HTTP_PORT: "8080"
    AUTH_ENABLED: "true"
    AUTH_ISSUER_URL: "https://auth.example.com/realms/wirety"
    AUTH_CLIENT_ID: "wirety-client"
    AUTH_CLIENT_SECRET: "votre-secret"
    AUTH_JWKS_CACHE_TTL: "3600"
frontend:
  replicaCount: 2
ingress:
  enabled: true
  className: nginx
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
  enabled: false
```
3. Installer :
```bash
helm install wirety ./helm -f values-production.yaml --namespace wirety --create-namespace
```
4. Vérifier :
```bash
kubectl get pods -n wirety
kubectl get ingress -n wirety
kubectl logs -n wirety -l app=wirety-server
```

### Mise à jour
```bash
helm upgrade wirety ./helm -f values-production.yaml -n wirety
```

### Suppression
```bash
helm uninstall wirety -n wirety
```

## Déploiement Docker Compose
Pour démo ou environnements simples.

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
```
Démarrer :
```bash
docker compose up -d
```
Logs :
```bash
docker compose logs -f
```

## Déploiement Binaire
### Serveur
```bash
curl -fsSL https://github.com/pewty/wirety/releases/latest/download/wirety-server-linux-amd64 -o /usr/local/bin/wirety-server
chmod +x /usr/local/bin/wirety-server
```
Service systemd :
```bash
cat > /etc/systemd/system/wirety-server.service <<'EOF'
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
systemctl daemon-reload
systemctl enable --now wirety-server
```

### Frontend
Servir avec nginx :
```nginx
server {
  listen 80;
  server_name wirety.example.com;
  root /var/www/wirety/frontend;
  index index.html;
  location /api {
    proxy_pass http://localhost:8080;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
  }
  location / { try_files $uri $uri/ /index.html; }
}
```

### Agent
```bash
curl -fsSL https://github.com/pewty/wirety/releases/latest/download/wirety-agent-linux-amd64 -o /usr/local/bin/wirety-agent
chmod +x /usr/local/bin/wirety-agent
cat > /etc/systemd/system/wirety-agent.service <<'EOF'
[Unit]
Description=Wirety Agent
After=network.target
[Service]
Type=simple
Environment="SERVER_URL=https://wirety.example.com"
Environment="TOKEN=<TOKEN>"
Environment="WG_INTERFACE=wg0"
ExecStart=/usr/local/bin/wirety-agent
Restart=on-failure
[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now wirety-agent
```

## Variables d'Environnement
### Serveur
| Variable | Description | Défaut |
|----------|-------------|--------|
| HTTP_PORT | Port HTTP | 8080 |
| AUTH_ENABLED | Active OIDC | false |
| AUTH_ISSUER_URL | URL fournisseur OIDC | (vide) |
| AUTH_CLIENT_ID | Client ID | (vide) |
| AUTH_CLIENT_SECRET | Secret client | (vide) |
| AUTH_JWKS_CACHE_TTL | TTL cache JWKS | 3600 |

### Agent
| Variable | Description | Défaut |
|----------|-------------|--------|
| SERVER_URL | URL serveur Wirety | (vide) |
| TOKEN | Token d'inscription | (vide) |
| WG_INTERFACE | Interface WireGuard | wg0 |
| WG_CONFIG_PATH | Chemin config | /etc/wireguard/wg0.conf |
| WG_APPLY_METHOD | Méthode (wg-quick/syncconf) | wg-quick |
| NAT_INTERFACE | Interface NAT (jump) | eth0 |

## Supervision
- Endpoint santé : `GET /health`
- Statut agent : `systemctl status wirety-agent` + `wg show wg0`
- (Futur) métriques Prometheus `/metrics`

## Sauvegarde / Restauration
Exporter configurations :
```bash
curl -H "Authorization: Bearer $TOKEN" https://wirety.example.com/api/v1/peers > peers.json
curl -H "Authorization: Bearer $TOKEN" https://wirety.example.com/api/v1/networks > networks.json
```

## Sécurité
1. Toujours TLS en production
2. Activer OIDC pour authentification centralisée
3. Restreindre accès API (network policies / firewall)
4. Stocker secrets de façon sécurisée (K8s secrets, Vault)
5. Rotation régulière des tokens d'agent
6. Limiter ports WireGuard exposés
7. Surveiller incidents et résoudre rapidement

## Dépannage
### Serveur ne démarre pas
```bash
kubectl logs -n wirety -l app=wirety-server
```
Vérifier variables + configuration OIDC.

### Échec inscription agent
```bash
journalctl -u wirety-agent -f
```
Vérifier TOKEN & connectivité vers le serveur.

### Problèmes de connectivité
```bash
wg show wg0
ping <ip-peer>
ip route | grep wg0
```

Bon déploiement de Wirety !
