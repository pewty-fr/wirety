---
id: agent
title: Agent
sidebar_position: 6
---

L'agent Wirety automatise la récupération de configuration et les rapports d'état.

## Responsabilités
- Inscription via token.
- Maintenir la configuration WireGuard à jour (poll / déclenchement WebSocket).
- Heartbeat : hostname, uptime, observation d'endpoint, horodatages.

## Options CLI
```bash
wirety-agent [options]

Options:
  -server string
        URL de base du serveur (sans / final)
        (env: SERVER_URL, défaut: http://localhost:8080)
  -token string
        Token d'inscription (obligatoire)
        (env: TOKEN)
  -config string
        Chemin vers le fichier de configuration WireGuard
        (env: WG_CONFIG_PATH)
  -apply string
        Méthode d'application : wg-quick|syncconf
        (env: WG_APPLY_METHOD, défaut: syncconf)
  -nat-interfaces string
        Liste d'interfaces NAT séparées par des virgules (env: NAT_INTERFACES)
        Défaut : détection automatique de toutes les interfaces de sortie depuis la table de routage
  -portal-url string
        URL de la page du portail captif
        (env: CAPTIVE_PORTAL_URL, défaut: <SERVER_URL>/captive-portal)
  -server-host string
        Remplace l'en-tête HTTP Host pour toutes les requêtes vers le serveur
        (env: SERVER_HOST, défaut: dérivé de l'URL -server)
        Utile pour accéder au serveur Wirety par IP derrière un reverse proxy
        qui route par hostname (ex. SERVER_URL=http://10.0.0.7 SERVER_HOST=wirety.internal)
  -skip-tls-verify
        Désactive la vérification du certificat TLS pour les connexions au serveur
        (env: SKIP_TLS_VERIFY, défaut: false)
        À utiliser uniquement si le serveur utilise un certificat auto-signé ou signé par une CA interne
        que l'hôte de l'agent ne peut pas vérifier. Ne jamais utiliser en production avec des certificats publics.
  -log-level string
        Verbosité des logs : trace|debug|info|warn|error|fatal
        (env: LOG_LEVEL, défaut: info)
  -log-format string
        Format de sortie des logs : text|json
        (env: LOG_FORMAT, défaut: text)
  -audit-log
        Émet des événements d'audit JSON sur stdout
        (env: AUDIT_LOG, défaut: false)
```

## Exemple d'utilisation
```bash
# Lancer l'agent — les interfaces NAT sont détectées automatiquement depuis la table de routage
export TOKEN=<ENROLLMENT_TOKEN>
export SERVER_URL=https://wirety.example.com
wirety-agent

# Interfaces NAT explicites (jump peer avec sortie internet + VLAN privé)
wirety-agent -server https://wirety.example.com -token <TOKEN> -nat-interfaces ens2,ens6

# Équivalent en variables d'environnement
export NAT_INTERFACES=ens2,ens6
wirety-agent

# Accéder au serveur par IP (sans DNS) avec en-tête Host pour le routage reverse proxy
wirety-agent -server http://10.0.0.7 -server-host wirety.internal -token <TOKEN>

# Équivalent en variables d'environnement
export SERVER_URL=http://10.0.0.7
export SERVER_HOST=wirety.internal
wirety-agent

# Reverse proxy avec certificat auto-signé (désactiver la vérification TLS)
wirety-agent \
  -server https://10.0.0.1 \
  -server-host wirety.internal \
  -portal-url https://wirety.internal/captive-portal \
  -skip-tls-verify \
  -token <TOKEN>

# Logs JSON structurés au niveau debug (utile avec des agrégateurs comme Loki / Datadog)
wirety-agent -log-format json -log-level debug

# Sortie minimale en production (avertissements et erreurs uniquement)
wirety-agent -log-level warn

# Activer le log d'audit avec le fonctionnement normal
wirety-agent -audit-log -log-format json

# Équivalent en variables d'environnement
export LOG_FORMAT=json
export LOG_LEVEL=debug
export AUDIT_LOG=true
wirety-agent
```

## Reverse Proxy / Accès sans DNS (`SERVER_HOST`)

Lorsque le serveur Wirety est déployé dans un réseau privé derrière un reverse proxy et qu'aucun DNS interne n'est disponible, l'agent peut atteindre le serveur par IP tout en envoyant le bon en-tête `Host` pour que le proxy route la requête.

```bash
# Sans DNS : connexion à 10.0.0.7:80, mais envoi de "Host: wirety.internal"
# pour que Nginx / Caddy / Traefik route la requête vers le backend Wirety.
wirety-agent \
  -server http://10.0.0.7 \
  -server-host wirety.internal \
  -token <TOKEN>
```

C'est équivalent à :
```bash
curl http://10.0.0.7/api/v1/agent/resolve \
  -H "Host: wirety.internal" \
  -H "Authorization: Bearer <TOKEN>"
```

Le remplacement `SERVER_HOST` est appliqué à **toutes** les connexions sortantes de l'agent :
- Résolution initiale du token (`/api/v1/agent/resolve`)
- Connexion WebSocket (`/api/v1/ws`)
- Création de token du portail captif (`/api/v1/captive-portal/token`)

### Isolation vhost du portail captif avec `SERVER_HOST`

Les règles iptables du portail captif dérivent le hostname virtuel pour le filtrage SNI/Host depuis `SERVER_URL`, et non depuis `SERVER_HOST`. Lorsque `SERVER_URL` contient une IP brute (ex. `http://10.0.0.7`), l'agent ne peut pas effectuer de filtrage au niveau du hostname et revient à un filtrage par port uniquement — les autres hôtes virtuels sur la même IP:port deviennent accessibles avant la fin de l'authentification.

Pour bénéficier à la fois de l'accès sans DNS **et** de l'isolation par hostname, utilisez un hostname résolvable dans `SERVER_URL` et omettez `SERVER_HOST` :

```bash
# Recommandé : DNS résout wirety.internal → 10.0.0.7
# L'agent se connecte à l'IP, le filtrage SNI/Host utilise "wirety.internal"
wirety-agent -server http://wirety.internal -token <TOKEN>
```

Si le DNS est réellement indisponible, `SERVER_HOST` + un `SERVER_URL` en IP brute fonctionne — vous perdez simplement l'isolation vhost pour les peers non authentifiés. Consultez la documentation du portail captif pour les implications de sécurité complètes.

## Détection des interfaces NAT

Sur les jump peers, l'agent ajoute une règle `MASQUERADE` pour chaque interface de sortie afin que le trafic forwardé soit correctement NATé, quelle que soit l'interface sélectionnée par la table de routage pour une destination donnée.

Par défaut, l'agent **détecte automatiquement toutes les interfaces de sortie** en analysant la table de routage (`ip route show`) et en conservant chaque interface qui :
- Possède une adresse IPv4
- N'est pas loopback (`lo`)
- N'est pas l'interface WireGuard elle-même

Cela signifie qu'un jump peer disposant à la fois d'un lien internet (`ens2`) et d'une interface VLAN privé (`ens6`) recevra automatiquement `MASQUERADE` sur les deux, permettant aux peers d'atteindre les ressources derrière l'une ou l'autre interface.

Utilisez `NAT_INTERFACES` pour remplacer la détection automatique si elle capture des interfaces indésirables :

```bash
# NAT uniquement via l'interface VLAN privé
export NAT_INTERFACES=ens6
```

## Prérequis de l'hôte
| Exigence | Raison |
|----------|--------|
| Kernel/module WireGuard | Création de l'interface |
| curl / bibliothèques TLS | Requêtes d'inscription |
| Permissions suffisantes | Configurer l'interface réseau, exécuter iptables |
| Port 80 libre sur l'IP de l'interface WireGuard | Le serveur HTTP du portail captif s'attache à `<wg-ip>:80` |
| Port 443 libre sur l'IP de l'interface WireGuard | Le serveur HTTPS du portail captif s'attache à `<wg-ip>:443` (certificat auto-signé) |
| Module kernel `nf_conntrack` | Correspondance d'état conntrack dans les règles pare-feu du portail captif |
| Module kernel `xt_string` | Isolation vhost SNI / en-tête Host dans les règles pare-feu du portail captif |

L'agent appelle `modprobe nf_conntrack` et `modprobe xt_string` automatiquement au démarrage. Ces modules sont inclus avec le kernel sur toutes les distributions grand public et ne nécessitent pas d'installation manuelle. Si l'un des modules est indisponible, l'agent journalise un avertissement et continue avec une isolation vhost du portail captif dégradée.

## Journalisation

L'agent utilise [zerolog](https://github.com/rs/zerolog) pour la journalisation structurée. Le niveau et le format sont tous deux configurables via un flag CLI ou une variable d'environnement — le flag a la priorité.

### `LOG_LEVEL`

Contrôle quelles entrées de log sont émises.

| Valeur | Quand l'utiliser |
|--------|-----------------|
| `trace` | Débogage approfondi au niveau protocole (très verbeux) |
| `debug` | Développement et tests d'intégration |
| `info` | Fonctionnement normal en production *(défaut)* |
| `warn` | Avertissements et erreurs uniquement |
| `error` | Erreurs et événements fatals uniquement |
| `fatal` | Silencieux sauf en cas de crash |

### `LOG_FORMAT`

| Valeur | Sortie | Quand l'utiliser |
|--------|--------|-----------------|
| `text` | Sortie console colorée et lisible par l'humain *(défaut)* | Développement local, accès direct au terminal |
| `json` | Un objet JSON par ligne | Agrégateurs de logs (Loki, Datadog, Elastic, etc.) |

**Exemple `text` :**
```
2:47PM INF websocket connected url=wss://wirety.example.com/api/v1/ws
2:47PM INF DNS server starting addr=10.255.0.1:53
```

**Exemple `json` :**
```json
{"level":"info","time":1744563600,"message":"websocket connected","url":"wss://wirety.example.com/api/v1/ws"}
{"level":"info","time":1744563600,"message":"DNS server starting","addr":"10.255.0.1:53"}
```

:::tip Recommandation pour la production
Utilisez `LOG_FORMAT=json` et `LOG_LEVEL=info` en production afin que les lignes de log soient analysables par les machines et puissent être ingérées sans règles de parsing supplémentaires.
:::

## Sécurité
- Le token est utilisé uniquement lors de l'inscription ; l'authentification éphémère est utilisée ensuite.
- Les clés privées sont générées côté serveur ; l'agent reçoit uniquement les données publiques et de configuration.
- Le blocage ACL empêche l'agent de recevoir des mises à jour (mise en quarantaine).

## Données de heartbeat
| Champ | Description |
|-------|-------------|
| hostname | Hostname système rapporté |
| uptime | Secondes depuis le démarrage |
| endpoint | Endpoint public détecté |
| last_seen | Horodatage serveur |

## Évolutions futures
- Rotation automatique des clés.
- Émission de métriques (Prometheus).
