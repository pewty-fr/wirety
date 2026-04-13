---
id: troubleshooting
title: Dépannage
sidebar_position: 9
---

Problèmes courants et solutions lors de l'utilisation de Wirety.

## Problèmes serveur

### Le serveur ne démarre pas

**Symptômes :**
- Le pod/conteneur plante immédiatement
- Les logs d'erreur indiquent des problèmes de configuration

**Solutions :**

1. **Vérifier les variables d'environnement**
```bash
kubectl logs -n wirety <server-pod-name>
# ou
docker logs wirety-server
```

2. **Vérifier la configuration OIDC**
```bash
# Si AUTH_ENABLED=true, s'assurer que toutes les variables d'auth sont définies :
# - AUTH_ISSUER_URL
# - AUTH_CLIENT_ID
# - AUTH_CLIENT_SECRET
```

3. **Vérifier la disponibilité du port**
```bash
# S'assurer que HTTP_PORT n'est pas déjà utilisé
netstat -tlnp | grep 8080
```

### L'authentification ne fonctionne pas

**Symptômes :**
- Les redirections de connexion échouent
- Erreurs 401 Unauthorized
- Erreurs de récupération JWKS

**Solutions :**

1. **Vérifier l'accessibilité du fournisseur OIDC**
```bash
curl https://auth.example.com/realms/wirety/.well-known/openid-configuration
```

2. **Vérifier les URI de redirection**
   - S'assurer que `https://wirety.example.com/api/v1/auth/callback` est enregistré dans le fournisseur OIDC

3. **Vérifier la synchronisation horaire**
```bash
# La validation du token échoue si les horloges sont décalées
timedatectl status
ntpdate -q pool.ntp.org
```

4. **Vérifier le cache JWKS**
   - Redémarrer le serveur si le JWKS est périmé
   - Ajuster `AUTH_JWKS_CACHE_TTL` si nécessaire

### La connexion WebSocket échoue

**Symptômes :**
- Les agents ne reçoivent pas les mises à jour en temps réel
- Le frontend affiche des avertissements de déconnexion

**Solutions :**

1. **Vérifier la configuration du reverse proxy**
```nginx
# Nginx a besoin des en-têtes d'upgrade WebSocket
location /api {
    proxy_pass http://backend;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
```

2. **Vérifier les règles de pare-feu**
   - S'assurer que les connexions WebSocket ne sont pas bloquées

## Problèmes d'agent

### L'inscription de l'agent échoue

**Symptômes :**
- Erreur : "enrollment failed"
- Erreurs HTTP 401/403
- Timeout de connexion

**Solutions :**

1. **Vérifier la validité du token**
   - Générer un nouveau token depuis l'interface
   - Vérifier que le token n'a pas expiré ou été révoqué

2. **Vérifier l'accessibilité de l'URL du serveur**
```bash
curl -v https://wirety.example.com/api/v1/health
```

3. **Vérifier que WireGuard est installé**
```bash
which wg
modprobe wireguard
```

4. **Vérifier les logs de l'agent**
```bash
journalctl -u wirety-agent -f
# ou
./wirety-agent -server https://wirety.example.com -token <token>
```

### La configuration de l'agent ne se met pas à jour

**Symptômes :**
- Les changements dans l'interface ne se reflètent pas sur l'agent
- La configuration WireGuard reste ancienne

**Solutions :**

1. **Vérifier que l'agent est en cours d'exécution**
```bash
systemctl status wirety-agent
ps aux | grep wirety-agent
```

2. **Vérifier la connexion WebSocket**
   - L'agent doit se reconnecter après des problèmes réseau
   - Vérifier les logs pour des erreurs WebSocket

3. **Rafraîchissement manuel de la configuration**
```bash
# Redémarrer l'agent pour forcer le rafraîchissement
systemctl restart wirety-agent
```

4. **Vérifier le blocage ACL**
   - Le peer peut être bloqué suite à un incident
   - Résoudre l'incident dans l'interface

### Problèmes d'interface WireGuard

**Symptômes :**
- Interface `wg0` non créée
- Interface existante mais sans connectivité

**Solutions :**

1. **Vérifier le module WireGuard**
```bash
lsmod | grep wireguard
modprobe wireguard
```

2. **Vérifier les permissions**
```bash
# L'agent a besoin de CAP_NET_ADMIN ou des droits root
sudo -u wirety-agent wg show
```

3. **Vérifier la syntaxe de la configuration**
```bash
wg-quick up /etc/wireguard/wg0.conf
# Vérifier les erreurs de syntaxe
```

4. **Vérifier le routage**
```bash
ip route | grep wg0
# Les routes pour les IP autorisées doivent être visibles
```

## Problèmes de connectivité

### Impossible de pinger les autres peers

**Symptômes :**
- Timeouts de ping entre peers
- `wg show` ne montre pas de handshake

**Solutions :**

1. **Vérifier que le peer n'est pas isolé**
   - Vérifier l'appartenance aux groupes et les politiques attachées dans l'interface
   - Les peers isolés ne peuvent atteindre que les jump peers

2. **Vérifier les IP autorisées**
```bash
wg show wg0 allowed-ips
# L'IP du peer cible doit être incluse
```

3. **Vérifier l'accessibilité de l'endpoint**
```bash
# Tester la connectivité UDP vers l'endpoint du peer
nc -u -v <peer-ip> <peer-port>
```

4. **Vérifier le NAT / pare-feu**
   - S'assurer que le port WireGuard est ouvert (défaut : 51820)
   - Configurer le port forwarding si derrière NAT

5. **Keepalive persistant**
   - Pour la traversée NAT, ajouter un keepalive persistant :
   ```
   PersistentKeepalive = 25
   ```

### Le routage du jump peer ne fonctionne pas

**Symptômes :**
- Encapsulation complète activée mais le trafic n'est pas routé
- Peut atteindre le jump peer mais pas internet

**Solutions :**

1. **Vérifier que le NAT est configuré**
```bash
# Sur le jump peer
iptables -t nat -L POSTROUTING -n -v
# Une règle MASQUERADE pour wg0 doit être visible
```

2. **Activer le forwarding IP**
```bash
# Sur le jump peer
sysctl net.ipv4.ip_forward
# Doit être à 1
echo 1 > /proc/sys/net/ipv4/ip_forward
```

3. **Vérifier les IP supplémentaires autorisées**
   - Vérifier que `0.0.0.0/0` est dans AllowedIPs pour l'encapsulation complète

4. **Vérifier la route par défaut**
```bash
# Sur le peer regular avec encapsulation complète
ip route
# La route par défaut doit pointer vers wg0
```

### Incidents de conflit de session

**Symptômes :**
- Peer automatiquement bloqué
- Type d'incident : "session conflict"

**Solutions :**

1. **Vérifier les agents en double**
   - S'assurer qu'il n'y a qu'une seule instance d'agent par peer
   - Arrêter les agents en double

2. **Revoir le processus d'inscription**
   - Ne pas réutiliser les tokens sur plusieurs hôtes
   - Chaque hôte a besoin d'une entrée peer unique

3. **Résoudre l'incident**
   - Une fois le doublon supprimé, résoudre l'incident dans l'interface
   - Le peer recevra à nouveau les mises à jour de configuration

## Problèmes IPAM

### L'allocation d'IP échoue

**Symptômes :**
- Erreur : "no available IPs"
- La création du peer échoue

**Solutions :**

1. **Vérifier la capacité du réseau**
   - Voir le réseau dans l'interface pour vérifier la capacité
   - Étendre le CIDR si nécessaire (uniquement s'il n'y a pas de peers statiques)

2. **Supprimer les peers inutilisés**
   - Libérer des IP en supprimant les anciens peers

### Impossible de modifier le CIDR du réseau

**Symptômes :**
- Erreur : "static peers exist"

**Solutions :**

1. **Convertir les peers statiques en dynamiques**
   - Ou supprimer les peers statiques avant le changement de CIDR
   - Les peers statiques nécessitent une reconfiguration manuelle

2. **Créer un nouveau réseau**
   - Alternative : créer un nouveau réseau avec le CIDR souhaité
   - Migrer les peers progressivement

## Problèmes de performance

### Latence élevée

**Symptômes :**
- Temps de réponse lents
- Perte de paquets

**Solutions :**

1. **Vérifier les paramètres MTU**
```bash
# MTU typique pour WireGuard
ip link set wg0 mtu 1420
```

2. **Optimiser le routage**
   - Utiliser des connexions directes entre peers lorsque possible
   - Minimiser les sauts via les jump peers

3. **Vérifier la bande passante**
```bash
# Surveiller le trafic WireGuard
watch -n 1 wg show wg0 transfer
```

### Utilisation CPU élevée de l'agent

**Symptômes :**
- Agent consommant un CPU excessif
- Ralentissement du système

**Solutions :**

1. **Vérifier l'intervalle de polling**
   - L'agent sonde les mises à jour
   - Vérifier l'absence de boucles de reconnexion excessives

2. **Examiner les logs**
```bash
journalctl -u wirety-agent | grep -i error
```

3. **Mettre à jour vers la dernière version**
   - Les corrections de bugs peuvent améliorer les performances

## Problèmes de base de données / stockage

### Données en mémoire perdues au redémarrage

**Symptômes :**
- Tous les peers/réseaux disparus après le redémarrage du serveur

**Solution :**
- C'est attendu avec le stockage en mémoire par défaut
- Activer PostgreSQL avec `DB_ENABLED=true` et `DB_DSN` pour la persistance
- Exporter régulièrement les configurations comme sauvegarde

## Problèmes de sécurité

### Changements d'endpoint déclenchant des incidents

**Symptômes :**
- Plusieurs incidents de changement d'endpoint
- Peer automatiquement bloqué

**Solutions :**

1. **Utiliser des endpoints statiques pour les jump peers**
   - Configurer une IP publique fixe
   - Utiliser DNS avec un TTL court si l'IP change

2. **Revoir les seuils de détection**
   - 30 minutes pour la détection de configuration partagée
   - 10 changements/jour pour l'activité suspecte

3. **Résoudre les incidents légitimes**
   - Si légitime (ex. changements réseau), résoudre dans l'interface

## Journalisation et débogage

### Activer les logs verbeux

**Serveur :**
```bash
# Ajouter la variable d'environnement
LOG_LEVEL=debug
```

**Agent :**
```bash
# Exécuter en premier plan avec sortie verbeuse
wirety-agent -server https://wirety.example.com -token <token> -log-level debug
```

### Collecter des informations de diagnostic

```bash
# État WireGuard
wg show all dump

# État de l'agent
systemctl status wirety-agent
journalctl -u wirety-agent -n 100

# Connectivité réseau
ip route
ip addr
iptables -t nat -L -n -v

# Résolution DNS
nslookup wirety.example.com

# Logs serveur
kubectl logs -n wirety <server-pod> --tail=100
```

## Obtenir de l'aide

Si les problèmes persistent :

1. **Consulter les Issues GitHub**
   - Rechercher des problèmes similaires
   - Examiner les issues fermées pour des solutions

2. **Ouvrir une Issue**
   - Fournir une description claire
   - Inclure les logs pertinents
   - Préciser les versions et l'environnement

3. **Inclure les diagnostics**
   ```bash
   # Collecter les informations pertinentes
   wg show > wg-status.txt
   journalctl -u wirety-agent > agent-logs.txt
   kubectl logs -n wirety <server-pod> > server-logs.txt
   ```

## Documentation liée

- [Configuration de l'agent](./agent)
- [Configuration du serveur](./server)
- [Configuration réseau](./network)
- [Guide OIDC](./guides/oidc)
