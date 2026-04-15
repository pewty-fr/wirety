---
id: server
title: Server
sidebar_position: 7
---

Le serveur Wirety fournit des API REST et WebSocket, orchestre les peers, les incidents, les ACL et l'IPAM.

## Variables d'environnement

### Core
| Variable | Description | Défaut |
|----------|-------------|--------|
| `HTTP_PORT` | Port HTTP du serveur | `8080` |
| `CORS_ORIGIN` | Origine(s) CORS autorisée(s) — séparées par des virgules pour plusieurs origines (ex. `https://app.example.com,https://admin.example.com`). `ALLOWED_ORIGIN` est un alias hérité. | `*` |
| `AUDIT_LOG` | Activer la journalisation d'audit JSON structurée sur stdout | `false` |

### Authentification
| Variable | Description | Défaut |
|----------|-------------|--------|
| `AUTH_ENABLED` | Activer l'authentification OIDC (false = auth simple) | `false` |
| `AUTH_ISSUER_URL` | URL du fournisseur OIDC (ex. `https://keycloak.example.com/realms/wirety`) | - |
| `AUTH_CLIENT_ID` | ID client OIDC | - |
| `AUTH_CLIENT_SECRET` | Secret client OIDC | - |
| `AUTH_JWKS_CACHE_TTL` | Durée du cache JWKS en secondes | `3600` |
| `AUTH_PASSWORD` | Mot de passe administrateur pour le mode auth simple | généré automatiquement (journalisé au démarrage) |
| `COOKIE_SECURE` | Active l'attribut `Secure` sur le cookie de session — désactiver uniquement en HTTP local (développement) | `true` |

### Base de données
| Variable | Description | Défaut |
|----------|-------------|--------|
| `DB_ENABLED` | Activer la persistance PostgreSQL (false = en mémoire) | `false` |
| `DB_DSN` | Chaîne de connexion PostgreSQL | `postgres://wirety:wirety@localhost:5432/wirety?sslmode=disable` |
| `DB_MIGRATIONS_DIR` | Chemin vers les fichiers de migration SQL | `cmd/kodata` |

## Modes d'authentification

### Auth simple (défaut, `AUTH_ENABLED=false`)
Au premier démarrage, le serveur génère un mot de passe administrateur aléatoire et le journalise :
```
WRN Simple auth enabled - generated admin password password=abc123 username=admin
```
Définir un mot de passe fixe via `AUTH_PASSWORD` pour éviter la régénération au redémarrage.

Connexion via `POST /api/v1/auth/simple-login` avec `{"username":"admin","password":"..."}`.

### OIDC / OAuth (`AUTH_ENABLED=true`)
Les utilisateurs s'authentifient via votre fournisseur d'identité ; leurs rôles et accès réseau sont gérés dans l'interface Wirety.

Consultez le [guide Fournisseurs d'identité](./guides/identity-providers) pour la configuration étape par étape avec **Keycloak**, **Azure Entra ID**, **Slack** et **GitHub**. Tout autre fournisseur OIDC standard fonctionne également sans configuration supplémentaire.

## Tokens API
Les utilisateurs peuvent créer des tokens API à longue durée de vie (mêmes permissions que leur compte) pour les scripts et intégrations :

```bash
# Créer un token
curl -X POST http://localhost:8080/api/v1/users/me/tokens \
  -H "Authorization: Bearer <session-token>" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-ci-token"}'

# Utiliser un token
curl http://localhost:8080/api/v1/networks \
  -H "Authorization: Bearer wirety_<64-hex-chars>"
```

Les tokens utilisent le préfixe `wirety_` et sont acceptés en mode auth simple et OIDC. Le token brut n'est affiché qu'une seule fois à la création ; seul son hash SHA-256 est stocké.

## Serveur MCP
Un serveur [Model Context Protocol](https://modelcontextprotocol.io) intégré est disponible à `GET/POST /mcp` avec le transport HTTP Streamable. Il expose les capacités de Wirety comme des outils appelables par l'IA (lister/créer/supprimer réseaux, peers, groupes, politiques, routes, incidents et tokens API).

L'authentification utilise les mêmes tokens API que l'API REST :
```json
{
  "mcpServers": {
    "wirety": {
      "type": "http",
      "url": "http://localhost:8080/mcp",
      "headers": { "Authorization": "Bearer wirety_<token>" }
    }
  }
}
```

Consultez la documentation MCP pour la configuration Claude Desktop / Claude Code.

## Données stockées
- Peers (clé publique, endpoint, flags, token, IP supplémentaires autorisées).
- Réseaux (CIDR, domaine, liste de peers).
- Carte ACL BlockedPeers.
- États des incidents + audit (resolvedBy).
- Allocations IPAM.
- Tokens API (hachés).

## Swagger / OpenAPI
La documentation Swagger est disponible à `/swagger/docs/index.html` lors de l'exécution du serveur. L'API est documentée avec :
- Titre : Wirety Server API
- Version : 1.0
- BasePath : /api/v1
- Sécurité : authentification par token Bearer (JWT ou token API `wirety_`)

## Notifications
Le canal WebSocket émet des événements de mise à jour des peers réseau, permettant aux agents de rafraîchir leurs configurations.
