---
id: user
title: Guide utilisateur
sidebar_position: 7
---

Ce guide fournit des instructions étape par étape pour les tâches courantes dans le système de gestion de réseaux WireGuard.

## Table des matières

1. [Gestion des groupes](#groups-management)
2. [Gestion des politiques](#policies-management)
3. [Gestion des routes](#routes-management)
4. [Correspondances DNS](#dns-mappings)
5. [Configuration des groupes par défaut](#default-groups-configuration)
6. [Workflows courants](#common-workflows)

---

## Prérequis

- Compte administrateur (toutes les opérations de ce guide nécessitent des privilèges admin)
- Token d'accès API
- ID réseau de votre réseau

### Obtenir votre token API

```bash
# Se connecter et obtenir un token
curl -X POST https://votre-serveur/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "votre-mot-de-passe"}'

# Sauvegarder le token pour une utilisation ultérieure
export TOKEN="votre-token"
export API_URL="https://votre-serveur/api/v1"
export NETWORK_ID="votre-network-id"
```

---

## Gestion des groupes {#groups-management}

Les groupes organisent les peers en collections logiques pour l'application de politiques et de routes.

### Créer un groupe

**Étape 1 :** Définir les détails du groupe

**Étape 2 :** Créer le groupe via l'API

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "engineering-team",
    "description": "Membres de l'\''équipe ingénierie"
  }'
```

**Étape 3 :** Sauvegarder l'ID du groupe depuis la réponse

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

### Ajouter des peers à un groupe

```bash
# Ajouter un seul peer
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"

# Ajouter plusieurs peers (boucle)
for PEER_ID in peer-1 peer-2 peer-3; do
  curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
    -H "Authorization: Bearer $TOKEN"
done
```

### Lister les groupes

```bash
# Lister tous les groupes d'un réseau
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups"
```

### Voir les détails d'un groupe

```bash
# Obtenir un groupe spécifique avec tous ses membres
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID"
```

### Mettre à jour un groupe

```bash
# Mettre à jour le nom et la description
curl -X PUT "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "senior-engineers",
    "description": "Membres seniors de l'\''équipe ingénierie"
  }'
```

### Retirer des peers d'un groupe

```bash
# Retirer un peer d'un groupe
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Supprimer un groupe

```bash
# Supprimer un groupe (les peers ne sont pas supprimés)
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Gestion des politiques {#policies-management}

Les politiques définissent les règles iptables appliquées sur les jump peers pour contrôler le filtrage du trafic.

### Comprendre les règles de politique

Chaque règle de politique possède :
- **Direction** : "input" (trafic entrant) ou "output" (trafic sortant)
- **Action** : "allow" ou "deny"
- **Cible** : IP/CIDR, ID de peer ou ID de groupe
- **Type de cible** : "cidr", "peer" ou "group"

### Créer une politique depuis zéro

**Étape 1 :** Concevoir les règles de la politique

**Étape 2 :** Créer la politique

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
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
        "action": "allow",
        "target": "10.0.0.0/24",
        "target_type": "cidr",
        "description": "Autoriser entrant depuis le réseau"
      }
    ]
  }'
```

### Utiliser les templates de politiques

Le système fournit trois templates intégrés :

**1. Encapsulation complète** — Autoriser sortant, refuser entrant

```bash
# Obtenir les templates
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/policies/templates"

# Créer une politique depuis le template fully-encapsulated
curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-encapsulated-policy",
    "description": "Autoriser sortant, refuser entrant",
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
  }'
```

**2. Isolé** — Refuser tout le trafic

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "isolated-policy",
    "description": "Refuser tout le trafic",
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
  }'
```

**3. Réseau par défaut** — Autoriser le trafic dans le CIDR réseau

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "network-only",
    "description": "Autoriser le trafic dans le réseau",
    "rules": [
      {
        "direction": "input",
        "action": "allow",
        "target": "10.0.0.0/24",
        "target_type": "cidr",
        "description": "Autoriser entrant depuis le réseau"
      },
      {
        "direction": "output",
        "action": "allow",
        "target": "10.0.0.0/24",
        "target_type": "cidr",
        "description": "Autoriser sortant vers le réseau"
      }
    ]
  }'
```

### Attacher une politique à un groupe

```bash
# Attacher la politique au groupe
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

Cela applique automatiquement les règles iptables de la politique sur tous les jump peers pour tous les membres du groupe.

### Gérer les règles d'une politique

**Ajouter une règle à une politique existante :**

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/policies/$POLICY_ID/rules" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "direction": "input",
    "action": "allow",
    "target": "192.168.1.0/24",
    "target_type": "cidr",
    "description": "Autoriser depuis le réseau bureau"
  }'
```

**Supprimer une règle :**

```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/policies/$POLICY_ID/rules/$RULE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Voir les politiques

```bash
# Lister toutes les politiques
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/policies"

# Obtenir une politique spécifique avec ses règles
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/policies/$POLICY_ID"

# Obtenir les politiques attachées à un groupe
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies"
```

### Détacher une politique d'un groupe

```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Supprimer une politique

```bash
# Supprimer une politique (retire de tous les groupes)
curl -X DELETE "$API_URL/networks/$NETWORK_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Gestion des routes {#routes-management}

Les routes définissent les destinations réseau externes accessibles via les jump peers.

### Créer une route

**Étape 1 :** Identifier votre jump peer

```bash
# Lister les peers et trouver les jump peers (is_jump: true)
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/peers"
```

**Étape 2 :** Créer la route

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/routes" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "aws-vpc",
    "description": "Réseau VPC AWS",
    "destination_cidr": "172.31.0.0/16",
    "jump_peer_id": "880e8400-e29b-41d4-a716-446655440000",
    "domain_suffix": "aws.internal"
  }'
```

**Paramètres :**
- `destination_cidr`: Le CIDR du réseau externe (obligatoire)
- `jump_peer_id`: Le jump peer qui route le trafic (obligatoire)
- `domain_suffix`: Suffixe DNS personnalisé (optionnel, défaut : "internal")

### Attacher une route à un groupe

```bash
# Attacher la route au groupe
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

Cela automatiquement :
1. Ajoute le CIDR de la route aux AllowedIPs pour tous les membres du groupe
2. Configure le jump peer comme passerelle
3. Régénère les configurations WireGuard

### Lister les routes

```bash
# Lister toutes les routes du réseau
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/routes"

# Obtenir les routes attachées à un groupe
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes"
```

### Mettre à jour une route

```bash
curl -X PUT "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "aws-vpc-updated",
    "description": "VPC AWS mis à jour",
    "destination_cidr": "172.31.0.0/16",
    "jump_peer_id": "880e8400-e29b-41d4-a716-446655440000",
    "domain_suffix": "aws.internal"
  }'
```

Les mises à jour déclenchent une régénération automatique de la configuration WireGuard pour les peers affectés.

### Détacher une route d'un groupe

```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Supprimer une route

```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Correspondances DNS {#dns-mappings}

Les correspondances DNS fournissent la résolution de noms pour les adresses IP au sein des réseaux de routes.

### Créer une correspondance DNS

**Étape 1 :** S'assurer d'avoir une route créée

**Étape 2 :** Créer la correspondance DNS (l'IP doit être dans le CIDR de la route)

```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "database-server",
    "ip_address": "172.31.10.50"
  }'
```

**Format FQDN :** `database-server.aws-vpc.aws.internal`
- `database-server`: Le nom que vous avez spécifié
- `aws-vpc`: Le nom de la route
- `aws.internal`: Le suffixe de domaine de la route

### Lister les correspondances DNS

```bash
# Lister les correspondances DNS d'une route
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns"

# Obtenir tous les enregistrements DNS du réseau (peers + routes)
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/dns"
```

### Mettre à jour une correspondance DNS

```bash
curl -X PUT "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns/$DNS_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "db-primary",
    "ip_address": "172.31.10.51"
  }'
```

Les changements se propagent aux serveurs DNS des jump peers dans les 60 secondes.

### Supprimer une correspondance DNS

```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns/$DNS_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Tester la résolution DNS

Depuis un peer du réseau :

```bash
# Tester la résolution DNS
nslookup database-server.aws-vpc.aws.internal

# Ou avec dig
dig database-server.aws-vpc.aws.internal
```

---

## Configuration des groupes par défaut {#default-groups-configuration}

Les groupes par défaut sont automatiquement assignés aux peers créés par des utilisateurs non administrateurs.

### Configurer les groupes par défaut

**Étape 1 :** Créer des groupes pour l'assignation par défaut

```bash
# Créer un groupe "standard-users"
curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "standard-users",
    "description": "Groupe par défaut pour tous les utilisateurs"
  }'
```

**Étape 2 :** Attacher des politiques et routes au groupe

```bash
# Attacher une politique
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"

# Attacher une route
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

**Étape 3 :** Configurer le groupe comme groupe par défaut pour le réseau

```bash
curl -X PUT "$API_URL/networks/$NETWORK_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "default_group_ids": ["'$GROUP_ID'"]
  }'
```

### Fonctionnement des groupes par défaut

- **Un non-admin crée un peer** : Automatiquement ajouté à tous les groupes par défaut
- **Un admin crée un peer** : NON automatiquement ajouté aux groupes par défaut
- **Peers existants** : Non affectés par les changements de configuration des groupes par défaut

### Voir les groupes par défaut

```bash
# Obtenir les détails du réseau incluant les groupes par défaut
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID"
```

### Supprimer la configuration des groupes par défaut

```bash
# Vider les groupes par défaut
curl -X PUT "$API_URL/networks/$NETWORK_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "default_group_ids": []
  }'
```

---

## Workflows courants {#common-workflows}

### Workflow 1 : Configurer une nouvelle équipe {#workflow-1-setting-up-a-new-team}

**Objectif :** Créer un groupe pour l'équipe ingénierie avec accès internet et accès au réseau interne.

**Étapes :**

1. Créer le groupe :
```bash
GROUP_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "engineering", "description": "Équipe ingénierie"}' \
  | jq -r '.id')
```

2. Créer une politique pour l'accès internet :
```bash
POLICY_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "internet-access",
    "rules": [
      {"direction": "output", "action": "allow", "target": "0.0.0.0/0", "target_type": "cidr"},
      {"direction": "input", "action": "allow", "target": "10.0.0.0/24", "target_type": "cidr"}
    ]
  }' | jq -r '.id')
```

3. Créer une route pour le réseau bureau :
```bash
ROUTE_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/routes" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "office-network",
    "destination_cidr": "192.168.1.0/24",
    "jump_peer_id": "'$JUMP_PEER_ID'"
  }' | jq -r '.id')
```

4. Attacher la politique et la route au groupe :
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"

curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

5. Ajouter les membres de l'équipe au groupe :
```bash
for PEER_ID in peer-1 peer-2 peer-3; do
  curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
    -H "Authorization: Bearer $TOKEN"
done
```

### Workflow 2 : Isoler un peer compromis {#workflow-2-isolating-a-compromised-peer}

**Objectif :** Isoler immédiatement un peer potentiellement compromis.

**Étapes :**

1. Créer une politique d'isolation (si elle n'existe pas) :
```bash
ISOLATED_POLICY_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "isolated",
    "rules": [
      {"direction": "input", "action": "deny", "target": "0.0.0.0/0", "target_type": "cidr"},
      {"direction": "output", "action": "deny", "target": "0.0.0.0/0", "target_type": "cidr"}
    ]
  }' | jq -r '.id')
```

2. Créer un groupe de quarantaine :
```bash
QUARANTINE_GROUP_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "quarantine", "description": "Peers isolés"}' \
  | jq -r '.id')
```

3. Attacher la politique d'isolation :
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$QUARANTINE_GROUP_ID/policies/$ISOLATED_POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

4. Déplacer le peer dans le groupe de quarantaine :
```bash
# Retirer des groupes actuels
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$OLD_GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"

# Ajouter à la quarantaine
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$QUARANTINE_GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Workflow 3 : Fournir un accès aux ressources cloud {#workflow-3-providing-access-to-cloud-resources}

**Objectif :** Donner à un groupe un accès à un VPC AWS avec résolution DNS.

**Étapes :**

1. Créer une route vers le VPC AWS :
```bash
ROUTE_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/routes" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "aws-vpc",
    "destination_cidr": "172.31.0.0/16",
    "jump_peer_id": "'$JUMP_PEER_ID'",
    "domain_suffix": "aws.internal"
  }' | jq -r '.id')
```

2. Créer des correspondances DNS pour les serveurs importants :
```bash
# Serveur base de données
curl -X POST "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "database", "ip_address": "172.31.10.50"}'

# Serveur web
curl -X POST "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "web", "ip_address": "172.31.20.100"}'
```

3. Attacher la route au groupe :
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

4. Tester la résolution DNS depuis un peer :
```bash
nslookup database.aws-vpc.aws.internal
nslookup web.aws-vpc.aws.internal
```

### Workflow 4 : Sécurité d'application multi-niveaux {#workflow-4-multi-tier-application-security}

**Objectif :** Configurer des niveaux web, app et base de données avec des contrôles d'accès appropriés.

**Étapes :**

1. Créer des groupes pour chaque niveau :
```bash
WEB_GROUP=$(curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name": "web-tier"}' | jq -r '.id')

APP_GROUP=$(curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name": "app-tier"}' | jq -r '.id')

DB_GROUP=$(curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name": "db-tier"}' | jq -r '.id')
```

2. Créer des politiques pour chaque niveau :
```bash
# Niveau web : Autoriser depuis internet, autoriser vers app tier
WEB_POLICY=$(curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "web-policy",
    "rules": [
      {"direction": "input", "action": "allow", "target": "0.0.0.0/0", "target_type": "cidr"},
      {"direction": "output", "action": "allow", "target": "'$APP_GROUP'", "target_type": "group"}
    ]
  }' | jq -r '.id')

# Niveau app : Autoriser depuis web, autoriser vers db
APP_POLICY=$(curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "app-policy",
    "rules": [
      {"direction": "input", "action": "allow", "target": "'$WEB_GROUP'", "target_type": "group"},
      {"direction": "output", "action": "allow", "target": "'$DB_GROUP'", "target_type": "group"}
    ]
  }' | jq -r '.id')

# Niveau db : Autoriser uniquement depuis app
DB_POLICY=$(curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "db-policy",
    "rules": [
      {"direction": "input", "action": "allow", "target": "'$APP_GROUP'", "target_type": "group"}
    ]
  }' | jq -r '.id')
```

3. Attacher les politiques aux groupes :
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$WEB_GROUP/policies/$WEB_POLICY" \
  -H "Authorization: Bearer $TOKEN"

curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$APP_GROUP/policies/$APP_POLICY" \
  -H "Authorization: Bearer $TOKEN"

curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$DB_GROUP/policies/$DB_POLICY" \
  -H "Authorization: Bearer $TOKEN"
```

4. Ajouter les peers aux niveaux appropriés :
```bash
# Ajouter les serveurs web
for PEER in web-1 web-2; do
  curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$WEB_GROUP/peers/$PEER" \
    -H "Authorization: Bearer $TOKEN"
done

# Ajouter les serveurs app
for PEER in app-1 app-2 app-3; do
  curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$APP_GROUP/peers/$PEER" \
    -H "Authorization: Bearer $TOKEN"
done

# Ajouter les serveurs base de données
for PEER in db-1 db-2; do
  curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$DB_GROUP/peers/$PEER" \
    -H "Authorization: Bearer $TOKEN"
done
```

---

## Bonnes pratiques

### Organisation des groupes

1. **Utiliser des noms descriptifs** : "engineering-team" et non "group1"
2. **Documenter l'objectif** : Utiliser le champ description
3. **Garder les groupes ciblés** : Un objectif par groupe
4. **Planifier la hiérarchie** : Considérer les patterns d'accès imbriqués

### Conception des politiques

1. **Commencer restrictif** : Partir d'un tout-refuser, ajouter des autorisations selon les besoins
2. **Utiliser les templates** : Tirer parti des templates intégrés pour les patterns courants
3. **Documenter les règles** : Ajouter des descriptions à chaque règle
4. **Tester par incréments** : Ajouter une règle à la fois et tester
5. **L'ordre a de l'importance** : Les règles sont appliquées dans l'ordre d'attachement

### Gestion des routes

1. **Vérifier les CIDR** : Vérifier attentivement la notation CIDR avant de créer
2. **Utiliser des noms significatifs** : "aws-vpc-prod" et non "route1"
3. **Documenter les jump peers** : Noter quel jump peer sert quelle route
4. **Surveiller la capacité** : S'assurer que les jump peers peuvent gérer le trafic

### Correspondances DNS

1. **Utiliser un nommage cohérent** : Suivre une convention de nommage
2. **Documenter les IPs** : Tenir une documentation externe des correspondances
3. **Vérifier le CIDR** : S'assurer que les IPs sont dans le CIDR de la route
4. **Tester la résolution** : Toujours tester le DNS après la création de correspondances

### Groupes par défaut

1. **Garder la simplicité** : Commencer avec un groupe par défaut
2. **Accès de base** : Fournir l'accès minimum nécessaire
3. **Réviser régulièrement** : Auditer les politiques des groupes par défaut trimestriellement
4. **Communiquer** : Informer les utilisateurs de l'assignation automatique aux groupes

---

## Dépannage

### Groupes

**Problème :** Impossible d'ajouter un peer à un groupe
- **Vérifier :** Le peer existe dans le réseau
- **Vérifier :** Vous avez les privilèges admin
- **Vérifier :** Le groupe existe dans le même réseau

**Problème :** La suppression du groupe échoue
- **Vérifier :** Pas de contraintes de clés étrangères
- **Vérifier :** Le groupe existe
- **Solution :** Détacher les politiques et routes d'abord

### Politiques

**Problème :** La politique ne prend pas effet
- **Vérifier :** La politique est attachée au groupe
- **Vérifier :** Le peer est membre du groupe
- **Vérifier :** L'agent du jump peer est en cours d'exécution
- **Solution :** Vérifier iptables sur le jump peer : `iptables -L -n -v`

**Problème :** Trafic bloqué de façon inattendue
- **Vérifier :** L'ordre des règles de politique
- **Vérifier :** Le comportement de refus par défaut
- **Solution :** Ajouter une règle d'autorisation explicite

### Routes

**Problème :** Impossible d'atteindre la destination de la route
- **Vérifier :** La route est attachée au groupe du peer
- **Vérifier :** Le jump peer est en ligne
- **Vérifier :** La configuration WireGuard inclut le CIDR de la route
- **Solution :** Vérifier AllowedIPs : `wg show`

**Problème :** La création de route échoue
- **Vérifier :** Le format CIDR est valide
- **Vérifier :** Le jump peer existe et a `is_jump=true`
- **Vérifier :** Pas de noms de route en doublon

### DNS

**Problème :** Le DNS ne se résout pas
- **Vérifier :** La correspondance DNS existe
- **Vérifier :** L'IP est dans le CIDR de la route
- **Vérifier :** Le serveur DNS du jump peer fonctionne
- **Solution :** Vérifier les logs du serveur DNS sur le jump peer

**Problème :** Mauvaise IP retournée
- **Vérifier :** La correspondance DNS est correcte
- **Vérifier :** Le cache DNS (vider avec `systemd-resolve --flush-caches`)
- **Solution :** Mettre à jour la correspondance DNS

---

## Ressources supplémentaires

- [Référence API](./api-reference) — Documentation complète de l'API
- [Guide de migration](./migration-guide) — Migrer depuis le système hérité
- Architecture — Vue d'ensemble de l'architecture système
- Dépannage — Guide de dépannage détaillé

---

## Obtenir de l'aide

Si vous avez besoin d'assistance :

1. Consulter ce guide et la section dépannage
2. Examiner les logs serveur : `docker logs wireguard-server`
3. Vérifier les logs agent : `journalctl -u wireguard-agent`
4. Ouvrir une issue sur GitHub avec :
   - Ce que vous essayez de faire
   - Ce que vous attendiez qu'il se passe
   - Ce qui s'est réellement passé
   - Logs et messages d'erreur pertinents
