---
id: groups-policies-routes-overview
title: Vue d'ensemble Groupes, Politiques & Routes
sidebar_position: 5
---

Ce document présente une vue d'ensemble de l'architecture des groupes, politiques et routes dans le système de gestion de réseaux WireGuard.

## Liens rapides

- **[Référence API](./api-reference)** — Documentation complète de l'API pour tous les endpoints
- **[Guide de migration](./migration-guide)** — Migrer depuis le système ACL et de flags de peers hérité
- **[Guide utilisateur](./user-guide)** — Guides étape par étape pour les tâches courantes
- **[Guide de gestion des groupes](./guides/groups-management)** — Gestion détaillée des groupes

## Nouveautés

Le système a été repensé avec une architecture moderne et flexible :

### Groupes
Organisez les peers en collections logiques. Les groupes servent de point d'attachement pour les politiques et les routes.

**Fonctionnalités clés :**
- Gestion réservée aux administrateurs
- Les peers peuvent appartenir à plusieurs groupes
- Opérations non destructives (supprimer un groupe ne supprime pas les peers)

**En savoir plus :** [Guide de gestion des groupes](./guides/groups-management)

### Politiques
Définissent les règles iptables appliquées sur les jump peers pour le filtrage du trafic. Les politiques sont attachées aux groupes.

**Fonctionnalités clés :**
- Support des directions input/output
- Support des actions allow/deny
- Ciblage par CIDR, peer ou groupe
- Templates intégrés pour les patterns courants

**En savoir plus :** [Guide utilisateur - Politiques](./user-guide#policies-management)

### Routes
Définissent les destinations réseau externes accessibles via les jump peers. Les routes sont ajoutées aux AllowedIPs WireGuard.

**Fonctionnalités clés :**
- Spécifier le CIDR de destination et le jump peer
- Attacher aux groupes pour une configuration automatique
- Support de suffixes de domaine DNS personnalisés

**En savoir plus :** [Guide utilisateur - Routes](./user-guide#routes-management)

### Correspondances DNS
Fournissent la résolution de noms pour les adresses IP au sein des réseaux de routes.

**Fonctionnalités clés :**
- Associer des noms aux IPs dans les CIDR de routes
- Format FQDN : `nom.route.domaine`
- Propagation vers les serveurs DNS des jump peers

**En savoir plus :** [Guide utilisateur - DNS](./user-guide#dns-mappings)

### Groupes par défaut
Assignent automatiquement des groupes aux peers créés par des utilisateurs non administrateurs.

**Fonctionnalités clés :**
- Configuration réservée aux administrateurs
- Appliqués uniquement aux peers créés par des non-admins
- Utile pour les politiques de base

**En savoir plus :** [Guide utilisateur - Groupes par défaut](./user-guide#default-groups-configuration)

## Premiers pas

1. Lire l'[introduction du guide utilisateur](./user-guide)
2. Suivre les [workflows courants](./user-guide#common-workflows)
3. Explorer la [référence API](./api-reference)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Réseau                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                       Groupes                           │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │ │
│  │  │  Groupe 1    │  │  Groupe 2    │  │  Groupe 3    │ │ │
│  │  │              │  │              │  │              │ │ │
│  │  │ Peers: 3     │  │ Peers: 5     │  │ Peers: 2     │ │ │
│  │  │ Politiques:2 │  │ Politiques:1 │  │ Politiques:3 │ │ │
│  │  │ Routes: 1    │  │ Routes: 2    │  │ Routes: 0    │ │ │
│  │  └──────────────┘  └──────────────┘  └──────────────┘ │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                     Politiques                          │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │ │
│  │  │ Politique 1  │  │ Politique 2  │  │ Politique 3  │ │ │
│  │  │              │  │              │  │              │ │ │
│  │  │ Règles: 3    │  │ Règles: 2    │  │ Règles: 5    │ │ │
│  │  │ Groupes: 2   │  │ Groupes: 1   │  │ Groupes: 1   │ │ │
│  │  └──────────────┘  └──────────────┘  └──────────────┘ │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                       Routes                            │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │ │
│  │  │   Route 1    │  │   Route 2    │  │   Route 3    │ │ │
│  │  │              │  │              │  │              │ │ │
│  │  │ CIDR: /16    │  │ CIDR: /24    │  │ CIDR: /8     │ │ │
│  │  │ Jump: peer-1 │  │ Jump: peer-1 │  │ Jump: peer-2 │ │ │
│  │  │ DNS: 3       │  │ DNS: 1       │  │ DNS: 0       │ │ │
│  │  │ Groupes: 2   │  │ Groupes: 1   │  │ Groupes: 1   │ │ │
│  │  └──────────────┘  └──────────────┘  └──────────────┘ │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Cas d'utilisation courants

### 1. Organiser par département
Créer des groupes pour chaque département et appliquer les politiques et routes appropriées.

**Exemple :** Groupes Ingénierie, Commercial, Support avec différents niveaux d'accès.

**Guide :** [Guide utilisateur - Workflow 1](./user-guide#workflow-1-setting-up-a-new-team)

### 2. Sécurité d'application multi-niveaux
Configurer des niveaux web, app et base de données avec des communications contrôlées.

**Exemple :** Le niveau web peut accéder au niveau app, le niveau app peut accéder au niveau base de données.

**Guide :** [Guide utilisateur - Workflow 4](./user-guide#workflow-4-multi-tier-application-security)

### 3. Accès aux ressources cloud
Fournir un accès aux réseaux cloud (AWS, Azure, GCP) via les jump peers.

**Exemple :** Route vers un VPC AWS avec résolution DNS pour les services internes.

**Guide :** [Guide utilisateur - Workflow 3](./user-guide#workflow-3-providing-access-to-cloud-resources)

### 4. Réponse aux incidents
Isoler rapidement les peers compromis en les déplaçant vers un groupe de quarantaine.

**Exemple :** Déplacer un peer suspect vers un groupe isolé avec une politique de tout-refuser.

**Guide :** [Guide utilisateur - Workflow 2](./user-guide#workflow-2-isolating-a-compromised-peer)

### 5. Accès utilisateur par défaut
Configurer un accès de base pour tous les peers créés par des non-admins.

**Exemple :** Tous les utilisateurs obtiennent par défaut un accès au réseau bureau et à internet.

**Guide :** [Guide utilisateur - Groupes par défaut](./user-guide#default-groups-configuration)

## Vue d'ensemble de l'API

### Endpoints Groupes
```
POST   /api/v1/networks/:networkId/groups
GET    /api/v1/networks/:networkId/groups
GET    /api/v1/networks/:networkId/groups/:groupId
PUT    /api/v1/networks/:networkId/groups/:groupId
DELETE /api/v1/networks/:networkId/groups/:groupId
POST   /api/v1/networks/:networkId/groups/:groupId/peers/:peerId
DELETE /api/v1/networks/:networkId/groups/:groupId/peers/:peerId
```

### Endpoints Politiques
```
POST   /api/v1/networks/:networkId/policies
GET    /api/v1/networks/:networkId/policies
GET    /api/v1/networks/:networkId/policies/:policyId
PUT    /api/v1/networks/:networkId/policies/:policyId
DELETE /api/v1/networks/:networkId/policies/:policyId
POST   /api/v1/networks/:networkId/policies/:policyId/rules
DELETE /api/v1/networks/:networkId/policies/:policyId/rules/:ruleId
GET    /api/v1/networks/:networkId/policies/templates
POST   /api/v1/networks/:networkId/groups/:groupId/policies/:policyId
DELETE /api/v1/networks/:networkId/groups/:groupId/policies/:policyId
GET    /api/v1/networks/:networkId/groups/:groupId/policies
```

### Endpoints Routes
```
POST   /api/v1/networks/:networkId/routes
GET    /api/v1/networks/:networkId/routes
GET    /api/v1/networks/:networkId/routes/:routeId
PUT    /api/v1/networks/:networkId/routes/:routeId
DELETE /api/v1/networks/:networkId/routes/:routeId
POST   /api/v1/networks/:networkId/groups/:groupId/routes/:routeId
DELETE /api/v1/networks/:networkId/groups/:groupId/routes/:routeId
GET    /api/v1/networks/:networkId/groups/:groupId/routes
```

### Endpoints DNS
```
POST   /api/v1/networks/:networkId/routes/:routeId/dns
GET    /api/v1/networks/:networkId/routes/:routeId/dns
PUT    /api/v1/networks/:networkId/routes/:routeId/dns/:dnsId
DELETE /api/v1/networks/:networkId/routes/:routeId/dns/:dnsId
GET    /api/v1/networks/:networkId/dns
```

**Détails complets :** [Référence API](./api-reference)

## Considérations de sécurité

### Opérations réservées aux administrateurs
Toutes les opérations sur les groupes, politiques, routes et DNS nécessitent des privilèges administrateur.

### Autorisation
- Les utilisateurs non-admin reçoivent HTTP 403 pour les opérations admin
- L'autorisation est vérifiée aux niveaux API et service

### Journalisation d'audit
Toutes les opérations administratives sont journalisées pour l'audit de sécurité.

### Refus par défaut
Le trafic est refusé par défaut sauf s'il est explicitement autorisé par des politiques.

### Principe du moindre privilège
Commencer avec un accès minimal et ajouter des permissions selon les besoins.

## Considérations de performance

### Scalabilité
- Groupes : Des milliers par réseau
- Politiques : Des centaines par réseau
- Routes : Des centaines par réseau
- Peers par groupe : Des milliers

### Mises à jour de configuration
- Changements de politiques : Appliqués en quelques secondes via WebSocket
- Changements de routes : Configurations WireGuard régénérées automatiquement
- Changements DNS : Propagés vers les jump peers en moins de 60 secondes

### Optimisation de la base de données
- Clés étrangères indexées pour des recherches rapides
- Pooling de connexions pour les requêtes concurrentes
- Opérations en lot pour les changements en masse

## Dépannage

### Diagnostics rapides

**Vérifier l'appartenance à un groupe :**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID"
```

**Vérifier les politiques attachées :**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies"
```

**Vérifier les routes attachées :**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes"
```

**Vérifier iptables sur le jump peer :**
```bash
iptables -L -n -v
```

**Vérifier la configuration WireGuard :**
```bash
wg show
```

**Guide complet :** Voir la page Dépannage dans la barre latérale.

## Support

### Documentation
- [Référence API](./api-reference)
- [Guide utilisateur](./user-guide)
- Dépannage (voir barre latérale)

### Obtenir de l'aide
1. Consulter la documentation et les guides de dépannage
2. Examiner les logs serveur : `docker logs wireguard-server`
3. Vérifier les logs agent : `journalctl -u wireguard-agent`
4. Ouvrir une issue GitHub avec les détails
