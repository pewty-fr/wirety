---
id: peers
title: Peers
sidebar_position: 4
---

Les peers forment le maillage WireGuard. Catégories : Jump, Regular Dynamiques, Regular Statiques.

## Jump
Route/NAT le trafic des peers regular. Reçoit plus de connexions entrantes. Doit être dimensionné (CPU, bande passante).

## Regular Dynamique
Inscription par agent + token. Reçoit mises à jour automatiques (ACL, nouvelles clés des voisins, endpoints).

## Regular Statique
Pas d'agent. L'UI fournit configuration (pubkey, endpoint, peers autorisés). L'opérateur l'applique sur l'hôte.

## Inscription dynamique
```bash
curl -fsSL https://wirety.example.com/install-agent.sh | bash -s -- --token <TOKEN>
```

## Rotation de clé
Wirety peut pousser rotation (futur). Agents appliquent sans interruption majeure.

## Sessions
Heartbeats contiennent endpoints et statut; conflits → incident.

## Bonnes pratiques
- Limiter nombre de jumps.
- Utiliser dynamiques pour serveurs volatils (cloud auto-scale).
- Statiques pour endpoints utilisateur.

## Incidents
Voir [Incidents](./incidents.md) pour blocage via ACL.
