---
id: mcp
title: Serveur MCP
sidebar_position: 9
---

Wirety embarque un serveur [Model Context Protocol (MCP)](https://modelcontextprotocol.io) directement dans le binaire principal. Il expose les capacités de Wirety comme des outils appelables par l'IA, permettant à Claude (ou tout assistant compatible MCP) d'explorer et de gérer vos réseaux.

## Endpoint

```
GET/POST /mcp
```

Transport : **HTTP Streamable** (spec MCP 2025-03-26). Le même binaire serveur sert à la fois l'API REST et l'endpoint MCP — aucun processus supplémentaire n'est nécessaire.

## Authentification

Le MCP utilise les mêmes tokens API que l'API REST. Créez-en un depuis votre profil dans l'interface (Profil → Tokens API → Nouveau Token), puis passez-le comme en-tête :

```
Authorization: Bearer wirety_<64-hex-chars>
```

Les permissions sont appliquées par token — un token admin peut appeler les outils réservés aux admins ; un token utilisateur normal ne le peut pas.

## Outils disponibles

### Utilisateurs
| Outil | Description | Admin uniquement |
|-------|-------------|-----------------|
| `get_current_user` | Obtenir le profil de l'utilisateur authentifié | Non |
| `list_users` | Lister tous les utilisateurs | Oui |

### Réseaux
| Outil | Description | Admin uniquement |
|-------|-------------|-----------------|
| `list_networks` | Lister les réseaux WireGuard accessibles | Non |
| `get_network` | Obtenir les détails d'un réseau par ID | Non |
| `create_network` | Créer un nouveau réseau | Oui |
| `update_network` | Mettre à jour le nom/DNS d'un réseau | Oui |
| `delete_network` | Supprimer un réseau | Oui |

### Peers
| Outil | Description | Admin uniquement |
|-------|-------------|-----------------|
| `list_peers` | Lister les peers d'un réseau | Non |
| `get_peer` | Obtenir les détails d'un peer | Non |
| `create_peer` | Créer un nouveau peer | Non |
| `update_peer` | Renommer un peer | Non |
| `delete_peer` | Supprimer un peer | Non |
| `get_peer_config` | Obtenir le fichier config WireGuard d'un peer | Non |

### Groupes *(nécessite DB)*
| Outil | Description | Admin uniquement |
|-------|-------------|-----------------|
| `list_groups` | Lister les groupes d'un réseau | Non |
| `create_group` | Créer un nouveau groupe | Oui |
| `update_group` | Mettre à jour le nom, la description ou la priorité d'un groupe | Oui |

### Politiques *(nécessite DB)*
| Outil | Description | Admin uniquement |
|-------|-------------|-----------------|
| `list_policies` | Lister les politiques d'un réseau | Non |
| `create_policy` | Créer une nouvelle politique avec des règles | Oui |
| `update_policy` | Mettre à jour le nom ou la description d'une politique | Oui |

### Routes *(nécessite DB)*
| Outil | Description | Admin uniquement |
|-------|-------------|-----------------|
| `list_routes` | Lister les routes d'un réseau | Non |
| `create_route` | Créer une route (CIDR de destination via jump peer) | Oui |
| `update_route` | Mettre à jour la configuration d'une route | Oui |

### Incidents de sécurité
| Outil | Description | Admin uniquement |
|-------|-------------|-----------------|
| `list_incidents` | Lister tous les incidents de sécurité | Non |
| `get_incident` | Obtenir les détails d'un incident | Non |
| `resolve_incident` | Marquer un incident comme résolu | Non |

Les outils de groupes, politiques et routes ne sont enregistrés que lorsque le backend de base de données est activé (`DB_ENABLED=true`).

## Configuration Claude Code

Ajouter à `~/.claude/settings.json` (niveau utilisateur, tous les projets) ou `.mcp.json` (niveau projet) :

```json
{
  "mcpServers": {
    "wirety": {
      "type": "http",
      "url": "http://localhost:8080/mcp",
      "headers": {
        "Authorization": "Bearer wirety_<votre-token>"
      }
    }
  }
}
```

## Configuration Claude Desktop

Ajouter à `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) :

```json
{
  "mcpServers": {
    "wirety": {
      "type": "http",
      "url": "http://localhost:8080/mcp",
      "headers": {
        "Authorization": "Bearer wirety_<votre-token>"
      }
    }
  }
}
```

Redémarrer Claude Desktop après avoir modifié la configuration.

## Dépannage

| Problème | Cause | Solution |
|---------|-------|---------|
| "not valid MCP server configurations" dans Claude Desktop | `"type": "http"` manquant | Ajouter `"type": "http"` à la configuration du serveur |
| 401 Unauthorized | Token invalide ou expiré | Re-créer le token dans l'interface |
| Outils manquants (groupes, politiques, routes) | DB non activée | Définir `DB_ENABLED=true` et configurer `DB_DSN` |
| MCP fonctionne via curl mais pas Claude | Mauvais transport | S'assurer que le serveur a été recompilé après la migration vers HTTP Streamable |
