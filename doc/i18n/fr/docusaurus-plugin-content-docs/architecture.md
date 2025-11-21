---
id: architecture
title: Architecture
sidebar_position: 2
---

```mermaid
flowchart LR
  subgraph UserSpace[Utilisateurs]
    UI[Frontend (React)]
  end
  subgraph Backend[Backend]
    API[API REST + WebSocket]
    Auth[OIDC / Auth interne]
    Incidents[Détection incidents]
    ACLs[ACL par Réseau]
    IPAM[IPAM]
    WGKeys[Gestion clés WireGuard]
  end
  subgraph Mesh[Maillage WireGuard]
    J1[Peer Jump]
    J2[Peer Jump]
    R1[Peer Regular]
    R2[Peer Regular]
  end
  UI --> API
  API --> ACLs
  API --> IPAM
  API --> WGKeys
  ACLs --> Incidents
  Incidents --> API
  API <-->|WS: heartbeats + mises à jour| J1
  API <-->|WS| J2
  API <-->|WS| R1
  API <-->|WS| R2
  J1 <-->|WireGuard| J2
  J1 <-->|WireGuard| R1
  J2 <-->|WireGuard| R2
```

## Composants

### Server
Gère l'API REST, WebSocket, IPAM, ACL, génération de configs WireGuard, détection d'incidents et intégration OIDC optionnelle.

### Agent
S'exécute sur les hôtes dynamiques pour :
- S'enregistrer via token
- Envoyer heartbeats (endpoints, état session)
- Recevoir mises à jour de configuration (ACL, peers, clés)

### Frontend
Interface opérateur pour : réseaux, peers, incidents, tokens et statut des sessions.

### Helm Chart
Déploie server + frontend, configure l'ingress et paramètres principaux.

## Modèle de Peer
- **Jump:** Route/NAT pour les peers regular; pivot d'interconnexion.
- **Regular Dynamic:** Inscription automatisée; configuration poussée.
- **Regular Static:** Pas d'agent; configuration fournie à l'opérateur.

## Sécurité & Incidents
Wirety préfère l'isolement ACL au drop de connexion. Quand un incident est détecté, le peer est ajouté à `BlockedPeers`. Résolution = retrait et push de config.

### Types d'incidents
- Conflit de session (duplicat heartbeat)
- Saut de comportement (pattern endpoint inattendu)
- Activité suspecte personnalisable (futur)

## Flux de configuration
1. Agent heartbeat → Server actualise état.
2. Détection incident → marquage ACL.
3. Mise à jour push via WebSocket → agent applique.

## IPAM
Propose des CIDR disponibles selon capacité (`max_peers`). Assigne IP aux peers dynamiques; statiques fournissent leur pubkey et reçoivent IP/config.

## OIDC (optionnel)
Active authentification fédérée; remplace login local.

## Résilience
- Heartbeats réguliers maintiennent vue cohérente.
- ACL permet confinement sans perte de traçabilité.
- Separation Jump/Regular réduit la surface exposée.

## Étapes suivantes
Consulter [network](./network.md), [peers](./peers.md), et [incidents](./incidents.md) pour les détails opérationnels.
