# Référence API

Ce document fournit des informations détaillées sur les endpoints de l'API de gestion de réseaux WireGuard.

## Authentification

Tous les endpoints API nécessitent une authentification via un token Bearer dans l'en-tête `Authorization`. Deux types de tokens sont acceptés :

- **Token de session** (JWT OIDC ou session auth simple) : obtenu depuis le flux de connexion.
- **Token API** (préfixe `wirety_`) : token d'accès personnel à longue durée de vie créé via `/users/me/tokens`.

```
Authorization: Bearer <token>
```

Les deux types de tokens portent les mêmes permissions que l'utilisateur qui les possède.

## Endpoints réservés aux administrateurs

Les endpoints marqués avec 🔒 nécessitent des privilèges administrateur. Les utilisateurs non administrateurs recevront HTTP 403 Forbidden.

## API Groupes

Les groupes permettent aux administrateurs d'organiser les peers en collections logiques pour l'application de politiques et de routes.

### Créer un groupe 🔒

Créer un nouveau groupe dans un réseau.

**Endpoint :** `POST /api/v1/networks/:networkId/groups`

**Corps de la requête :**
```json
{
  "name": "engineering-team",
  "description": "Membres de l'équipe ingénierie"
}
```

**Réponse :** `201 Created`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "network_id": "660e8400-e29b-41d4-a716-446655440000",
  "name": "engineering-team",
  "description": "Membres de l'équipe ingénierie",
  "peer_ids": [],
  "policy_ids": [],
  "route_ids": [],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

**Réponses d'erreur :**
- `400 Bad Request` - Corps de requête invalide ou nom de groupe en doublon
- `403 Forbidden` - Utilisateur non administrateur
- `404 Not Found` - Réseau introuvable

---

### Lister les groupes 🔒

Lister tous les groupes d'un réseau.

**Endpoint :** `GET /api/v1/networks/:networkId/groups`

**Réponse :** `200 OK`
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "network_id": "660e8400-e29b-41d4-a716-446655440000",
    "name": "engineering-team",
    "description": "Membres de l'équipe ingénierie",
    "peer_ids": ["peer-1", "peer-2"],
    "policy_ids": ["policy-1"],
    "route_ids": ["route-1"],
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

---

### Obtenir un groupe 🔒

Obtenir les détails d'un groupe spécifique.

**Endpoint :** `GET /api/v1/networks/:networkId/groups/:groupId`

**Réponse :** `200 OK`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "network_id": "660e8400-e29b-41d4-a716-446655440000",
  "name": "engineering-team",
  "description": "Membres de l'équipe ingénierie",
  "peer_ids": ["peer-1", "peer-2"],
  "policy_ids": ["policy-1"],
  "route_ids": ["route-1"],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

**Réponses d'erreur :**
- `403 Forbidden` - Utilisateur non administrateur
- `404 Not Found` - Groupe ou réseau introuvable

---

### Mettre à jour un groupe 🔒

Mettre à jour le nom ou la description d'un groupe.

**Endpoint :** `PUT /api/v1/networks/:networkId/groups/:groupId`

**Corps de la requête :**
```json
{
  "name": "senior-engineers",
  "description": "Membres seniors de l'équipe ingénierie"
}
```

**Réponse :** `200 OK`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "network_id": "660e8400-e29b-41d4-a716-446655440000",
  "name": "senior-engineers",
  "description": "Membres seniors de l'équipe ingénierie",
  "peer_ids": ["peer-1", "peer-2"],
  "policy_ids": ["policy-1"],
  "route_ids": ["route-1"],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T11:00:00Z"
}
```

---

### Supprimer un groupe 🔒

Supprimer un groupe. Les peers du groupe ne sont pas supprimés.

**Endpoint :** `DELETE /api/v1/networks/:networkId/groups/:groupId`

**Réponse :** `204 No Content`

**Réponses d'erreur :**
- `403 Forbidden` - Utilisateur non administrateur
- `404 Not Found` - Groupe ou réseau introuvable

---

### Ajouter un peer à un groupe 🔒

Ajouter un peer à un groupe. Applique automatiquement les politiques et routes du groupe.

**Endpoint :** `POST /api/v1/networks/:networkId/groups/:groupId/peers/:peerId`

**Réponse :** `200 OK`

**Réponses d'erreur :**
- `403 Forbidden` - Utilisateur non administrateur
- `404 Not Found` - Groupe, peer ou réseau introuvable

---

### Retirer un peer d'un groupe 🔒

Retirer un peer d'un groupe. Supprime les politiques et routes du groupe du peer.

**Endpoint :** `DELETE /api/v1/networks/:networkId/groups/:groupId/peers/:peerId`

**Réponse :** `204 No Content`

**Réponses d'erreur :**
- `403 Forbidden` - Utilisateur non administrateur
- `404 Not Found` - Groupe, peer ou réseau introuvable

---

## API Politiques

Les politiques définissent les règles iptables appliquées sur les jump peers pour contrôler le filtrage du trafic.

### Créer une politique 🔒

Créer une nouvelle politique avec des règles optionnelles.

**Endpoint :** `POST /api/v1/networks/:networkId/policies`

**Corps de la requête :**
```json
{
  "name": "allow-web-traffic",
  "description": "Autoriser le trafic HTTP et HTTPS",
  "rules": [
    {
      "direction": "output",
      "action": "allow",
      "target": "0.0.0.0/0",
      "target_type": "cidr",
      "description": "Autoriser tout le trafic sortant"
    },
    {
      "direction": "input",
      "action": "deny",
      "target": "0.0.0.0/0",
      "target_type": "cidr",
      "description": "Refuser tout le trafic entrant"
    }
  ]
}
```

**Réponse :** `201 Created`
```json
{
  "id": "770e8400-e29b-41d4-a716-446655440000",
  "network_id": "660e8400-e29b-41d4-a716-446655440000",
  "name": "allow-web-traffic",
  "description": "Autoriser le trafic HTTP et HTTPS",
  "rules": [
    {
      "id": "rule-1",
      "direction": "output",
      "action": "allow",
      "target": "0.0.0.0/0",
      "target_type": "cidr",
      "description": "Autoriser tout le trafic sortant"
    },
    {
      "id": "rule-2",
      "direction": "input",
      "action": "deny",
      "target": "0.0.0.0/0",
      "target_type": "cidr",
      "description": "Refuser tout le trafic entrant"
    }
  ],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

**Champs des règles de politique :**
- `direction`: "input" ou "output"
- `action`: "allow" ou "deny"
- `target`: IP/CIDR, ID de peer ou ID de groupe
- `target_type`: "cidr", "peer" ou "group"

---

### Lister les politiques 🔒

Lister toutes les politiques d'un réseau.

**Endpoint :** `GET /api/v1/networks/:networkId/policies`

**Réponse :** `200 OK`
```json
[
  {
    "id": "770e8400-e29b-41d4-a716-446655440000",
    "network_id": "660e8400-e29b-41d4-a716-446655440000",
    "name": "allow-web-traffic",
    "description": "Autoriser le trafic HTTP et HTTPS",
    "rules": [...],
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

---

### Obtenir une politique 🔒

Obtenir les détails d'une politique spécifique incluant toutes les règles.

**Endpoint :** `GET /api/v1/networks/:networkId/policies/:policyId`

**Réponse :** `200 OK`
```json
{
  "id": "770e8400-e29b-41d4-a716-446655440000",
  "network_id": "660e8400-e29b-41d4-a716-446655440000",
  "name": "allow-web-traffic",
  "description": "Autoriser le trafic HTTP et HTTPS",
  "rules": [...],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

---

### Mettre à jour une politique 🔒

Mettre à jour le nom ou la description d'une politique. Déclenche la régénération d'iptables sur les jump peers affectés.

**Endpoint :** `PUT /api/v1/networks/:networkId/policies/:policyId`

**Corps de la requête :**
```json
{
  "name": "updated-policy-name",
  "description": "Description mise à jour"
}
```

**Réponse :** `200 OK`

---

### Supprimer une politique 🔒

Supprimer une politique. Retire de tous les groupes et met à jour les jump peers affectés.

**Endpoint :** `DELETE /api/v1/networks/:networkId/policies/:policyId`

**Réponse :** `204 No Content`

---

### Ajouter une règle à une politique 🔒

Ajouter une nouvelle règle à une politique.

**Endpoint :** `POST /api/v1/networks/:networkId/policies/:policyId/rules`

**Corps de la requête :**
```json
{
  "direction": "input",
  "action": "allow",
  "target": "10.0.0.0/24",
  "target_type": "cidr",
  "description": "Autoriser le trafic depuis le réseau interne"
}
```

**Réponse :** `201 Created`

---

### Retirer une règle d'une politique 🔒

Retirer une règle d'une politique.

**Endpoint :** `DELETE /api/v1/networks/:networkId/policies/:policyId/rules/:ruleId`

**Réponse :** `204 No Content`

---

### Obtenir les templates de politiques 🔒

Obtenir les templates de politiques prédéfinis.

**Endpoint :** `GET /api/v1/networks/:networkId/policies/templates`

**Réponse :** `200 OK`
```json
[
  {
    "name": "fully-encapsulated",
    "description": "Autoriser tout le trafic sortant, refuser tout le trafic entrant",
    "rules": [
      {
        "direction": "output",
        "action": "allow",
        "target": "0.0.0.0/0",
        "target_type": "cidr",
        "description": "Autoriser tout sortant"
      },
      {
        "direction": "input",
        "action": "deny",
        "target": "0.0.0.0/0",
        "target_type": "cidr",
        "description": "Refuser tout entrant"
      }
    ]
  },
  {
    "name": "isolated",
    "description": "Refuser tout le trafic entrant et sortant",
    "rules": [
      {
        "direction": "input",
        "action": "deny",
        "target": "0.0.0.0/0",
        "target_type": "cidr",
        "description": "Refuser tout entrant"
      },
      {
        "direction": "output",
        "action": "deny",
        "target": "0.0.0.0/0",
        "target_type": "cidr",
        "description": "Refuser tout sortant"
      }
    ]
  },
  {
    "name": "default-network",
    "description": "Autoriser tout le trafic dans le CIDR réseau",
    "rules": [
      {
        "direction": "input",
        "action": "allow",
        "target": "{network_cidr}",
        "target_type": "cidr",
        "description": "Autoriser entrant depuis le réseau"
      },
      {
        "direction": "output",
        "action": "allow",
        "target": "{network_cidr}",
        "target_type": "cidr",
        "description": "Autoriser sortant vers le réseau"
      }
    ]
  }
]
```

---

### Attacher une politique à un groupe 🔒

Attacher une politique à un groupe. Applique les règles iptables sur les jump peers pour tous les membres du groupe.

**Endpoint :** `POST /api/v1/networks/:networkId/groups/:groupId/policies/:policyId`

**Réponse :** `200 OK`

---

### Détacher une politique d'un groupe 🔒

Détacher une politique d'un groupe. Supprime les règles iptables des jump peers pour les membres du groupe.

**Endpoint :** `DELETE /api/v1/networks/:networkId/groups/:groupId/policies/:policyId`

**Réponse :** `204 No Content`

---

### Obtenir les politiques d'un groupe 🔒

Obtenir toutes les politiques attachées à un groupe.

**Endpoint :** `GET /api/v1/networks/:networkId/groups/:groupId/policies`

**Réponse :** `200 OK`
```json
[
  {
    "id": "770e8400-e29b-41d4-a716-446655440000",
    "network_id": "660e8400-e29b-41d4-a716-446655440000",
    "name": "allow-web-traffic",
    "description": "Autoriser le trafic HTTP et HTTPS",
    "rules": [...],
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

---

## API Routes

Les routes définissent les destinations réseau externes accessibles via les jump peers, ajoutées aux AllowedIPs WireGuard.

### Créer une route 🔒

Créer une nouvelle route vers un réseau externe.

**Endpoint :** `POST /api/v1/networks/:networkId/routes`

**Corps de la requête :**
```json
{
  "name": "aws-vpc",
  "description": "Réseau VPC AWS",
  "destination_cidr": "172.31.0.0/16",
  "jump_peer_id": "880e8400-e29b-41d4-a716-446655440000",
  "domain_suffix": "aws.internal"
}
```

**Réponse :** `201 Created`
```json
{
  "id": "990e8400-e29b-41d4-a716-446655440000",
  "network_id": "660e8400-e29b-41d4-a716-446655440000",
  "name": "aws-vpc",
  "description": "Réseau VPC AWS",
  "destination_cidr": "172.31.0.0/16",
  "jump_peer_id": "880e8400-e29b-41d4-a716-446655440000",
  "domain_suffix": "aws.internal",
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

**Réponses d'erreur :**
- `400 Bad Request` - Format CIDR invalide ou jump peer introuvable
- `403 Forbidden` - Utilisateur non administrateur
- `404 Not Found` - Réseau introuvable

---

### Lister les routes 🔒

Lister toutes les routes d'un réseau.

**Endpoint :** `GET /api/v1/networks/:networkId/routes`

**Réponse :** `200 OK`
```json
[
  {
    "id": "990e8400-e29b-41d4-a716-446655440000",
    "network_id": "660e8400-e29b-41d4-a716-446655440000",
    "name": "aws-vpc",
    "description": "Réseau VPC AWS",
    "destination_cidr": "172.31.0.0/16",
    "jump_peer_id": "880e8400-e29b-41d4-a716-446655440000",
    "domain_suffix": "aws.internal",
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

---

### Obtenir une route 🔒

Obtenir les détails d'une route spécifique.

**Endpoint :** `GET /api/v1/networks/:networkId/routes/:routeId`

**Réponse :** `200 OK`

---

### Mettre à jour une route 🔒

Mettre à jour une route. Déclenche la régénération de la configuration WireGuard pour les peers affectés.

**Endpoint :** `PUT /api/v1/networks/:networkId/routes/:routeId`

**Corps de la requête :**
```json
{
  "name": "aws-vpc-updated",
  "description": "Réseau VPC AWS mis à jour",
  "destination_cidr": "172.31.0.0/16",
  "jump_peer_id": "880e8400-e29b-41d4-a716-446655440000",
  "domain_suffix": "aws.internal"
}
```

**Réponse :** `200 OK`

---

### Supprimer une route 🔒

Supprimer une route. Retire de tous les groupes et met à jour les configurations des peers affectés.

**Endpoint :** `DELETE /api/v1/networks/:networkId/routes/:routeId`

**Réponse :** `204 No Content`

---

### Attacher une route à un groupe 🔒

Attacher une route à un groupe. Ajoute le CIDR de la route aux AllowedIPs pour tous les membres du groupe.

**Endpoint :** `POST /api/v1/networks/:networkId/groups/:groupId/routes/:routeId`

**Réponse :** `200 OK`

---

### Détacher une route d'un groupe 🔒

Détacher une route d'un groupe. Supprime le CIDR de la route des AllowedIPs des membres du groupe.

**Endpoint :** `DELETE /api/v1/networks/:networkId/groups/:groupId/routes/:routeId`

**Réponse :** `204 No Content`

---

### Obtenir les routes d'un groupe 🔒

Obtenir toutes les routes attachées à un groupe.

**Endpoint :** `GET /api/v1/networks/:networkId/groups/:groupId/routes`

**Réponse :** `200 OK`
```json
[
  {
    "id": "990e8400-e29b-41d4-a716-446655440000",
    "network_id": "660e8400-e29b-41d4-a716-446655440000",
    "name": "aws-vpc",
    "description": "Réseau VPC AWS",
    "destination_cidr": "172.31.0.0/16",
    "jump_peer_id": "880e8400-e29b-41d4-a716-446655440000",
    "domain_suffix": "aws.internal",
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

---

## API Correspondances DNS

Les correspondances DNS fournissent la résolution de noms pour les adresses IP au sein des réseaux de routes.

### Créer une correspondance DNS 🔒

Créer une correspondance DNS pour une route.

**Endpoint :** `POST /api/v1/networks/:networkId/routes/:routeId/dns`

**Corps de la requête :**
```json
{
  "name": "database-server",
  "ip_address": "172.31.10.50"
}
```

**Réponse :** `201 Created`
```json
{
  "id": "aa0e8400-e29b-41d4-a716-446655440000",
  "route_id": "990e8400-e29b-41d4-a716-446655440000",
  "name": "database-server",
  "ip_address": "172.31.10.50",
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

**Format FQDN :** `database-server.aws-vpc.aws.internal`

**Réponses d'erreur :**
- `400 Bad Request` - L'adresse IP n'est pas dans le CIDR de la route
- `403 Forbidden` - Utilisateur non administrateur
- `404 Not Found` - Route introuvable

---

### Lister les correspondances DNS 🔒

Lister toutes les correspondances DNS d'une route.

**Endpoint :** `GET /api/v1/networks/:networkId/routes/:routeId/dns`

**Réponse :** `200 OK`
```json
[
  {
    "id": "aa0e8400-e29b-41d4-a716-446655440000",
    "route_id": "990e8400-e29b-41d4-a716-446655440000",
    "name": "database-server",
    "ip_address": "172.31.10.50",
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

---

### Mettre à jour une correspondance DNS 🔒

Mettre à jour une correspondance DNS. Se propage aux serveurs DNS des jump peers dans les 60 secondes.

**Endpoint :** `PUT /api/v1/networks/:networkId/routes/:routeId/dns/:dnsId`

**Corps de la requête :**
```json
{
  "name": "db-primary",
  "ip_address": "172.31.10.51"
}
```

**Réponse :** `200 OK`

---

### Supprimer une correspondance DNS 🔒

Supprimer une correspondance DNS. Retire des serveurs DNS des jump peers dans les 60 secondes.

**Endpoint :** `DELETE /api/v1/networks/:networkId/routes/:routeId/dns/:dnsId`

**Réponse :** `204 No Content`

---

### Obtenir les enregistrements DNS d'un réseau 🔒

Obtenir tous les enregistrements DNS d'un réseau (enregistrements de peers + enregistrements de routes).

**Endpoint :** `GET /api/v1/networks/:networkId/dns`

**Réponse :** `200 OK`
```json
{
  "peer_records": [
    {
      "name": "peer1.mynetwork.internal",
      "ip_address": "10.0.0.2"
    }
  ],
  "route_records": [
    {
      "name": "database-server.aws-vpc.aws.internal",
      "ip_address": "172.31.10.50"
    }
  ]
}
```

---

## Mises à jour de l'API Réseaux

### Créer un réseau

Les réseaux supportent maintenant des suffixes de domaine personnalisés et des groupes par défaut.

**Champs supplémentaires :**
```json
{
  "domain_suffix": "mycompany.internal",
  "default_group_ids": ["group-id-1", "group-id-2"]
}
```

**Valeurs par défaut :**
- `domain_suffix`: "internal"
- `default_group_ids`: []

---

## API Tokens API

Tokens d'accès personnels à longue durée de vie pour les scripts, pipelines CI/CD et intégration MCP. Les tokens portent les mêmes permissions que l'utilisateur qui les a créés.

### Lister les tokens

**Endpoint :** `GET /api/v1/users/me/tokens`

**Réponse :** `200 OK`
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "CI/CD pipeline",
    "created_at": "2025-03-01T10:00:00Z",
    "expires_at": null,
    "last_used_at": "2025-03-29T08:12:00Z"
  }
]
```

---

### Créer un token

**Endpoint :** `POST /api/v1/users/me/tokens`

**Corps de la requête :**
```json
{
  "name": "CI/CD pipeline",
  "expires_at": "2026-01-01T00:00:00Z"
}
```
`expires_at` est optionnel. S'il est omis, le token n'expire jamais.

**Réponse :** `201 Created`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "CI/CD pipeline",
  "token": "wirety_a3f4b2c1...",
  "created_at": "2025-03-29T10:00:00Z",
  "expires_at": null
}
```

> ⚠️ Le champ `token` n'est retourné qu'au moment de la création. Stockez-le de façon sécurisée — il ne peut pas être récupéré à nouveau.

---

### Révoquer un token

**Endpoint :** `DELETE /api/v1/users/me/tokens/:tokenId`

**Réponse :** `204 No Content`

---

## Mises à jour de l'API Peers

### Changements du modèle Peer

**Champs supprimés :**
- `is_isolated` - Remplacé par les politiques
- `full_encapsulation` - Remplacé par les politiques

**Nouveaux champs :**
- `group_ids`: Tableau des IDs de groupes auxquels le peer appartient

**Exemple de réponse Peer :**
```json
{
  "id": "peer-1",
  "name": "laptop-1",
  "public_key": "...",
  "address": "10.0.0.2",
  "is_jump": false,
  "group_ids": ["group-1", "group-2"],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

---

## Format des réponses d'erreur

Toutes les réponses d'erreur suivent ce format :

```json
{
  "error": "Message d'erreur descriptif"
}
```

**Codes HTTP courants :**
- `200 OK` - Opération GET/PUT/POST réussie
- `201 Created` - Création de ressource réussie
- `204 No Content` - Opération DELETE réussie
- `400 Bad Request` - Données de requête invalides
- `403 Forbidden` - Permissions insuffisantes
- `404 Not Found` - Ressource introuvable
- `409 Conflict` - Conflit de ressource (ex. nom en doublon)
- `500 Internal Server Error` - Erreur serveur

---

## Limitation de débit

Les endpoints API sont limités en débit pour prévenir les abus :
- 100 requêtes par minute par utilisateur
- 1000 requêtes par heure par utilisateur

Les en-têtes de limitation de débit sont inclus dans les réponses :
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1642252800
```

---

## Pagination

Les endpoints de liste supportent la pagination via des paramètres de requête :

**Paramètres de requête :**
- `page`: Numéro de page (défaut : 1)
- `page_size`: Éléments par page (défaut : 50, max : 100)

**Exemple :**
```
GET /api/v1/networks/:networkId/groups?page=2&page_size=25
```

**La réponse inclut les métadonnées de pagination :**
```json
{
  "data": [...],
  "page": 2,
  "page_size": 25,
  "total": 150
}
```
