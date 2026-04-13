---
id: audit-logs
title: Logs d'audit
sidebar_position: 11
---

Wirety émet des logs d'audit structurés pour chaque action liée à la sécurité sur le serveur et chaque agent. Les logs d'audit sont distincts des logs opérationnels de l'application : ils sont toujours en JSON, toujours sur **stdout**, et conçus pour l'ingestion par des systèmes d'agrégation de logs (Loki, Elasticsearch, Splunk, etc.).

## Activer les logs d'audit

Définissez la variable d'environnement `AUDIT_LOG=true` sur le **serveur** et/ou chaque instance **agent**.

```bash
# Serveur
AUDIT_LOG=true ./wirety-server

# Agent
AUDIT_LOG=true wirety-agent -server https://wirety.example.com -token <TOKEN>
```

Lorsque `AUDIT_LOG` est `false` (valeur par défaut), le logger d'audit est un vrai no-op — aucune allocation ne se produit, donc il n'y a aucun impact sur les performances.

## Format des logs

Chaque événement d'audit est une seule ligne JSON sur `stdout` :

```json
{
  "level": "info",
  "log_type": "audit",
  "time": 1748000000,
  "actor_id": "alice",
  "actor_email": "alice@example.com",
  "remote_ip": "10.0.0.5",
  "action": "peer.create",
  "network_id": "net-abc123",
  "peer_id": "peer-xyz789",
  "peer_name": "office-laptop",
  "message": "audit"
}
```

| Champ | Toujours présent | Description |
|-------|-----------------|-------------|
| `log_type` | ✓ | Toujours `"audit"` — utiliser ce champ pour filtrer les lignes d'audit depuis les logs applicatifs |
| `time` | ✓ | Timestamp Unix |
| `action` | ✓ | Ce qui s'est passé (voir les tableaux ci-dessous) |
| `message` | ✓ | Toujours `"audit"` |
| `actor_id` | Serveur uniquement | ID de l'utilisateur authentifié qui a déclenché l'action |
| `actor_email` | Serveur uniquement | Email de l'utilisateur authentifié |
| `remote_ip` | Serveur uniquement | Adresse IP du client |
| `peer_id` / `network_id` | Agent uniquement | Identité de l'agent qui a effectué l'action |

---

## Événements serveur

Les événements d'audit serveur sont émis après chaque appel API mutant réussi.

### Authentification

| `action` | Déclencheur |
|----------|------------|
| `auth.login` | Connexion réussie en mode auth simple (`POST /auth/login`) |

### Réseaux

| `action` | Champs | Déclencheur |
|----------|--------|------------|
| `network.create` | `network_id`, `network_name` | Réseau créé |
| `network.update` | `network_id`, `network_name` | Configuration réseau mise à jour |
| `network.delete` | `network_id` | Réseau supprimé |
| `acl.update` | `network_id` | Liste de blocage ACL mise à jour |

### Peers

| `action` | Champs | Déclencheur |
|----------|--------|------------|
| `peer.create` | `network_id`, `peer_id`, `peer_name` | Peer créé |
| `peer.update` | `network_id`, `peer_id`, `peer_name` | Peer mis à jour |
| `peer.delete` | `network_id`, `peer_id` | Peer supprimé |

### Groupes

| `action` | Champs | Déclencheur |
|----------|--------|------------|
| `group.create` | `network_id`, `group_id`, `group_name` | Groupe créé |
| `group.update` | `network_id`, `group_id`, `group_name` | Groupe mis à jour |
| `group.delete` | `network_id`, `group_id` | Groupe supprimé |
| `group.peer.add` | `network_id`, `group_id`, `peer_id` | Peer ajouté au groupe |
| `group.peer.remove` | `network_id`, `group_id`, `peer_id` | Peer retiré du groupe |

### Politiques

| `action` | Champs | Déclencheur |
|----------|--------|------------|
| `policy.create` | `network_id`, `policy_id`, `policy_name` | Politique créée |
| `policy.update` | `network_id`, `policy_id`, `policy_name` | Politique mise à jour |
| `policy.delete` | `network_id`, `policy_id` | Politique supprimée |
| `policy.rule.add` | `network_id`, `policy_id`, `rule_id` | Règle ajoutée à la politique |
| `policy.rule.remove` | `network_id`, `policy_id`, `rule_id` | Règle retirée de la politique |
| `policy.group.attach` | `network_id`, `group_id`, `policy_id` | Politique attachée au groupe |
| `policy.group.detach` | `network_id`, `group_id`, `policy_id` | Politique détachée du groupe |
| `policy.group.reorder` | `network_id`, `group_id` | Ordre de priorité des politiques modifié |

### Routes

| `action` | Champs | Déclencheur |
|----------|--------|------------|
| `route.create` | `network_id`, `route_id`, `route_name` | Route créée |
| `route.update` | `network_id`, `route_id`, `route_name` | Route mise à jour |
| `route.delete` | `network_id`, `route_id` | Route supprimée |
| `route.group.attach` | `network_id`, `group_id`, `route_id` | Route attachée au groupe |
| `route.group.detach` | `network_id`, `group_id`, `route_id` | Route détachée du groupe |

### Utilisateurs

| `action` | Champs | Déclencheur |
|----------|--------|------------|
| `user.update` | `target_user_id` | Rôle utilisateur ou réseaux mis à jour |
| `user.delete` | `target_user_id` | Utilisateur supprimé |
| `user.defaults.update` | — | Permissions réseau par défaut modifiées |

---

## Événements agent

Les événements d'audit agent sont émis par chaque processus agent. Les champs `peer_id` et `network_id` identifient quel agent a produit l'événement.

### Configuration & Pare-feu

| `action` | Champs | Déclencheur |
|----------|--------|------------|
| `config.sync` | — | Configuration WireGuard écrite et appliquée avec succès |
| `firewall.sync` | `rule_count` | Règles iptables appliquées avec succès |
| `dns.update` | `domain`, `peer_count` | Table des peers du serveur DNS mise à jour |
| `peer.rename` | `old_name`, `new_name`, `new_interface` | Peer renommé, interface migrée |

### Sessions de tunnel

L'agent surveille les timestamps de handshake WireGuard toutes les 15 secondes. Un peer est considéré **connecté** lorsque son dernier handshake est dans les 180 dernières secondes (seuil d'inactivité standard de WireGuard).

| `action` | Champs | Déclencheur |
|----------|--------|------------|
| `tunnel.connected` | `peer_name`, `peer_pubkey`, `endpoint`, `handshake_at` | Le peer a établi un handshake WireGuard |
| `tunnel.disconnected` | `peer_name`, `peer_pubkey`, `endpoint`, `last_handshake`, `session_duration` | Le peer a arrêté d'envoyer des handshakes |

Exemples d'événements de tunnel :

```json
{"level":"info","log_type":"audit","time":1748000100,"actor_type":"agent","peer_id":"peer-jump-1","network_id":"net-abc123","action":"tunnel.connected","peer_name":"office-laptop","peer_pubkey":"abc123...","endpoint":"203.0.113.42:51820","handshake_at":1748000098,"message":"audit"}
{"level":"info","log_type":"audit","time":1748003700,"actor_type":"agent","peer_id":"peer-jump-1","network_id":"net-abc123","action":"tunnel.disconnected","peer_name":"office-laptop","peer_pubkey":"abc123...","endpoint":"203.0.113.42:51820","last_handshake":1748003520,"session_duration":3600000000000,"message":"audit"}
```

Note : les événements de tunnel sont également émis comme lignes de log ordinaires (non audit) avec `"event":"tunnel.connected"` / `"event":"tunnel.disconnected"` pour une surveillance lisible par l'humain.

---

## Log opérationnel (non audit)

Le serveur et l'agent émettent également des **logs opérationnels** lisibles par l'humain sur `stderr` via le `ConsoleWriter` de zerolog. Ce ne sont pas des logs d'audit — ne vous fiez pas à eux pour la conformité. Utilisez `AUDIT_LOG=true` + stdout à cette fin.

---

## Exemples d'intégration

### Docker / docker-compose

```yaml
services:
  wirety-server:
    image: rg.fr-par.scw.cloud/wirety/server:latest
    environment:
      AUDIT_LOG: "true"
    logging:
      driver: json-file
```

Redirigez stdout vers votre expéditeur de logs ; stderr transporte le log console opérationnel.

### Kubernetes

```yaml
env:
  - name: AUDIT_LOG
    value: "true"
```

Avec un sidecar expéditeur de logs (Fluent Bit, Vector) configuré pour collecter stdout des pods wirety.

### Filtrage avec jq

```bash
# Tous les événements d'audit de la dernière exécution
docker logs wirety-server 2>/dev/null | jq 'select(.log_type == "audit")'

# Toutes les créations de peers
docker logs wirety-server 2>/dev/null | jq 'select(.action == "peer.create")'

# Toutes les sessions de tunnel d'un agent spécifique
cat agent.log | jq 'select(.log_type == "audit" and .peer_id == "peer-jump-1" and (.action | startswith("tunnel.")))'
```

### Loki / Grafana

Exemple de sélecteur de label (avec le driver de log Docker) :

```logql
{container="wirety-server"} | json | log_type="audit"
```

Filtrer par action :

```logql
{container="wirety-server"} | json | log_type="audit" | action=~"peer\\..*"
```
