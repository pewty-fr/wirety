---
id: server
title: Serveur
sidebar_position: 8
---

Le serveur central orchestre Wirety: API REST, WebSocket, IPAM, ACL, incidents, OIDC.

## Démarrage
Variables d'env (exemple) :
```bash
AUTH_ENABLED=true \
AUTH_ISSUER_URL=https://identity.example.com/dex \
AUTH_CLIENT_ID=wirety \
AUTH_CLIENT_SECRET=secret \
go run cmd/main.go
```

## Endpoints principaux
- `/api/v1/networks`
- `/api/v1/ipam`
- `/api/v1/security/incidents`
- `/api/v1/agent/resolve`

## WebSocket
Canal pour heartbeats + push config.

## Sécurité
Blocage ACL plutôt que suppression brute des tunnels.
