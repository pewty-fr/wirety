---
id: deployment
title: Guide de Déploiement
sidebar_position: 6
---

Ce guide couvre le déploiement de Wirety dans des environnements de production.

## Options de déploiement

Wirety peut être déployé via :
- **Kubernetes avec Helm** (recommandé pour la production)
- **Docker Compose** (adapté aux tests et petits déploiements)
- **Déploiement binaire** (utilisateurs avancés)

## Déploiement Kubernetes avec Helm

### Prérequis
- Cluster Kubernetes (1.24+)
- Helm 3.x
- Contrôleur Ingress (nginx, traefik, etc.)
- Classe de stockage pour les volumes persistants (optionnel)

### Installation

1. **Ajouter le dépôt Helm** (bientôt disponible)
```bash
# Le chart Helm sera publié dans un dépôt
# Pour l'instant, utiliser le chart local
cd helm
```

2. **Créer un fichier de valeurs**
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
    AUTH_CLIENT_SECRET: "votre-secret"
    AUTH_JWKS_CACHE_TTL: "3600"
    LOG_LEVEL: "info"    # trace|debug|info|warn|error|fatal
    LOG_FORMAT: "json"   # json recommandé pour les agrégateurs de logs (Loki, Datadog, etc.)

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
  enabled: false  # Utiliser une base de données externe à la place
  # storageClass: "standard"
  # size: 10Gi
```

3. **Installer le chart**
```bash
helm install wirety ./helm -f values-production.yaml --namespace wirety --create-namespace
```

4. **Vérifier le déploiement**
```bash
kubectl get pods -n wirety
kubectl get ingress -n wirety
kubectl logs -n wirety -l app=wirety-server
```

### Mise à jour

```bash
helm upgrade wirety ./helm -f values-production.yaml --namespace wirety
```

### Désinstallation

```bash
helm uninstall wirety --namespace wirety
```

## Déploiement Docker Compose

Adapté aux tests et déploiements à petite échelle.

### Configuration

1. **Créer docker-compose.yml**
```yaml
version: '3.8'

services:
  server:
    image: ghcr.io/pewty-fr/wirety/server:latest
    ports:
      - "8080:8080"
    environment:
      HTTP_PORT: "8080"
      AUTH_ENABLED: "false"
      LOG_LEVEL: "info"     # trace|debug|info|warn|error|fatal
      LOG_FORMAT: "json"    # json est recommandé quand les logs sont ingérés par un collecteur
    volumes:
      - ./data:/data
    restart: unless-stopped

  frontend:
    image: ghcr.io/pewty-fr/wirety/front:latest
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

2. **Démarrer les services**
```bash
docker-compose up -d
```

3. **Vérifier les logs**
```bash
docker-compose logs -f
```

## Déploiement binaire

### Serveur

1. **Télécharger le binaire**
```bash
curl -fsSL https://github.com/pewty-fr/wirety/releases/latest/download/wirety-server-linux-amd64 -o /usr/local/bin/wirety-server
chmod +x /usr/local/bin/wirety-server
```

2. **Créer le service systemd**
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

3. **Activer et démarrer**
```bash
systemctl daemon-reload
systemctl enable wirety-server
systemctl start wirety-server
```

### Frontend

Servir le frontend compilé avec nginx ou tout autre serveur web :

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

## Déploiement de l'agent

### Installation manuelle

```bash
# Détecter l'architecture
ARCH=$(uname -m)
case $ARCH in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
esac

# Télécharger le binaire de l'agent
curl -fsSL "https://github.com/pewty-fr/wirety/releases/latest/download/wirety-agent-linux-${ARCH}" \
  -o /usr/local/bin/wirety-agent
chmod +x /usr/local/bin/wirety-agent
```

### Service Systemd

Le fichier unit suivant fournit une configuration prête pour la production avec durcissement de sécurité et redémarrage automatique.

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

# Journalisation
StandardOutput=append:/var/log/wirety/wirety.log
StandardError=append:/var/log/wirety/wirety.log

# Répertoires d'exécution (créés par systemd avant le démarrage)
RuntimeDirectory=wireguard
RuntimeDirectoryMode=0755
LogsDirectory=wirety
LogsDirectoryMode=0750

# Durcissement de sécurité
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

# Limites de ressources
LimitNOFILE=65536
LimitNPROC=512

[Install]
WantedBy=multi-user.target
```

Pour les **configurations reverse-proxy** où le serveur est accédé par IP (sans DNS), ajoutez `--server-host` et optionnellement `--skip-tls-verify` si le proxy utilise un certificat auto-signé :

```ini
ExecStart=/usr/local/bin/wirety-agent \
  --token <ENROLLMENT_TOKEN> \
  --server https://10.0.0.1 \
  --server-host wirety.internal \
  --portal-url https://wirety.internal/captive-portal \
  --skip-tls-verify
```

Activer et démarrer le service :

```bash
systemctl daemon-reload
systemctl enable --now wirety
```

Voir les logs :

```bash
journalctl -u wirety -f
```

### Cloud-Init (Provisionnement automatisé de jump peer)

La configuration cloud-init suivante provisionne entièrement une VM jump peer depuis zéro — modules kernel, tuning sysctl, durcissement de sécurité, mises à jour automatiques et installation de l'agent Wirety. Substituez les variables `${token}`, `${server}` et `${host}` avant utilisation.

```yaml
#cloud-config
package_update: true
package_upgrade: true
package_reboot_if_required: true

timezone: Europe/Paris
locale: en_US.UTF-8

packages:
  # Utilitaires de base
  - curl
  - wget
  - ca-certificates
  - git
  - jq
  # WireGuard
  - wireguard
  - wireguard-tools
  # Diagnostics réseau
  - net-tools
  - bind9-dnsutils
  - iputils-ping
  - traceroute
  - tcpdump
  # Surveillance système
  - htop
  - lsof
  # Gestion des logs
  - logrotate
  # Sécurité
  - fail2ban
  - unattended-upgrades
  # Synchronisation horaire
  - chrony

write_files:
  # Modules kernel requis par les règles iptables de Wirety
  - path: /etc/modules-load.d/wirety.conf
    content: |
      # Requis pour le suivi de connexion (règles ESTABLISHED/RELATED)
      nf_conntrack
      # Couche de compatibilité xtables — permet à xt_string de fonctionner avec iptables-nft
      nft_compat
      # Requis pour la correspondance de chaîne iptables (isolation vhost SNI / en-tête Host)
      xt_string

  # Tuning sysctl : forwarding IP, durcissement de sécurité, performance TCP
  - path: /etc/sysctl.d/99-wirety.conf
    content: |
      # Forwarding IP pour WireGuard VPN
      net.ipv4.ip_forward = 1
      net.ipv6.conf.all.forwarding = 1

      # Filtrage de chemin inverse
      net.ipv4.conf.all.rp_filter = 1
      net.ipv4.conf.default.rp_filter = 1

      # Désactiver les redirections ICMP
      net.ipv4.conf.all.accept_redirects = 0
      net.ipv4.conf.default.accept_redirects = 0
      net.ipv4.conf.all.send_redirects = 0
      net.ipv6.conf.all.accept_redirects = 0

      # Ignorer les pings broadcast
      net.ipv4.icmp_echo_ignore_broadcasts = 1

      # Tuning TCP
      net.core.somaxconn = 65535
      net.ipv4.tcp_rmem = 4096 87380 16777216
      net.ipv4.tcp_wmem = 4096 65536 16777216

      # Augmenter la limite de descripteurs de fichiers
      fs.file-max = 1000000

  - path: /etc/security/limits.d/99-wirety.conf
    content: |
      * soft nofile 65536
      * hard nofile 65536
      root soft nofile 65536
      root hard nofile 65536

  # Rotation des logs Wirety
  - path: /etc/logrotate.d/wirety
    content: |
      /var/log/wirety/*.log {
        daily
        rotate 14
        compress
        delaycompress
        missingok
        notifempty
        copytruncate        # ← copy then truncate in place, no fd invalidation
        create 0640 root root
        sharedscripts
      }

  # fail2ban : protéger SSH
  - path: /etc/fail2ban/jail.local
    content: |
      [DEFAULT]
      bantime  = 1h
      findtime = 10m
      maxretry = 5

      [sshd]
      enabled = true

  # Mises à jour de sécurité automatiques
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

  # Unité systemd Wirety
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

  # Script d'installation : télécharge le binaire agent et configure le système
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
          echo "Architecture non supportée : $ARCH" >&2
          exit 1
          ;;
      esac

      echo "Téléchargement de wirety-agent v${WIRETY_VERSION} pour ${ARCH}..."
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
  - echo "Installation Wirety terminée le $(date -u)" >> /var/log/cloud-init-output.log
```

:::caution `--skip-tls-verify`
Le flag `--skip-tls-verify` désactive la validation du certificat TLS pour la connexion entre l'agent et le serveur Wirety. Ne l'utiliser que si le serveur est derrière un reverse proxy avec un certificat auto-signé que l'agent ne peut pas vérifier (ex. CA interne). Ne jamais l'utiliser dans des environnements de production avec des certificats signés publiquement.
:::

### Playbook Ansible

```yaml
# playbooks/wirety-agent.yml
---
- name: Déployer l'agent Wirety
  hosts: wirety_peers
  become: yes

  vars:
    wirety_server_url: "https://wirety.example.com"
    wirety_token: "{{ lookup('env', 'WIRETY_TOKEN') }}"

  tasks:
    - name: Installer WireGuard
      apt:
        name: wireguard
        state: present

    - name: Télécharger l'agent Wirety
      get_url:
        url: "https://github.com/pewty-fr/wirety/releases/latest/download/wirety-agent-linux-{{ 'arm64' if ansible_architecture == 'aarch64' else 'amd64' }}"
        dest: /usr/local/bin/wirety-agent
        mode: '0755'

    - name: Créer le service systemd
      template:
        src: wirety.service.j2
        dest: /etc/systemd/system/wirety.service

    - name: Activer et démarrer l'agent
      systemd:
        name: wirety
        enabled: yes
        state: started
        daemon_reload: yes
```

## Configuration de l'environnement

### Variables d'environnement du serveur

| Variable | Description | Défaut | Obligatoire |
|----------|-------------|--------|-------------|
| HTTP_PORT | Port du serveur | `8080` | Non |
| AUTH_ENABLED | Activer l'auth OIDC | `false` | Non |
| AUTH_ISSUER_URL | URL du fournisseur OIDC | — | Si auth activée |
| AUTH_CLIENT_ID | ID client OIDC | — | Si auth activée |
| AUTH_CLIENT_SECRET | Secret client OIDC | — | Si auth activée |
| AUTH_JWKS_CACHE_TTL | Durée du cache JWKS (secondes) | `3600` | Non |
| COOKIE_SECURE | Active l'attribut `Secure` sur le cookie de session — mettre à `false` uniquement en HTTP local | `true` | Non |
| LOG_LEVEL | Verbosité des logs : `trace`\|`debug`\|`info`\|`warn`\|`error`\|`fatal` | `info` | Non |
| LOG_FORMAT | Format de sortie des logs : `text`\|`json` | `text` | Non |
| AUDIT_LOG | Émettre des événements d'audit JSON sur stdout | `false` | Non |

### Variables d'environnement de l'agent

| Variable | Description | Défaut | Obligatoire |
|----------|-------------|--------|-------------|
| SERVER_URL | URL du serveur Wirety | — | Oui |
| TOKEN | Token d'inscription | — | Oui |
| WG_CONFIG_PATH | Chemin du fichier config WireGuard | — | Non |
| WG_APPLY_METHOD | Méthode d'application : `wg-quick`\|`syncconf` | `syncconf` | Non |
| NAT_INTERFACES | Interfaces NAT séparées par des virgules | auto-détect | Non |
| CAPTIVE_PORTAL_URL | URL de la page du portail captif | `<SERVER_URL>/captive-portal` | Non |
| SERVER_HOST | Remplacer l'en-tête HTTP Host (configurations reverse-proxy) | — | Non |
| SKIP_TLS_VERIFY | Désactiver la vérification TLS | `false` | Non |
| LOG_LEVEL | Verbosité des logs : `trace`\|`debug`\|`info`\|`warn`\|`error`\|`fatal` | `info` | Non |
| LOG_FORMAT | Format de sortie des logs : `text`\|`json` | `text` | Non |
| AUDIT_LOG | Émettre des événements d'audit JSON sur stdout | `false` | Non |

## Surveillance

### Vérifications d'état

**Endpoint de santé du serveur :**
```bash
curl http://localhost:8080/health
```

**État de l'agent :**
```bash
systemctl status wirety-agent
wg show wg0
```

### Métriques (futur)

Les métriques Prometheus seront exposées à `/metrics` :
- Nombre de peers
- Connexions actives
- Utilisation IPAM

## Sauvegarde et récupération

### Données du serveur

Si vous utilisez le stockage en mémoire (défaut), les données sont perdues au redémarrage. Pour la production :
- Activer PostgreSQL avec `DB_ENABLED=true` et `DB_DSN`
- Sauvegardes régulières des configurations de peers
- Export des configurations réseau

### Sauvegarde de configuration

```bash
# Exporter les configs des peers
curl -H "Authorization: Bearer $TOKEN" \
  https://wirety.example.com/api/v1/peers > peers-backup.json

# Exporter les réseaux
curl -H "Authorization: Bearer $TOKEN" \
  https://wirety.example.com/api/v1/networks > networks-backup.json
```

## Considérations de sécurité

1. **TLS/HTTPS** : Toujours utiliser HTTPS en production
2. **Authentification** : Activer l'authentification OIDC
3. **Politiques réseau** : Restreindre l'accès à l'API du serveur
4. **Gestion des secrets** : Utiliser les secrets Kubernetes ou Vault
5. **Rotation des tokens** : Faire pivoter régulièrement les tokens d'inscription
6. **Règles de pare-feu** : Restreindre les ports WireGuard exposés

## Dépannage

### Le serveur ne démarre pas
```bash
kubectl logs -n wirety -l app=wirety-server
# Vérifier les variables d'environnement
# Vérifier la configuration OIDC
```

### L'inscription de l'agent échoue
```bash
journalctl -u wirety-agent -f
# Vérifier la validité du token
# Vérifier l'accessibilité de l'URL du serveur
# S'assurer que WireGuard est installé
```

### Problèmes de connectivité
```bash
# Vérifier l'état WireGuard
wg show wg0

# Tester la connectivité des peers
ping <peer-ip>

# Vérifier les IP autorisées
wg show wg0 allowed-ips

# Vérifier le routage
ip route | grep wg0
```

## Étapes suivantes

- [Configuration OIDC](/guides/oidc.md)
- [Configuration réseau](/network.md)
