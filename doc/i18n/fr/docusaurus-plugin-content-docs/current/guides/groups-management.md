---
id: groups-management
title: Guide de gestion des groupes
sidebar_position: 5
---

Ce guide fournit des informations détaillées sur la gestion des groupes dans le système de gestion de réseaux WireGuard.

## Qu'est-ce qu'un groupe ?

Les groupes sont des collections logiques de peers qui partagent des caractéristiques, des politiques ou des exigences d'accès communes. Ils servent d'unité organisationnelle principale pour l'application des politiques réseau et des routes.

## Concepts clés

### Appartenance aux groupes
- Les peers peuvent appartenir simultanément à plusieurs groupes
- L'appartenance aux groupes est gérée uniquement par les administrateurs
- L'ajout/suppression de peers dans les groupes est non destructif (les peers ne sont pas supprimés)

### Attachements de groupes
Les groupes peuvent avoir :
- **Politiques** : Règles de filtrage du trafic appliquées sur les jump peers
- **Routes** : Destinations réseau externes ajoutées aux configurations des peers

### Application automatique
Lorsqu'un peer rejoint un groupe :
- Toutes les politiques attachées sont automatiquement appliquées
- Toutes les routes attachées sont ajoutées à la configuration WireGuard du peer
- Les changements prennent effet en quelques secondes via les notifications WebSocket

## Créer des groupes

### Création basique d'un groupe

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "engineering-team",
    "description": "Membres de l'\''équipe ingénierie avec accès réseau complet"
  }'
```

### Bonnes pratiques de nommage

**Bons noms :**
- `engineering-team`
- `sales-department`
- `database-servers`
- `trusted-devices`
- `guest-network`

**À éviter :**
- `group1`, `group2` (non descriptif)
- `temp` (objectif peu clair)
- `test` (ambigu)

### Recommandations pour les descriptions

Rédigez des descriptions claires qui expliquent :
- L'objectif du groupe
- Qui devrait en faire partie
- Quel accès il fournit

**Exemple :**
```json
{
  "name": "remote-developers",
  "description": "Équipe de développement à distance avec accès à l'environnement staging et au VPN bureau"
}
```

## Gérer l'appartenance aux groupes

### Ajouter des peers

**Un seul peer :**
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

**Plusieurs peers :**
```bash
#!/bin/bash
PEERS=("peer-1" "peer-2" "peer-3" "peer-4")

for PEER_ID in "${PEERS[@]}"; do
  echo "Ajout de $PEER_ID au groupe..."
  curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
    -H "Authorization: Bearer $TOKEN"
done
```

### Supprimer des peers

```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

**Important :** Supprimer un peer d'un groupe :
- Supprime les politiques attachées du peer
- Supprime les routes attachées de la configuration du peer
- Ne supprime PAS le peer lui-même

### Voir les membres d'un groupe

```bash
# Obtenir les détails du groupe avec la liste des membres
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID" \
  | jq '.peer_ids'
```

## Attacher des politiques aux groupes

### Attacher une politique

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Voir les politiques attachées

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies"
```

### Détacher une politique

```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### L'ordre des politiques a de l'importance

Lorsque plusieurs politiques sont attachées à un groupe :
1. Les règles sont appliquées dans l'ordre d'attachement des politiques
2. La première règle correspondante l'emporte
3. Le refus par défaut s'applique si aucune règle ne correspond

**Exemple :**
```bash
# Attacher les politiques dans l'ordre
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$ALLOW_POLICY" \
  -H "Authorization: Bearer $TOKEN"

curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$DENY_POLICY" \
  -H "Authorization: Bearer $TOKEN"
```

## Attacher des routes aux groupes

### Attacher une route

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Voir les routes attachées

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes"
```

### Détacher une route

```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

## Patterns courants de groupes

### Pattern 1 : Groupes par département

Organiser par structure organisationnelle :

```bash
# Créer les groupes de département
curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "engineering", "description": "Département ingénierie"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "sales", "description": "Département commercial"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "support", "description": "Support client"}'
```

### Pattern 2 : Groupes par rôle

Organiser par fonction :

```bash
# Créer les groupes de rôle
curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "developers", "description": "Développeurs logiciel"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "admins", "description": "Administrateurs système"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "viewers", "description": "Accès lecture seule"}'
```

### Pattern 3 : Groupes par environnement

Organiser par accès à l'environnement :

```bash
# Créer les groupes d'environnement
curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "production-access", "description": "Accès environnement production"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "staging-access", "description": "Accès environnement staging"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "development-access", "description": "Accès environnement développement"}'
```

### Pattern 4 : Groupes par niveau de sécurité

Organiser par niveau de confiance :

```bash
# Créer les groupes de niveau de sécurité
curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "trusted", "description": "Appareils totalement fiables"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "standard", "description": "Appareils standard"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "restricted", "description": "Appareils à accès restreint"}'

curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -d '{"name": "quarantine", "description": "Appareils isolés/compromis"}'
```

## Gestion avancée des groupes

### Script d'opérations en masse

```bash
#!/bin/bash
# bulk-group-operations.sh

API_URL="https://votre-serveur/api/v1"
TOKEN="votre-token"
NETWORK_ID="votre-network-id"

# Fonction de création d'un groupe
create_group() {
  local name=$1
  local description=$2
  
  curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"name\": \"$name\", \"description\": \"$description\"}" \
    | jq -r '.id'
}

# Fonction d'ajout de peers à un groupe
add_peers_to_group() {
  local group_id=$1
  shift
  local peers=("$@")
  
  for peer_id in "${peers[@]}"; do
    curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$group_id/peers/$peer_id" \
      -H "Authorization: Bearer $TOKEN"
  done
}

# Exemple d'utilisation
ENGINEERING_GROUP=$(create_group "engineering" "Équipe ingénierie")
add_peers_to_group "$ENGINEERING_GROUP" "peer-1" "peer-2" "peer-3"
```

### Script d'audit des groupes

```bash
#!/bin/bash
# audit-groups.sh

# Lister tous les groupes avec le nombre de membres
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups" \
  | jq -r '.[] | "\(.name): \(.peer_ids | length) membres"'

# Rapport détaillé des groupes
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups" \
  | jq -r '.[] | "Groupe: \(.name)\nMembres: \(.peer_ids | length)\nPolitiques: \(.policy_ids | length)\nRoutes: \(.route_ids | length)\n"'
```

### Script de nettoyage des groupes

```bash
#!/bin/bash
# cleanup-empty-groups.sh

# Trouver et supprimer les groupes sans membres
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups" \
  | jq -r '.[] | select(.peer_ids | length == 0) | .id' \
  | while read group_id; do
      echo "Suppression du groupe vide : $group_id"
      curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$group_id" \
        -H "Authorization: Bearer $TOKEN"
    done
```

## Dépannage

### Problème : Impossible de créer un groupe

**Symptômes :**
- HTTP 400 Bad Request
- Erreur : "group name already exists"

**Solutions :**
1. Vérifier les noms en doublon :
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups" \
  | jq -r '.[].name'
```

2. Utiliser un nom différent ou mettre à jour le groupe existant

### Problème : Impossible d'ajouter un peer à un groupe

**Symptômes :**
- HTTP 404 Not Found
- Le peer n'apparaît pas dans le groupe

**Solutions :**
1. Vérifier que le peer existe :
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/peers/$PEER_ID"
```

2. Vérifier que le groupe existe :
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID"
```

3. Vérifier que l'ID réseau correspond

### Problème : Les politiques ne sont pas appliquées après l'ajout au groupe

**Symptômes :**
- Peer ajouté au groupe mais trafic non filtré
- Règles iptables absentes sur le jump peer

**Solutions :**
1. Vérifier que la politique est attachée au groupe :
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies"
```

2. Vérifier l'état de l'agent du jump peer :
```bash
systemctl status wireguard-agent
```

3. Vérifier iptables sur le jump peer :
```bash
iptables -L -n -v
```

4. Vérifier la connexion WebSocket :
```bash
journalctl -u wireguard-agent | grep -i websocket
```

### Problème : Les routes ne sont pas appliquées après l'ajout au groupe

**Symptômes :**
- Peer ajouté au groupe mais impossible d'atteindre la destination de la route
- AllowedIPs non mis à jour

**Solutions :**
1. Vérifier que la route est attachée au groupe :
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes"
```

2. Vérifier la configuration WireGuard :
```bash
wg show
```

3. Vérifier que AllowedIPs inclut le CIDR de la route

4. Vérifier que le jump peer est en ligne et accessible

## Bonnes pratiques

### 1. Planifier la structure des groupes

Avant de créer des groupes, planifiez :
- Comment organiserez-vous les peers ?
- Quels patterns d'accès avez-vous besoin ?
- Comment les groupes évolueront-ils au fil du temps ?

### 2. Utiliser un nommage cohérent

Établir des conventions de nommage :
- Utiliser des minuscules avec des tirets : `engineering-team`
- Inclure l'objectif : `prod-database-access`
- Éviter les abréviations sauf si elles sont standard

### 3. Documenter l'objectif des groupes

Toujours inclure des descriptions :
- À quoi sert le groupe ?
- Qui devrait en faire partie ?
- Quel accès fournit-il ?

### 4. Audits réguliers

Planifier des révisions régulières :
- Mensuel : Réviser les appartenances aux groupes
- Trimestriel : Auditer les politiques et routes attachées
- Annuel : Restructurer si nécessaire

### 5. Principe du moindre privilège

- Commencer avec un accès minimal
- Ajouter des permissions selon les besoins
- Supprimer les groupes inutilisés
- Réviser et révoquer les accès non nécessaires

### 6. Utiliser les groupes par défaut avec sagesse

- Garder les groupes par défaut simples
- Fournir uniquement l'accès de base
- Ne pas sur-provisionner
- Réviser régulièrement les politiques des groupes par défaut

### 7. Surveiller les changements de groupes

- Journaliser toutes les modifications de groupes
- Suivre qui effectue les changements
- Réviser régulièrement les logs d'audit
- Alerter sur les changements suspects

## Documentation liée

- [Référence API](../api-reference)
