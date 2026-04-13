---
id: api-reference
title: Référence API
sidebar_position: 10
---

# Référence API

Référence complète de l'API REST Wirety.

## Présentation générale

### URL de base

Tous les endpoints sont relatifs à :

```
/api/v1
```

### Authentification

La plupart des endpoints nécessitent une authentification. Trois mécanismes sont acceptés et peuvent être combinés :

| Mécanisme | En-tête / Cookie | Valeur |
|-----------|-----------------|--------|
| Hash de session | `Authorization: Session <hash>` | Obtenu lors de la connexion ou de l'échange de token OIDC |
| Token API | `Authorization: Bearer wirety_<hex>` | Token d'accès personnel à longue durée de vie |
| Cookie de session | `wirety_session` (cookie HttpOnly) | Défini automatiquement par le serveur lors de la connexion |

Les cookies de session sont définis automatiquement lors de l'utilisation des endpoints de connexion ou d'échange de token.

### Rôles

| Rôle | Description |
|------|-------------|
| `administrator` | Accès complet à toutes les ressources sur tous les réseaux |
| `user` | Accès limité aux réseaux autorisés et aux peers propres |

Les endpoints marqués **[admin]** nécessitent le rôle `administrator` et retournent `403 Forbidden` sinon.

### Format des erreurs

Toutes les réponses d'erreur utilisent :

```json
{ "error": "message d'erreur lisible" }
```

### Pagination

Les endpoints paginés acceptent les paramètres `page` (défaut `1`) et `page_size` (défaut `20`) et retournent :

```json
{
  "data": [...],
  "total": 100,
  "page": 1,
  "page_size": 20
}
```

---

## Santé

### Vérification de l'état

Vérifie que le serveur API est en cours d'exécution. Aucune authentification requise.

**`GET /health`**

**Réponse `200`**
```json
{ "status": "ok" }
```

---

## Authentification

### Obtenir la configuration d'authentification

Retourne la configuration d'authentification publique. Aucune authentification requise.

**`GET /auth/config`**

**Réponse `200`**
```json
{
  "enabled": true,
  "issuer_url": "https://accounts.example.com",
  "client_id": "wirety-app",
  "simple_auth": false
}
```

| Champ | Description |
|-------|-------------|
| `enabled` | Indique si l'authentification OIDC est activée |
| `issuer_url` | URL de l'émetteur OIDC (vide si OIDC est désactivé) |
| `client_id` | ID client OIDC pour le frontend |
| `simple_auth` | `true` quand `AUTH_ENABLED=false` — connexion admin/mot de passe utilisée à la place |

---

### Échanger un token OIDC

Échange un code d'autorisation OIDC contre une session côté serveur. Disponible uniquement quand OIDC est activé (`enabled: true` dans la configuration d'authentification).

**`POST /auth/token`**

**Corps de la requête**
```json
{
  "code": "authorization_code_from_oidc",
  "redirect_uri": "https://app.example.com/callback"
}
```

**Réponse `200`**
```json
{
  "session_hash": "abc123...",
  "expires_in": 3600
}
```

Le serveur définit également un cookie HttpOnly `wirety_session`. Les requêtes suivantes peuvent utiliser soit la valeur `session_hash` dans l'en-tête `Authorization: Session`, soit le cookie.

---

### Connexion (Auth Simple)

Authentification avec nom d'utilisateur/mot de passe. Disponible uniquement quand OIDC est désactivé (`simple_auth: true` dans la configuration d'authentification).

**`POST /auth/login`**

**Corps de la requête**
```json
{
  "username": "admin",
  "password": "votre-mot-de-passe-admin"
}
```

**Réponse `200`**
```json
{
  "session_hash": "abc123...",
  "expires_in": 2592000
}
```

Le serveur définit également un cookie HttpOnly `wirety_session`.

---

### Déconnexion

Invalide la session courante.

**`POST /auth/logout`**

**Corps de la requête** (optionnel — la session est de préférence résolue depuis le cookie ou l'en-tête `Authorization`)
```json
{
  "session_hash": "abc123..."
}
```

**Réponse `200`**
```json
{ "message": "Logged out successfully" }
```

---

## Utilisateurs

### Obtenir l'utilisateur courant

Retourne le profil de l'utilisateur authentifié.

**`GET /users/me`**

**Réponse `200`**
```json
{
  "id": "user-sub-from-oidc",
  "email": "alice@example.com",
  "name": "Alice",
  "role": "user",
  "authorized_networks": ["net-id-1", "net-id-2"],
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-10T12:00:00Z",
  "last_login_at": "2024-04-10T08:00:00Z"
}
```

---

### Lister les utilisateurs [admin]

Retourne tous les utilisateurs enregistrés.

**`GET /users`**

**Réponse `200`** — tableau d'objets User (même structure que ci-dessus).

---

### Obtenir un utilisateur [admin]

**`GET /users/:userId`**

**Paramètres de chemin**

| Paramètre | Description |
|-----------|-------------|
| `userId` | ID de l'utilisateur |

**Réponse `200`** — objet User.

---

### Mettre à jour un utilisateur [admin]

Met à jour le nom, le rôle ou les réseaux autorisés d'un utilisateur.

**`PUT /users/:userId`**

**Corps de la requête**
```json
{
  "name": "Alice Smith",
  "role": "administrator",
  "authorized_networks": ["net-id-1"]
}
```

Tous les champs sont optionnels. **Réponse `200`** — objet User mis à jour.

---

### Supprimer un utilisateur [admin]

**`DELETE /users/:userId`**

**Réponse `204 No Content`**

---

### Obtenir les permissions par défaut [admin]

Retourne le rôle et la liste de réseaux par défaut appliqués aux nouveaux utilisateurs.

**`GET /users/defaults`**

**Réponse `200`**
```json
{
  "default_role": "user",
  "default_authorized_networks": ["net-id-1"]
}
```

---

### Mettre à jour les permissions par défaut [admin]

**`PUT /users/defaults`**

**Corps de la requête**
```json
{
  "default_role": "user",
  "default_authorized_networks": ["net-id-1"]
}
```

**Réponse `200`** — objet DefaultNetworkPermissions mis à jour.

---

## Tokens API

Les tokens appartiennent à l'utilisateur authentifié et utilisent le préfixe `wirety_`.

### Lister les tokens API

**`GET /users/me/tokens`**

**Réponse `200`** — tableau d'objets token (le token brut **n'est pas** inclus dans la liste).
```json
[
  {
    "id": "token-uuid",
    "name": "ci-bot",
    "created_at": "2024-01-01T00:00:00Z",
    "expires_at": null,
    "last_used_at": "2024-04-10T08:00:00Z"
  }
]
```

---

### Créer un token API

**`POST /users/me/tokens`**

**Corps de la requête**
```json
{
  "name": "ci-bot",
  "expires_at": "2025-01-01T00:00:00Z"
}
```

`expires_at` est optionnel — omettez-le pour un token sans expiration.

**Réponse `201`**
```json
{
  "id": "token-uuid",
  "name": "ci-bot",
  "token": "wirety_deadbeef...",
  "created_at": "2024-04-13T00:00:00Z",
  "expires_at": "2025-01-01T00:00:00Z",
  "last_used_at": null
}
```

**Le champ `token` n'est affiché qu'une seule fois.** Conservez-le en lieu sûr — il ne peut pas être récupéré à nouveau.

---

### Supprimer un token API

**`DELETE /users/me/tokens/:tokenId`**

**Réponse `204 No Content`**

---

## Réseaux

### Lister les réseaux

Retourne les réseaux accessibles à l'utilisateur authentifié, avec pagination et filtrage optionnel.

**`GET /networks`**

**Paramètres de requête**

| Paramètre | Défaut | Description |
|-----------|--------|-------------|
| `page` | `1` | Numéro de page |
| `page_size` | `20` | Éléments par page (max 200) |
| `filter` | — | Filtre par sous-chaîne (insensible à la casse) sur le nom, le CIDR ou l'ID |

**Réponse `200`**
```json
{
  "data": [
    {
      "id": "net-uuid",
      "name": "bureau",
      "cidr": "10.10.0.0/16",
      "peer_count": 12,
      "dns": ["1.1.1.1"],
      "domain_suffix": "internal",
      "default_group_ids": [],
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-04-01T00:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 20
}
```

| Champ | Description |
|-------|-------------|
| `peer_count` | Nombre de peers (calculé) |
| `dns` | Serveurs DNS supplémentaires poussés aux peers |
| `domain_suffix` | Suffixe de domaine DNS interne (défaut : `internal`) |
| `default_group_ids` | Groupes automatiquement assignés aux peers non-admin |

---

### Créer un réseau [admin]

**`POST /networks`**

**Corps de la requête**
```json
{
  "name": "bureau",
  "cidr": "10.10.0.0/16",
  "dns": ["1.1.1.1", "8.8.8.8"],
  "domain_suffix": "internal"
}
```

`dns` et `domain_suffix` sont optionnels. **Réponse `201`** — objet Network.

---

### Obtenir un réseau

**`GET /networks/:networkId`**

**Réponse `200`** — objet Network.

---

### Mettre à jour un réseau [admin]

**`PUT /networks/:networkId`**

**Corps de la requête** (tous les champs optionnels)
```json
{
  "name": "bureau-v2",
  "cidr": "10.20.0.0/16",
  "dns": ["1.1.1.1"],
  "domain_suffix": "corp",
  "default_group_ids": ["group-uuid"]
}
```

**Réponse `200`** — objet Network mis à jour.

---

### Supprimer un réseau [admin]

**`DELETE /networks/:networkId`**

**Réponse `204 No Content`**

---

## Peers

### Lister les peers

**`GET /networks/:networkId/peers`**

Les utilisateurs non-admin ne voient que leurs propres peers. Supporte la pagination et le filtrage.

**Paramètres de requête**

| Paramètre | Défaut | Description |
|-----------|--------|-------------|
| `page` | `1` | Numéro de page |
| `page_size` | `20` | Éléments par page (max 500) |
| `filter` | — | Filtre par sous-chaîne sur le nom, l'adresse IP ou l'ID |

**Réponse `200`**
```json
{
  "data": [
    {
      "id": "peer-uuid",
      "name": "laptop-alice",
      "public_key": "base64pubkey=",
      "address": "10.10.0.2",
      "endpoint": "203.0.113.5:51820",
      "listen_port": 51820,
      "additional_allowed_ips": ["192.168.1.0/24"],
      "token": "enroll-token-value",
      "is_jump": false,
      "use_agent": true,
      "owner_id": "user-sub-123",
      "group_ids": ["group-uuid"],
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-04-01T00:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 20
}
```

| Champ | Description |
|-------|-------------|
| `endpoint` | IP:port externe (principalement pour les peers jump) |
| `listen_port` | Port d'écoute WireGuard (principalement pour les peers jump) |
| `additional_allowed_ips` | CIDRs supplémentaires que ce peer peut router |
| `token` | Token d'inscription de l'agent (secret, à traiter avec précaution) |
| `is_jump` | Indique si ce peer joue le rôle de hub/serveur jump |
| `use_agent` | Indique si l'agent dynamique gère ce peer |
| `owner_id` | ID utilisateur du propriétaire du peer (vide pour les peers créés par admin) |
| `group_ids` | Groupes auxquels ce peer appartient |

---

### Créer un peer

**`POST /networks/:networkId/peers`**

Les utilisateurs non-admin deviennent automatiquement propriétaires du peer.

**Corps de la requête**
```json
{
  "name": "laptop-alice",
  "endpoint": "203.0.113.5:51820",
  "listen_port": 51820,
  "is_jump": false,
  "use_agent": true,
  "additional_allowed_ips": ["192.168.1.0/24"]
}
```

Tous les champs sauf `name` sont optionnels. **Réponse `201`** — objet Peer.

---

### Obtenir un peer

**`GET /networks/:networkId/peers/:peerId`**

Les utilisateurs non-admin ne peuvent récupérer que leurs propres peers.

**Réponse `200`** — objet Peer.

---

### Mettre à jour un peer

**`PUT /networks/:networkId/peers/:peerId`**

Les utilisateurs non-admin ne peuvent mettre à jour que leurs propres peers. Seuls les admins peuvent modifier `owner_id`.

**Corps de la requête** (tous les champs optionnels)
```json
{
  "name": "laptop-alice-v2",
  "endpoint": "203.0.113.10:51820",
  "listen_port": 51820,
  "additional_allowed_ips": ["192.168.2.0/24"],
  "owner_id": "another-user-id"
}
```

**Réponse `200`** — objet Peer mis à jour.

---

### Supprimer un peer

**`DELETE /networks/:networkId/peers/:peerId`**

Les utilisateurs non-admin ne peuvent supprimer que leurs propres peers.

**Réponse `204 No Content`**

---

### Obtenir la configuration d'un peer

Retourne le fichier de configuration WireGuard pour un peer.

**`GET /networks/:networkId/peers/:peerId/config`**

Les utilisateurs non-admin ne peuvent récupérer que la config de leurs propres peers.

**Réponse `200`**
```json
{ "config": "[Interface]\nPrivateKey = ...\n..." }
```

---

### Obtenir le statut de session d'un peer

Retourne le statut de la session de sécurité pour un peer (sessions d'agent actives, changements d'endpoint, activité suspecte).

**`GET /networks/:networkId/peers/:peerId/session`**

**Réponse `200`**
```json
{
  "peer_id": "peer-uuid",
  "has_active_agent": true,
  "current_session": {
    "peer_id": "peer-uuid",
    "hostname": "laptop-alice",
    "system_uptime": 86400,
    "wireguard_uptime": 3600,
    "reported_endpoint": "203.0.113.5:51820",
    "last_seen": "2024-04-13T10:00:00Z",
    "first_seen": "2024-04-12T09:00:00Z",
    "session_id": "sess-uuid"
  },
  "conflicting_sessions": [],
  "recent_endpoint_changes": [],
  "suspicious_activity": false,
  "last_checked": "2024-04-13T10:05:00Z"
}
```

---

### Obtenir l'accessibilité d'un peer

Calcule quels peers, règles de politique et routes externes sont accessibles depuis un peer donné, en fonction de la configuration ACL et groupe/politique.

**`GET /networks/:networkId/peers/:peerId/reachability`**

**Réponse `200`**
```json
{
  "peer_id": "peer-uuid",
  "peer_name": "laptop-alice",
  "peer_address": "10.10.0.2",
  "is_jump": false,
  "peer_access": [
    {
      "peer_id": "peer-uuid-2",
      "peer_name": "server-bob",
      "address": "10.10.0.3",
      "is_jump": false,
      "allowed": true,
      "reason": "default_allow"
    }
  ],
  "rules": [
    {
      "direction": "output",
      "action": "allow",
      "target_type": "cidr",
      "target": "192.168.1.0/24",
      "addresses": ["192.168.1.0/24"],
      "policy_name": "allow-bureau",
      "group_name": "ingenierie",
      "description": "Autoriser le sous-réseau bureau"
    }
  ],
  "routes": [
    {
      "route_id": "route-uuid",
      "route_name": "bureau-lan",
      "destination_cidr": "192.168.1.0/24",
      "jump_peer_id": "jump-uuid",
      "jump_peer_name": "jump-server-1",
      "group_name": "ingenierie"
    }
  ]
}
```

Valeurs `reason` pour `peer_access` :

| Valeur | Signification |
|--------|---------------|
| `acl_disabled` | ACL désactivé — tous les peers peuvent communiquer |
| `allow_rule` | Correspondance avec une règle d'autorisation explicite |
| `deny_rule` | Correspondance avec une règle de refus explicite |
| `blocked` | Peer dans la liste de blocage |
| `default_allow` | Aucune règle correspondante — l'accès par défaut est autorisé |

---

## Groupes

Les groupes nécessitent `DB_ENABLED=true`. Tous les endpoints de groupe sont **[admin]** uniquement.

### Lister les groupes [admin]

**`GET /networks/:networkId/groups`**

**Réponse `200`** — tableau d'objets Group.

```json
[
  {
    "id": "group-uuid",
    "network_id": "net-uuid",
    "name": "ingenierie",
    "description": "Équipe ingénierie",
    "priority": 100,
    "peer_ids": ["peer-uuid"],
    "policy_ids": ["policy-uuid"],
    "route_ids": ["route-uuid"],
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-04-01T00:00:00Z"
  }
]
```

| Champ | Description |
|-------|-------------|
| `priority` | Ordre d'application des politiques — valeur plus faible = priorité plus élevée (plage 1–999) |

---

### Créer un groupe [admin]

**`POST /networks/:networkId/groups`**

**Corps de la requête**
```json
{
  "name": "ingenierie",
  "description": "Équipe ingénierie",
  "priority": 100
}
```

`description` et `priority` sont optionnels (priorité par défaut : 100). **Réponse `201`** — objet Group.

---

### Obtenir un groupe [admin]

**`GET /networks/:networkId/groups/:groupId`**

**Réponse `200`** — objet Group.

---

### Mettre à jour un groupe [admin]

**`PUT /networks/:networkId/groups/:groupId`**

**Corps de la requête** (tous les champs optionnels)
```json
{
  "name": "ingenierie-v2",
  "description": "Description mise à jour",
  "priority": 50
}
```

**Réponse `200`** — objet Group mis à jour.

---

### Supprimer un groupe [admin]

**`DELETE /networks/:networkId/groups/:groupId`**

**Réponse `204 No Content`**

---

### Ajouter un peer à un groupe [admin]

**`POST /networks/:networkId/groups/:groupId/peers/:peerId`**

**Réponse `200`**

Retourne `400 Bad Request` avec des détails si l'ajout du peer crée une dépendance de routage circulaire.

---

### Retirer un peer d'un groupe [admin]

**`DELETE /networks/:networkId/groups/:groupId/peers/:peerId`**

**Réponse `204 No Content`**

---

### Obtenir les routes d'un groupe [admin]

**`GET /networks/:networkId/groups/:groupId/routes`**

**Réponse `200`** — tableau d'objets Route attachés au groupe.

---

## Politiques

Les politiques nécessitent `DB_ENABLED=true`. Tous les endpoints de politique sont **[admin]** uniquement.

### Lister les politiques [admin]

**`GET /networks/:networkId/policies`**

**Réponse `200`** — tableau d'objets Policy.

```json
[
  {
    "id": "policy-uuid",
    "network_id": "net-uuid",
    "name": "allow-bureau",
    "description": "Autoriser l'accès au sous-réseau bureau",
    "rules": [
      {
        "id": "rule-uuid",
        "direction": "output",
        "action": "allow",
        "target": "192.168.1.0/24",
        "target_type": "cidr",
        "description": "LAN Bureau"
      }
    ],
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-04-01T00:00:00Z"
  }
]
```

**Champs PolicyRule**

| Champ | Valeurs |
|-------|---------|
| `direction` | `"input"` ou `"output"` |
| `action` | `"allow"` ou `"deny"` |
| `target_type` | `"cidr"`, `"peer"`, ou `"group"` |
| `target` | Chaîne CIDR, ID de peer ou ID de groupe selon `target_type` |

---

### Créer une politique [admin]

**`POST /networks/:networkId/policies`**

**Corps de la requête**
```json
{
  "name": "allow-bureau",
  "description": "Autoriser l'accès au sous-réseau bureau",
  "rules": [
    {
      "direction": "output",
      "action": "allow",
      "target": "192.168.1.0/24",
      "target_type": "cidr",
      "description": "LAN Bureau"
    }
  ]
}
```

`description` et `rules` sont optionnels. **Réponse `201`** — objet Policy.

---

### Obtenir une politique [admin]

**`GET /networks/:networkId/policies/:policyId`**

**Réponse `200`** — objet Policy.

---

### Mettre à jour une politique [admin]

**`PUT /networks/:networkId/policies/:policyId`**

**Corps de la requête** (tous les champs optionnels)
```json
{
  "name": "allow-bureau-v2",
  "description": "Description mise à jour"
}
```

**Réponse `200`** — objet Policy mis à jour.

---

### Supprimer une politique [admin]

**`DELETE /networks/:networkId/policies/:policyId`**

**Réponse `204 No Content`**

---

### Ajouter une règle à une politique [admin]

**`POST /networks/:networkId/policies/:policyId/rules`**

**Corps de la requête**
```json
{
  "direction": "output",
  "action": "allow",
  "target": "192.168.1.0/24",
  "target_type": "cidr",
  "description": "LAN Bureau"
}
```

**Réponse `201`** — objet PolicyRule.

---

### Retirer une règle d'une politique [admin]

**`DELETE /networks/:networkId/policies/:policyId/rules/:ruleId`**

**Réponse `204 No Content`**

---

### Attacher une politique à un groupe [admin]

**`POST /networks/:networkId/groups/:groupId/policies/:policyId`**

**Réponse `200`**

---

### Détacher une politique d'un groupe [admin]

**`DELETE /networks/:networkId/groups/:groupId/policies/:policyId`**

**Réponse `204 No Content`**

---

### Obtenir les politiques d'un groupe [admin]

**`GET /networks/:networkId/groups/:groupId/policies`**

**Réponse `200`** — tableau d'objets Policy attachés au groupe.

---

### Réordonner les politiques d'un groupe [admin]

Définit l'ordre d'évaluation des politiques au sein d'un groupe. Le tableau doit contenir tous les IDs de politiques actuellement attachés au groupe.

**`PUT /networks/:networkId/groups/:groupId/policies/order`**

**Corps de la requête**
```json
{
  "policy_ids": ["policy-uuid-1", "policy-uuid-2"]
}
```

**Réponse `200`**
```json
{ "message": "Policies reordered successfully" }
```

---

## Routes

Les routes nécessitent `DB_ENABLED=true`. Tous les endpoints de route sont **[admin]** uniquement.

### Lister les routes [admin]

**`GET /networks/:networkId/routes`**

**Réponse `200`** — tableau d'objets Route.

```json
[
  {
    "id": "route-uuid",
    "network_id": "net-uuid",
    "name": "bureau-lan",
    "description": "Réseau interne bureau",
    "destination_cidr": "192.168.1.0/24",
    "jump_peer_id": "jump-uuid",
    "domain_suffix": "bureau.internal",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-04-01T00:00:00Z"
  }
]
```

---

### Créer une route [admin]

**`POST /networks/:networkId/routes`**

**Corps de la requête**
```json
{
  "name": "bureau-lan",
  "description": "Réseau interne bureau",
  "destination_cidr": "192.168.1.0/24",
  "jump_peer_id": "jump-uuid",
  "domain_suffix": "bureau.internal"
}
```

`description` et `domain_suffix` sont optionnels. **Réponse `201`** — objet Route.

---

### Obtenir une route [admin]

**`GET /networks/:networkId/routes/:routeId`**

**Réponse `200`** — objet Route.

---

### Mettre à jour une route [admin]

**`PUT /networks/:networkId/routes/:routeId`**

**Corps de la requête** (tous les champs optionnels)
```json
{
  "name": "bureau-lan-v2",
  "description": "Description mise à jour",
  "destination_cidr": "192.168.2.0/24",
  "jump_peer_id": "jump-uuid-2",
  "domain_suffix": "corp.internal"
}
```

**Réponse `200`** — objet Route mis à jour.

---

### Supprimer une route [admin]

**`DELETE /networks/:networkId/routes/:routeId`**

**Réponse `204 No Content`**

---

### Attacher une route à un groupe [admin]

**`POST /networks/:networkId/groups/:groupId/routes/:routeId`**

**Réponse `200`**

Retourne `400 Bad Request` avec des détails si l'attachement crée une dépendance de routage circulaire.

---

### Détacher une route d'un groupe [admin]

**`DELETE /networks/:networkId/groups/:groupId/routes/:routeId`**

**Réponse `204 No Content`**

---

## Correspondances DNS

Les correspondances DNS nécessitent `DB_ENABLED=true`. Tous les endpoints DNS sont **[admin]** uniquement.

Les correspondances DNS résolvent des noms d'hôte dans le CIDR d'une route vers des adresses IP spécifiques. Le format FQDN est `<nom>.<nom_route>.<suffixe_domaine>`.

### Lister les correspondances DNS [admin]

**`GET /networks/:networkId/routes/:routeId/dns`**

**Réponse `200`** — tableau d'objets DNSMapping.

```json
[
  {
    "id": "dns-uuid",
    "route_id": "route-uuid",
    "name": "server1",
    "ip_address": "192.168.1.10",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-04-01T00:00:00Z"
  }
]
```

---

### Créer une correspondance DNS [admin]

**`POST /networks/:networkId/routes/:routeId/dns`**

**Corps de la requête**
```json
{
  "name": "server1",
  "ip_address": "192.168.1.10"
}
```

Le champ `name` doit respecter les règles des labels DNS (alphanumérique + tirets, max 63 caractères). **Réponse `201`** — objet DNSMapping.

---

### Mettre à jour une correspondance DNS [admin]

**`PUT /networks/:networkId/routes/:routeId/dns/:dnsId`**

**Corps de la requête** (tous les champs optionnels)
```json
{
  "name": "server1-nouveau",
  "ip_address": "192.168.1.11"
}
```

**Réponse `200`** — objet DNSMapping mis à jour.

---

### Supprimer une correspondance DNS [admin]

**`DELETE /networks/:networkId/routes/:routeId/dns/:dnsId`**

**Réponse `204 No Content`**

---

### Obtenir les enregistrements DNS d'un réseau [admin]

Retourne tous les enregistrements DNS pour un réseau (basés sur les peers et les correspondances de route).

**`GET /networks/:networkId/dns`**

**Réponse `200`** — tableau d'objets d'enregistrement DNS.

```json
[
  {
    "name": "laptop-alice",
    "ip_address": "10.10.0.2",
    "fqdn": "laptop-alice.bureau.internal",
    "type": "peer"
  },
  {
    "name": "server1",
    "ip_address": "192.168.1.10",
    "fqdn": "server1.bureau-lan.bureau.internal",
    "type": "route"
  }
]
```

| `type` | Source |
|--------|--------|
| `"peer"` | Peer dans le réseau |
| `"route"` | Correspondance DNS attachée à une route |

---

## ACL

Les endpoints ACL sont **[admin]** uniquement.

L'ACL est une couche légère d'autorisation/refus opérant au niveau peer-à-peer. Elle complète les politiques de groupe. Lorsqu'elle est désactivée, tous les peers peuvent communiquer librement.

### Obtenir l'ACL [admin]

**`GET /networks/:networkId/acl`**

**Réponse `200`**
```json
{
  "id": "acl-uuid",
  "name": "default",
  "enabled": true,
  "blocked_peers": {
    "peer-uuid-bad": true
  },
  "rules": [
    {
      "id": "rule-uuid",
      "source_peer": "*",
      "target_peer": "peer-uuid-3",
      "action": "deny",
      "description": "Bloquer l'accès au serveur"
    }
  ]
}
```

| Champ | Description |
|-------|-------------|
| `enabled` | Quand `false`, l'ACL est contournée et tous les peers peuvent communiquer |
| `blocked_peers` | Map des IDs de peers inconditionnellement bloqués |
| `rules` | Liste ordonnée de règles ; la première correspondance l'emporte ; défaut : autoriser |
| `source_peer` / `target_peer` | ID de peer ou `"*"` pour tous les peers |

---

### Mettre à jour l'ACL [admin]

Remplace la configuration ACL complète.

**`PUT /networks/:networkId/acl`**

**Corps de la requête** — objet ACL complet (même structure que la réponse GET).

**Réponse `200`** — objet ACL mis à jour. Notifie les agents connectés via WebSocket.

---

## Sécurité / Incidents

### Lister les incidents de sécurité

Retourne les incidents de sécurité. Les utilisateurs non-admin ne voient que les incidents pour leurs propres peers.

**`GET /security/incidents`**

**Paramètres de requête**

| Paramètre | Description |
|-----------|-------------|
| `resolved` | `true` ou `false` pour filtrer par statut résolu ; omettez pour tous |

**Réponse `200`** — tableau d'objets SecurityIncident.

```json
[
  {
    "id": "incident-uuid",
    "peer_id": "peer-uuid",
    "peer_name": "laptop-alice",
    "network_id": "net-uuid",
    "network_name": "bureau",
    "incident_type": "session_conflict",
    "detected_at": "2024-04-13T08:00:00Z",
    "public_key": "base64pubkey=",
    "endpoints": ["203.0.113.5:51820", "198.51.100.1:51820"],
    "details": "Plusieurs agents actifs détectés pour le même peer",
    "resolved": false,
    "resolved_at": null,
    "resolved_by": null
  }
]
```

Valeurs `incident_type` :

| Valeur | Description |
|--------|-------------|
| `shared_config` | Même clé WireGuard utilisée depuis plusieurs IPs |
| `session_conflict` | Plusieurs agents actifs pour le même peer |
| `suspicious_activity` | Changements d'endpoint rapides détectés |

---

### Lister les incidents de sécurité d'un réseau

**`GET /networks/:networkId/security/incidents`**

Même comportement que l'endpoint global mais limité à un réseau. Accepte le même paramètre `resolved`.

**Réponse `200`** — tableau d'objets SecurityIncident.

---

### Obtenir un incident de sécurité

**`GET /security/incidents/:incidentId`**

Les utilisateurs non-admin ne peuvent récupérer que les incidents pour leurs propres peers.

**Réponse `200`** — objet SecurityIncident.

---

### Résoudre un incident de sécurité [admin]

**`POST /security/incidents/:incidentId/resolve`**

**Réponse `200`**
```json
{ "message": "Incident resolved successfully" }
```

---

### Obtenir la configuration de sécurité d'un réseau [admin]

**`GET /networks/:networkId/security/config`**

**Réponse `200`**
```json
{
  "id": "cfg-uuid",
  "network_id": "net-uuid",
  "enabled": true,
  "session_conflict_threshold_minutes": 5,
  "endpoint_change_threshold_minutes": 5,
  "max_endpoint_changes_per_day": 10,
  "port_change_threshold_minutes": 5,
  "max_port_changes_per_window": 5,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-04-13T00:00:00Z"
}
```

Toutes les valeurs de seuil sont en **minutes**.

---

### Mettre à jour la configuration de sécurité d'un réseau [admin]

**`PUT /networks/:networkId/security/config`**

**Corps de la requête** (tous les champs optionnels)
```json
{
  "enabled": true,
  "session_conflict_threshold_minutes": 5,
  "endpoint_change_threshold_minutes": 10,
  "max_endpoint_changes_per_day": 20,
  "port_change_threshold_minutes": 5,
  "max_port_changes_per_window": 10
}
```

Validation : toutes les valeurs de seuil doivent être ≥ 1. `max_endpoint_changes_per_day` et `max_port_changes_per_window` doivent être entre 1 et 1000.

**Réponse `200`** — objet SecurityConfigResponse mis à jour.

---

## IPAM

### Suggérer des CIDRs disponibles

Suggère un ou plusieurs CIDRs de la bonne taille pour un nombre cible de peers, découpés depuis un CIDR de base. Utile lors de la planification d'un nouveau réseau.

**`GET /ipam/available-cidrs`**

**Paramètres de requête**

| Paramètre | Requis | Défaut | Description |
|-----------|--------|--------|-------------|
| `max_peers` | Oui | — | Nombre minimum d'adresses d'hôtes utilisables nécessaires |
| `count` | Non | `1` | Nombre de CIDRs à retourner (max 20) |
| `base_cidr` | Non | `10.0.0.0/8` | CIDR racine depuis lequel découper |

**Réponse `200`**
```json
{
  "base_cidr": "10.0.0.0/8",
  "requested_max_peers": 50,
  "suggested_prefix": 26,
  "usable_hosts": 62,
  "cidrs": ["10.0.0.0/26", "10.0.0.64/26"]
}
```

---

### Lister les allocations IPAM

Retourne toutes les allocations IP sur les réseaux accessibles, avec pagination et filtrage.

**`GET /ipam`**

Les utilisateurs non-admin ne voient que les peers dont ils sont propriétaires.

**Paramètres de requête**

| Paramètre | Défaut | Description |
|-----------|--------|-------------|
| `page` | `1` | Numéro de page |
| `page_size` | `20` | Éléments par page (max 100) |
| `filter` | — | Filtre par sous-chaîne sur le nom du réseau, l'IP ou le nom du peer |

**Réponse `200`**
```json
{
  "data": [
    {
      "network_id": "net-uuid",
      "network_name": "bureau",
      "network_cidr": "10.10.0.0/16",
      "ip": "10.10.0.2",
      "peer_id": "peer-uuid",
      "peer_name": "laptop-alice",
      "allocated": true
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 20
}
```

---

### Obtenir les allocations IPAM d'un réseau

**`GET /ipam/networks/:networkId`**

**Réponse `200`** — tableau d'objets IPAMAllocation pour le réseau spécifié.

---

## Sessions

### Lister les sessions d'un réseau

Retourne les sessions d'agent actives dans un réseau.

**`GET /networks/:networkId/sessions`**

**Réponse `200`** — tableau d'objets AgentSession.

```json
[
  {
    "peer_id": "peer-uuid",
    "hostname": "laptop-alice",
    "system_uptime": 86400,
    "wireguard_uptime": 3600,
    "reported_endpoint": "203.0.113.5:51820",
    "last_seen": "2024-04-13T10:00:00Z",
    "first_seen": "2024-04-12T09:00:00Z",
    "session_id": "sess-uuid"
  }
]
```

---

## Inscription de l'agent

### Résoudre un token d'agent

Échange un token d'inscription de peer contre les identifiants du peer et sa configuration WireGuard. Cet endpoint n'est pas authentifié (utilise le token lui-même comme authentification via `Authorization: Bearer`).

**`GET /agent/resolve`**

**En-têtes**

```
Authorization: Bearer <enrollment-token>
```

**Réponse `200`**
```json
{
  "network_id": "net-uuid",
  "peer_id": "peer-uuid",
  "peer_name": "laptop-alice",
  "config": "[Interface]\nPrivateKey = ...\n..."
}
```

Retourne `404 Not Found` si le token est invalide ou expiré.

---

## Serveur MCP

Wirety expose un serveur Model Context Protocol (MCP) sur `/api/v1/mcp` en utilisant le transport SSE. Les méthodes `GET` (flux) et `POST` (messages) sur le même chemin sont supportées. L'authentification est requise (mêmes mécanismes de session/token que les autres endpoints).

Consultez la documentation MCP pour la liste des outils disponibles.

---

## WebSocket

### WebSocket (basé sur un token)

**`GET /ws`**

Connexion avec un token WebSocket à courte durée de vie. Délivre des mises à jour de configuration de peer en temps réel.

### WebSocket (héritage)

**`GET /ws/:networkId/:peerId`**

Connexion WebSocket spécifique à un peer en utilisant directement les IDs de réseau et de peer.

---

## Portail captif

### Créer un token de portail captif

Appelé par un agent de peer jump (authentifié via un token d'inscription) lorsqu'un nouveau peer se connecte et doit s'authentifier.

**`POST /captive-portal/token`**

**En-têtes**

```
Authorization: Bearer <jump-peer-enrollment-token>
```

**Corps de la requête**
```json
{ "peer_ip": "10.10.0.5" }
```

**Réponse `201`** — objet token de portail captif.

Nécessite que l'authentification OIDC soit activée (`AUTH_ENABLED=true`).

---

### Authentifier le portail captif

Valide le token de portail captif contre la session utilisateur courante et met le peer sur liste blanche.

**`POST /captive-portal/authenticate`**

Nécessite la présence du cookie `wirety_session` (défini lors de la connexion).

**Corps de la requête**
```json
{ "captive_token": "captive-token-value" }
```

**Réponse `200`**
```json
{
  "peer_ip": "10.10.0.5",
  "network_id": "net-uuid",
  "whitelisted": true
}
```
