# Aide-mémoire

Référence rapide pour les opérations courantes dans le système de gestion de réseaux WireGuard.

## Configuration de l'environnement

```bash
export TOKEN="votre-admin-token"
export API_URL="https://votre-serveur/api/v1"
export NETWORK_ID="votre-network-id"
```

## Groupes

### Créer un groupe
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "nom-du-groupe", "description": "Description"}'
```

### Lister les groupes
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups"
```

### Ajouter un peer à un groupe
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Retirer un peer d'un groupe
```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Supprimer un groupe
```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID" \
  -H "Authorization: Bearer $TOKEN"
```

## Politiques

### Créer une politique
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "nom-politique",
    "description": "Description",
    "rules": [
      {
        "direction": "output",
        "action": "allow",
        "target": "0.0.0.0/0",
        "target_type": "cidr",
        "description": "Autoriser tout sortant"
      }
    ]
  }'
```

### Obtenir les templates de politiques
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/policies/templates"
```

### Attacher une politique à un groupe
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Détacher une politique d'un groupe
```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies/$POLICY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Ajouter une règle à une politique
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/policies/$POLICY_ID/rules" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "direction": "input",
    "action": "allow",
    "target": "10.0.0.0/24",
    "target_type": "cidr",
    "description": "Autoriser depuis le réseau"
  }'
```

## Routes

### Créer une route
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/routes" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "nom-route",
    "description": "Description",
    "destination_cidr": "172.31.0.0/16",
    "jump_peer_id": "jump-peer-id",
    "domain_suffix": "internal"
  }'
```

### Lister les routes
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/routes"
```

### Attacher une route à un groupe
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Détacher une route d'un groupe
```bash
curl -X DELETE "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

## Correspondances DNS

### Créer une correspondance DNS
```bash
curl -X POST "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "nom-serveur",
    "ip_address": "172.31.10.50"
  }'
```

### Lister les correspondances DNS d'une route
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns"
```

### Obtenir tous les enregistrements DNS du réseau
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/dns"
```

## Groupes par défaut

### Configurer les groupes par défaut
```bash
curl -X PUT "$API_URL/networks/$NETWORK_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "default_group_ids": ["group-id-1", "group-id-2"]
  }'
```

## Patterns courants

### Politique d'encapsulation complète
```json
{
  "name": "fully-encapsulated",
  "rules": [
    {
      "direction": "output",
      "action": "allow",
      "target": "0.0.0.0/0",
      "target_type": "cidr"
    },
    {
      "direction": "input",
      "action": "deny",
      "target": "0.0.0.0/0",
      "target_type": "cidr"
    }
  ]
}
```

### Politique d'isolation
```json
{
  "name": "isolated",
  "rules": [
    {
      "direction": "input",
      "action": "deny",
      "target": "0.0.0.0/0",
      "target_type": "cidr"
    },
    {
      "direction": "output",
      "action": "deny",
      "target": "0.0.0.0/0",
      "target_type": "cidr"
    }
  ]
}
```

### Politique réseau uniquement
```json
{
  "name": "network-only",
  "rules": [
    {
      "direction": "input",
      "action": "allow",
      "target": "10.0.0.0/24",
      "target_type": "cidr"
    },
    {
      "direction": "output",
      "action": "allow",
      "target": "10.0.0.0/24",
      "target_type": "cidr"
    }
  ]
}
```

## Diagnostics

### Vérifier les détails d'un groupe
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID" | jq
```

### Vérifier les politiques d'un groupe
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/policies" | jq
```

### Vérifier les routes d'un groupe
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/routes" | jq
```

### Vérifier iptables sur le jump peer
```bash
ssh jump-peer "sudo iptables -L -n -v"
```

### Vérifier la configuration WireGuard
```bash
ssh peer "sudo wg show"
```

### Vérifier la résolution DNS
```bash
ssh peer "nslookup server.route.internal"
```

### Vérifier l'état de l'agent
```bash
ssh jump-peer "sudo systemctl status wireguard-agent"
```

### Vérifier les logs de l'agent
```bash
ssh jump-peer "sudo journalctl -u wireguard-agent -f"
```

## Opérations en masse

### Ajouter plusieurs peers à un groupe
```bash
for PEER_ID in peer-1 peer-2 peer-3; do
  curl -X POST "$API_URL/networks/$NETWORK_ID/groups/$GROUP_ID/peers/$PEER_ID" \
    -H "Authorization: Bearer $TOKEN"
done
```

### Créer plusieurs correspondances DNS
```bash
declare -A servers=(
  ["web"]="172.31.10.10"
  ["app"]="172.31.10.20"
  ["db"]="172.31.10.30"
)

for name in "${!servers[@]}"; do
  curl -X POST "$API_URL/networks/$NETWORK_ID/routes/$ROUTE_ID/dns" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"name\": \"$name\", \"ip_address\": \"${servers[$name]}\"}"
done
```

## Codes d'erreur

- `200 OK` - Succès
- `201 Created` - Ressource créée
- `204 No Content` - Succès (pas de corps de réponse)
- `400 Bad Request` - Requête invalide
- `403 Forbidden` - Non autorisé (admin requis)
- `404 Not Found` - Ressource introuvable
- `409 Conflict` - Nom en doublon ou violation de contrainte
- `500 Internal Server Error` - Erreur serveur

## Champs des règles de politique

- **direction**: `input` ou `output`
- **action**: `allow` ou `deny`
- **target**: IP/CIDR, ID de peer ou ID de groupe
- **target_type**: `cidr`, `peer` ou `group`

## Format FQDN DNS

`nom.nom-route.suffixe-domaine`

Exemple : `database.aws-vpc.aws.internal`

## Documentation liée

- [Référence API](/api-reference.md)
- [Guide utilisateur](/guides/user.md)
- [Gestion des groupes](/guides/groups-management.md)
