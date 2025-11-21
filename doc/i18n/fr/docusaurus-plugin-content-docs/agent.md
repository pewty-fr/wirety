---
id: agent
title: Agent
sidebar_position: 7
---

L'agent Wirety automatise la participation au maillage WireGuard pour les peers dynamiques.

## Fonctionnalités
- Inscription via token
- Heartbeats (endpoints, état session)
- Application des mises à jour config (ACL, peers)

## Installation
```bash
curl -fsSL https://wirety.example.com/install-agent.sh | bash -s -- --token <TOKEN>
```

## Tokens
Générés dans l'UI (sécurité future: expiration, scope).

## Logs
Surveillez heartbeats et incidents pour réaction rapide.
