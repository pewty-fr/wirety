# Guide de migration : Groupes, Politiques et Routes

Ce guide vous aide à migrer depuis le système ACL et de flags de peers hérité vers la nouvelle architecture de groupes, politiques et routes.

## Vue d'ensemble

Le système de gestion de réseaux WireGuard a fait l'objet d'une refonte architecturale majeure. Il s'agit d'un **changement non rétrocompatible**.

### Nouveautés

- **Groupes** : Organiser les peers en collections logiques
- **Politiques** : Définir des règles iptables pour le filtrage du trafic sur les jump peers
- **Routes** : Configurer l'accès aux réseaux externes via les jump peers
- **Correspondances DNS** : Résoudre des domaines personnalisés pour les réseaux de routes
- **Groupes par défaut** : Assigner automatiquement des groupes aux peers créés par des non-admins

### Ce qui a été supprimé

- **Système ACL** : Complètement supprimé (tables : `acls`, `acl_rules`)
- **Flags de peers** : Les champs `is_isolated` et `full_encapsulation` ont été supprimés
- **Contrôle d'accès hérité** : Tout le contrôle d'accès passe maintenant par les politiques

---

## Changements non rétrocompatibles

### 1. Suppression du système ACL

**Ce qui a changé :**
- Les tables de base de données `acls` et `acl_rules` ont été supprimées
- Les endpoints API liés aux ACL n'existent plus
- La configuration ACL dans les objets peer et réseau est supprimée

**Chemin de migration :**
Les ACL doivent être recréées manuellement comme politiques. Voir [Conversion des ACL en politiques](#converting-acls-to-policies) ci-dessous.

---

### 2. Suppression des flags de peers

**Ce qui a changé :**
- Le champ `is_isolated` supprimé des peers
- Le champ `full_encapsulation` supprimé des peers
- Ces champs ne sont plus acceptés dans les requêtes API

**Chemin de migration :**
Utiliser les templates de politiques pour obtenir le même comportement :
- `is_isolated: true` → Utiliser le template de politique "isolated"
- `full_encapsulation: true` → Utiliser le template de politique "fully-encapsulated"

Voir [Conversion des flags de peers en politiques](#converting-peer-flags-to-policies) ci-dessous.

---

### 3. Changements d'API

**Endpoints supprimés :**
```
DELETE /api/v1/networks/:networkId/acls/:aclId
POST   /api/v1/networks/:networkId/acls
GET    /api/v1/networks/:networkId/acls
```

**Nouveaux endpoints :**
```
# Groupes
POST   /api/v1/networks/:networkId/groups
GET    /api/v1/networks/:networkId/groups
PUT    /api/v1/networks/:networkId/groups/:groupId
DELETE /api/v1/networks/:networkId/groups/:groupId

# Politiques
POST   /api/v1/networks/:networkId/policies
GET    /api/v1/networks/:networkId/policies
PUT    /api/v1/networks/:networkId/policies/:policyId
DELETE /api/v1/networks/:networkId/policies/:policyId

# Routes
POST   /api/v1/networks/:networkId/routes
GET    /api/v1/networks/:networkId/routes
PUT    /api/v1/networks/:networkId/routes/:routeId
DELETE /api/v1/networks/:networkId/routes/:routeId

# Correspondances DNS
POST   /api/v1/networks/:networkId/routes/:routeId/dns
GET    /api/v1/networks/:networkId/routes/:routeId/dns
```

Consultez la [Référence API](./api-reference) pour la documentation complète.

---

### 4. Changements du modèle Peer

**Avant :**
```json
{
  "id": "peer-1",
  "name": "laptop-1",
  "is_isolated": true,
  "full_encapsulation": false,
  ...
}
```

**Après :**
```json
{
  "id": "peer-1",
  "name": "laptop-1",
  "group_ids": ["group-1"],
  ...
}
```

**Impact :**
- Les requêtes API contenant `is_isolated` ou `full_encapsulation` retourneront HTTP 400
- Les réponses API n'incluent plus ces champs
- Le comportement des peers est maintenant contrôlé par l'appartenance aux groupes et les politiques attachées

---

### 5. Changements du modèle Réseau

**Nouveaux champs :**
```json
{
  "domain_suffix": "internal",
  "default_group_ids": ["group-1", "group-2"]
}
```

**Impact :**
- Les réseaux peuvent maintenant spécifier des suffixes de domaine DNS personnalisés
- Les groupes par défaut sont automatiquement assignés aux peers créés par des non-admins

---

## Étapes de migration

### Étape 1 : Sauvegarder les données

Avant la mise à niveau, sauvegarder la base de données :

```bash
# Sauvegarde PostgreSQL
pg_dump -h localhost -U postgres -d wireguard > backup_$(date +%Y%m%d).sql

# Ou via Docker
docker exec postgres pg_dump -U postgres wireguard > backup_$(date +%Y%m%d).sql
```

### Étape 2 : Documenter la configuration actuelle

Exporter les configurations ACL et peer actuelles :

```bash
# Exporter les ACL
curl -H "Authorization: Bearer $TOKEN" \
  https://votre-serveur/api/v1/networks/$NETWORK_ID/acls > acls_backup.json

# Exporter les peers avec leurs flags
curl -H "Authorization: Bearer $TOKEN" \
  https://votre-serveur/api/v1/networks/$NETWORK_ID/peers > peers_backup.json
```

### Étape 3 : Mettre à niveau le serveur

Déployer la nouvelle version du serveur :

```bash
# Via Docker
docker pull votre-registry/wireguard-server:latest
docker-compose up -d

# Via Kubernetes
kubectl apply -f deployment.yaml
```

La migration de base de données s'exécute automatiquement au démarrage.

### Étape 4 : Recréer le contrôle d'accès

Suivre les guides de conversion ci-dessous pour recréer la configuration de contrôle d'accès avec le nouveau système.

### Étape 5 : Mettre à jour les agents

Mettre à niveau tous les agents de jump peers pour supporter les nouvelles fonctionnalités DNS :

```bash
# Sur chaque jump peer
systemctl stop wireguard-agent
wget https://votre-serveur/downloads/agent-latest
chmod +x agent-latest
mv agent-latest /usr/local/bin/wireguard-agent
systemctl start wireguard-agent
```

### Étape 6 : Vérifier la configuration

Tester la connectivité et vérifier que les politiques fonctionnent correctement :

```bash
# Vérifier la connectivité des peers
ping <peer-ip>

# Vérifier la résolution DNS
nslookup server.route.internal

# Vérifier les règles iptables sur les jump peers
iptables -L -n -v
```

---

## Conversion des ACL en politiques {#converting-acls-to-policies}

### Comprendre la correspondance

**ACL héritée :**
- Appliquée au niveau réseau
- Contrôlait la communication peer-à-peer
- Règles allow/deny simples

**Nouvelles politiques :**
- Appliquées aux groupes
- Génèrent des règles iptables sur les jump peers
- Support de la direction input/output
- Support des cibles CIDR, peer et groupe

### Exemple de conversion

**ACL héritée :**
```json
{
  "name": "allow-internal",
  "rules": [
    {
      "action": "allow",
      "source": "10.0.0.0/24",
      "destination": "10.0.0.0/24"
    }
  ]
}
```

**Nouvelle politique :**
```json
{
  "name": "allow-internal",
  "description": "Autoriser le trafic dans le réseau interne",
  "rules": [
    {
      "direction": "input",
      "action": "allow",
      "target": "10.0.0.0/24",
      "target_type": "cidr",
      "description": "Autoriser entrant depuis le réseau interne"
    },
    {
      "direction": "output",
      "action": "allow",
      "target": "10.0.0.0/24",
      "target_type": "cidr",
      "description": "Autoriser sortant vers le réseau interne"
    }
  ]
}
```

### Script de conversion

Utiliser ce script pour faciliter la conversion des ACL en politiques :

```bash
#!/bin/bash

NETWORK_ID="votre-network-id"
TOKEN="votre-admin-token"
API_URL="https://votre-serveur/api/v1"

# Créer une politique depuis une ACL
create_policy() {
  local acl_name=$1
  local rules=$2
  
  curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"$acl_name\",
      \"description\": \"Converti depuis l'ACL héritée\",
      \"rules\": $rules
    }"
}

# Exemple : Convertir une ACL allow-all
create_policy "allow-all" '[
  {
    "direction": "input",
    "action": "allow",
    "target": "0.0.0.0/0",
    "target_type": "cidr",
    "description": "Autoriser tout entrant"
  },
  {
    "direction": "output",
    "action": "allow",
    "target": "0.0.0.0/0",
    "target_type": "cidr",
    "description": "Autoriser tout sortant"
  }
]'
```

---

## Conversion des flags de peers en politiques {#converting-peer-flags-to-policies}

### Utiliser les templates de politiques

Le système fournit trois templates de politiques intégrés qui remplacent les anciens flags de peers :

#### 1. Peers isolés

**Héritage :**
```json
{
  "is_isolated": true
}
```

**Nouvelle approche :**
1. Obtenir le template "isolated" :
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/policies/templates"
```

2. Créer une politique depuis le template :
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

3. Créer un groupe et attacher la politique :
```bash
# Créer le groupe
GROUP_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "isolated-peers"}' | jq -r '.id')

# Attacher la politique au groupe
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"

# Ajouter le peer au groupe
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

#### 2. Peers en encapsulation complète

**Héritage :**
```json
{
  "full_encapsulation": true
}
```

**Nouvelle approche :**
Utiliser le template "fully-encapsulated" :
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "fully-encapsulated",
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

#### 3. Accès réseau par défaut

**Héritage :**
```json
{
  "is_isolated": false,
  "full_encapsulation": false
}
```

**Nouvelle approche :**
Utiliser le template "default-network" :
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "default-network",
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

### Script de conversion en masse

Convertir tous les peers avec des flags en groupes et politiques :

```bash
#!/bin/bash

NETWORK_ID="votre-network-id"
TOKEN="votre-admin-token"
API_URL="https://votre-serveur/api/v1"

# Créer les politiques depuis les templates
ISOLATED_POLICY_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "isolated", "rules": [...]}' | jq -r '.id')

ENCAPSULATED_POLICY_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "fully-encapsulated", "rules": [...]}' | jq -r '.id')

# Créer les groupes
ISOLATED_GROUP_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "isolated-peers"}' | jq -r '.id')

ENCAPSULATED_GROUP_ID=$(curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "encapsulated-peers"}' | jq -r '.id')

# Attacher les politiques aux groupes
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$ISOLATED_GROUP_ID/policies/$ISOLATED_POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"

curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$ENCAPSULATED_GROUP_ID/policies/$ENCAPSULATED_POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"

# Traiter chaque peer depuis la sauvegarde
jq -c '.[]' peers_backup.json | while read peer; do
  PEER_ID=$(echo $peer | jq -r '.id')
  IS_ISOLATED=$(echo $peer | jq -r '.is_isolated')
  FULL_ENCAP=$(echo $peer | jq -r '.full_encapsulation')
  
  if [ "$IS_ISOLATED" = "true" ]; then
    echo "Ajout de $PEER_ID au groupe isolé"
    curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$ISOLATED_GROUP_ID/peers/$PEER_ID" \
      -H "Authorization: Bearer $TOKEN"
  elif [ "$FULL_ENCAP" = "true" ]; then
    echo "Ajout de $PEER_ID au groupe encapsulé"
    curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$ENCAPSULATED_GROUP_ID/peers/$PEER_ID" \
      -H "Authorization: Bearer $TOKEN"
  fi
done
```

---

## Scénarios de migration courants

### Scénario 1 : Réseau simple avec peers isolés

**Avant :**
- Réseau avec 10 peers
- 3 peers avec `is_isolated: true`
- 7 peers avec les paramètres par défaut

**Après :**
1. Créer une politique "isolated" depuis le template
2. Créer un groupe "isolated-peers"
3. Attacher la politique au groupe
4. Ajouter les 3 peers isolés au groupe

### Scénario 2 : Réseau avec règles ACL

**Avant :**
- Réseau avec ACL autorisant le trafic interne
- ACL refusant le trafic externe

**Après :**
1. Créer une politique "internal-only" avec les règles :
   - Autoriser entrant depuis le CIDR réseau
   - Autoriser sortant vers le CIDR réseau
   - Refuser tout autre trafic
2. Créer un groupe "internal-users"
3. Attacher la politique au groupe
4. Ajouter tous les peers au groupe

### Scénario 3 : Jump peer avec accès externe

**Avant :**
- Jump peer fournissant un accès internet
- Peers regular avec `full_encapsulation: true`

**Après :**
1. Créer une route pour l'accès internet (0.0.0.0/0)
2. Créer une politique "fully-encapsulated"
3. Créer un groupe "internet-users"
4. Attacher la politique et la route au groupe
5. Ajouter les peers regular au groupe

### Scénario 4 : Application multi-niveaux

**Avant :**
- Niveau web : 3 peers
- Niveau app : 5 peers
- Niveau DB : 2 peers (isolés)
- ACL contrôlant la communication entre niveaux

**Après :**
1. Créer des groupes : "web-tier", "app-tier", "db-tier"
2. Créer des politiques :
   - "web-policy" : Autoriser depuis internet, autoriser vers app tier
   - "app-policy" : Autoriser depuis web tier, autoriser vers db tier
   - "db-policy" : Autoriser depuis app tier uniquement
3. Attacher les politiques aux groupes correspondants
4. Ajouter les peers à leurs groupes de niveau

---

## Dépannage

### Problème : Les peers ne peuvent pas communiquer après la migration

**Cause :** Aucune politique attachée aux groupes, comportement de refus par défaut.

**Solution :**
1. Vérifier si les peers sont dans des groupes : `GET /api/v1/networks/:networkId/peers/:peerId`
2. Vérifier si les groupes ont des politiques : `GET /api/v1/networks/:networkId/groups/:groupId/policies`
3. Créer et attacher les politiques appropriées

### Problème : Les domaines de routes ne se résolvent pas

**Cause :** Agent du jump peer non mis à jour ou correspondances DNS non créées.

**Solution :**
1. Vérifier la version de l'agent : `wireguard-agent --version`
2. Vérifier les correspondances DNS : `GET /api/v1/networks/:networkId/routes/:routeId/dns`
3. Vérifier le serveur DNS sur le jump peer : `systemctl status wireguard-agent`

### Problème : Les routes ne fonctionnent pas

**Cause :** Route non attachée au groupe ou jump peer non configuré.

**Solution :**
1. Vérifier l'attachement de la route : `GET /api/v1/networks/:networkId/groups/:groupId/routes`
2. Vérifier la configuration du jump peer : `wg show`
3. Vérifier que AllowedIPs inclut le CIDR de la route

### Problème : L'API retourne 403 Forbidden

**Cause :** Utilisateur non administrateur tentant une opération réservée aux admins.

**Solution :**
1. Vérifier le rôle de l'utilisateur : `GET /api/v1/users/me`
2. Utiliser un compte administrateur pour les opérations sur les groupes/politiques/routes
3. Contacter un administrateur pour effectuer l'opération

---

## Procédure de rollback

Si vous devez revenir à la version précédente :

### Étape 1 : Arrêter le nouveau serveur

```bash
docker-compose down
# ou
kubectl delete deployment wireguard-server
```

### Étape 2 : Restaurer la base de données

```bash
# Restaurer depuis la sauvegarde
psql -h localhost -U postgres -d wireguard < backup_YYYYMMDD.sql

# Ou via Docker
docker exec -i postgres psql -U postgres wireguard < backup_YYYYMMDD.sql
```

### Étape 3 : Déployer la version précédente

```bash
# Via Docker
docker pull votre-registry/wireguard-server:previous-version
docker-compose up -d

# Via Kubernetes
kubectl apply -f deployment-previous.yaml
```

### Étape 4 : Vérifier le fonctionnement

Tester que la version précédente fonctionne correctement avec la base de données restaurée.

---

## Support

En cas de problèmes lors de la migration :

1. Consulter le guide de dépannage (voir barre latérale)
2. Examiner les logs serveur : `docker logs wireguard-server`
3. Vérifier les logs agent sur les jump peers : `journalctl -u wireguard-agent`
4. Ouvrir une issue sur GitHub avec :
   - Version du serveur
   - Logs de migration de la base de données
   - Messages d'erreur
   - Étapes pour reproduire

---

## Ressources supplémentaires

- [Référence API](./api-reference) — Documentation complète de l'API
- [Guide utilisateur](./user-guide) — Guides étape par étape pour les tâches courantes
- Architecture — Vue d'ensemble de l'architecture système
- Dépannage — Problèmes courants et solutions
