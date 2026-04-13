---
id: captive-portal
title: Portail Captif
sidebar_position: 7
---

Le portail captif applique l'authentification des utilisateurs avant d'accorder l'accès réseau à travers un jump peer.
Lorsqu'un nouveau peer WireGuard se connecte, tout son trafic est bloqué jusqu'à ce qu'il s'authentifie via l'interface web Wirety.

:::info OIDC requis
Le portail captif est **désactivé lorsque `AUTH_ENABLED=false`** (auth simple / mot de passe admin partagé). Comme l'auth simple n'a pas d'identité par utilisateur, la propriété des peers ne peut pas être appliquée. L'endpoint de création de token (côté agent) et l'endpoint d'authentification (côté navigateur) retournent tous deux `403` dans ce mode.
:::

## Fonctionnement

```
Le peer se connecte au tunnel WireGuard
        │
        ▼
iptables FORWARD DROP (chaîne WIRETY_JUMP)
        │
        ├─── Tunnel complet (AllowedIPs = 0.0.0.0/0)
        │         │
        │         ▼
        │    La détection du portail captif par l'OS se déclenche automatiquement
        │    (CNA sur macOS/iOS, NCSI sur Windows)
        │         │
        │         ▼
        │    Sonde HTTP → IP WG du jump peer:80
        │
        └─── Tunnel partagé (AllowedIPs = plage privée uniquement)
                  │
                  ▼
             Le peer tente d'atteindre une ressource privée
             (ex. server1.wg.example.com)
                  │
                  ▼
             DNS : domaine VPN interne → IP portail captif
                  │
                  ▼
             Requête HTTP/HTTPS → IP WG du jump peer:80/443
        │
        ▼
Serveur HTTP du portail captif (écoute sur <wg-ip>:80)
Serveur HTTPS du portail captif (écoute sur <wg-ip>:443, certificat auto-signé)
        │
        ▼
302 redirect → https://<server>/captive-portal?token=cpt_...&redirect=<url-originale>
        │
        ▼
L'utilisateur s'authentifie avec son compte OIDC
        │
        ▼
Le serveur vérifie : utilisateur authentifié == propriétaire du peer  ──► rejeter si différent
        │
        ▼
Le serveur met l'IP du peer en liste blanche (DB + push WebSocket vers l'agent)
        │
        ▼
L'agent re-synchronise iptables : règle ACCEPT ajoutée pour l'IP du peer
        │
        ▼
Le peer a un accès réseau complet
```

## Prérequis

- `AUTH_ENABLED=true` (OIDC) — le portail captif est désactivé en mode auth simple.
- Le peer doit avoir un **propriétaire** (défini lorsqu'un utilisateur crée le peer). Les peers sans propriétaire créés par un admin ne peuvent pas utiliser le portail captif.
- L'utilisateur authentifié doit être le propriétaire du peer. Ni un autre utilisateur ni un administrateur ne peut s'authentifier au nom du peer d'une autre personne.

## Configuration de l'agent

Les serveurs HTTP et HTTPS du portail captif démarrent automatiquement lorsque l'agent reçoit sa première politique. Aucune configuration supplémentaire n'est requise au-delà de ce qui est déjà nécessaire pour le fonctionnement du jump peer.

Le seul flag optionnel est `-portal-url` (ou `CAPTIVE_PORTAL_URL` env), qui prend par défaut la valeur `<SERVER_URL>/captive-portal`.

```bash
# Par défaut — l'URL du portail est dérivée de l'URL du serveur
wirety-agent -server https://wirety.example.com -token <TOKEN>

# URL de portail explicite (ex. si le portail captif est sur un domaine différent)
wirety-agent -server https://wirety.example.com -token <TOKEN> \
  -portal-url https://wirety.example.com/captive-portal
```

L'agent écoute directement sur l'IP de l'interface WireGuard sur les ports 80 et 443 (ex. `10.255.0.1:80` et `10.255.0.1:443`). Aucune règle DNAT ni sysctl `route_localnet` n'est nécessaire.

:::caution Disponibilité des ports
L'agent se lie aux **ports 80 et 443** sur l'IP de l'interface WireGuard. S'assurer que rien d'autre n'écoute déjà sur ces combinaisons adresse/port sur l'hôte jump peer.
:::

## Détection du portail captif par l'OS

Les systèmes d'exploitation modernes envoient des sondes HTTP vers des URL bien connues lors de la connexion à un réseau pour détecter les portails captifs. L'agent intercepte ces sondes à deux niveaux.

### Peers en tunnel complet (`AllowedIPs = 0.0.0.0/0`)

Lorsque tout le trafic est routé via le VPN, la détection du portail captif par l'OS se déclenche automatiquement :

- **macOS / iOS** — Captive Network Assistant (CNA) envoie des sondes HTTP à travers le tunnel
- **Windows** — Network Connectivity Status Indicator (NCSI) envoie des sondes HTTP à travers le tunnel
- **Android / Linux** — les vérifications de connectivité passent par le tunnel

Les sondes atteignent le serveur HTTP de l'agent sur `<wg-ip>:80` et reçoivent une redirection vers la page d'authentification.

### Peers en tunnel partagé (`AllowedIPs` = plage privée uniquement)

Lorsque seul le trafic privé est routé via le VPN, les sondes OS vont vers le réseau physique (pas le tunnel) et ne peuvent pas être interceptées. L'agent intercepte à la place les requêtes DNS pour les domaines de sonde bien connus et les ressources VPN internes.

#### Interception DNS des domaines de sonde

Le serveur DNS de l'agent résout les domaines de sonde bien connus vers l'IP WireGuard du jump peer, de sorte que les sondes initiées par l'OS voyagent à travers le tunnel :

| OS | Domaine de sonde |
|----|-----------------|
| Android / Chrome | `connectivitycheck.gstatic.com` |
| Android / Chrome | `clients3.google.com` |
| Apple (iOS / macOS) | `captive.apple.com` |
| Apple (iOS / macOS) | `www.apple.com` |
| Windows | `www.msftconnecttest.com` |
| Firefox | `detectportal.firefox.com` |
| GNOME | `nmcheck.gnome.org` |
| Debian | `network-test.debian.org` |

Les requêtes AAAA pour tous les domaines de sonde retournent NODATA pour forcer IPv4, empêchant les peers qui préfèrent IPv6 de contourner l'interception.

#### Interception du domaine VPN interne

Pour les peers non authentifiés, toutes les requêtes DNS pour les **noms de domaine VPN internes** (hostnames de peers, FQDNs de routes) sont résolus vers l'IP du portail captif au lieu de la vraie IP du peer. Cela signifie que toute tentative d'atteindre une ressource privée redirige le peer vers la page d'authentification.

Le DNS externe (internet) n'est pas affecté — les peers non authentifiés peuvent toujours naviguer sur le web normalement.

```
Peer non authentifié résout server1.wg.example.com
  → DNS retourne 10.255.0.1 (IP portail captif, TTL 5s)
  → Requête HTTP/HTTPS atteint le serveur du portail captif
  → Redirection vers la page d'authentification

Peer authentifié résout server1.wg.example.com
  → DNS retourne 10.255.0.2 (vraie IP du peer, TTL 60s)
  → La connexion va directement vers la ressource privée
```

:::info Exigence DNS
L'interception des sondes et l'interception du domaine interne ne fonctionnent que lorsque la configuration WireGuard définit `DNS = <ip-wg-jump-peer>` pour que le peer utilise le serveur DNS du jump peer.
:::

### Réponses aux sondes HTTP et HTTPS

Le serveur HTTP (`:80`) et le serveur HTTPS (`:443`) traitent les requêtes interceptées avec la même logique :

| État du peer | Comportement |
|-------------|--------------|
| **Non authentifié** | Retourne une redirection `302` vers la page d'authentification du portail captif |
| **Authentifié** | Retourne la réponse de succès spécifique à l'OS — l'OS ferme la notification du portail captif |

Réponses de succès spécifiques à l'OS (servies aux peers authentifiés) :

| OS | Chemin | Réponse |
|----|--------|---------|
| Google / Android | `/generate_204` | `204 No Content` |
| Apple | `/hotspot-detect.html` | `200` + `<HTML>...Success...</HTML>` |
| Windows | `/connecttest.txt` | `200` + `Microsoft Connect Test` |
| Firefox | `/success.txt` | `200` + `success\n` |
| GNOME / Debian | tout | `204 No Content` |

## Serveur HTTPS du portail captif

L'agent exécute un serveur HTTPS auto-signé sur `<wg-ip>:443` aux côtés du serveur HTTP. Cela gère les peers non authentifiés qui tentent un accès HTTPS aux ressources VPN internes.

### Certificat auto-signé

Le certificat est généré en mémoire au démarrage de l'agent (jamais écrit sur disque) et couvre :

- **IP SAN** — l'IP de l'interface WireGuard
- **DNS SAN générique** — `*.<vpnDomain>` (ex. `*.wg.example.com`) pour que les hostnames de peers internes correspondent au certificat

Le domaine VPN pour le générique est tiré de la configuration DNS poussée par le serveur.

### Comportement du navigateur

Comme le certificat est auto-signé (non émis par une CA de confiance), les navigateurs affichent un avertissement de sécurité. Le comportement diffère selon le domaine :

| Type de domaine | Comportement du navigateur |
|----------------|---------------------------|
| **Domaine VPN interne** (ex. `server1.wg.example.com`) | Page d'avertissement avec option "Continuer quand même" — l'utilisateur peut contourner et être redirigé vers le portail captif |
| **Domaine public dans la liste de préchargement HSTS** (ex. `google.com`) | Bloqué de façon permanente — aucun contournement disponible. Les peers utilisant des navigateurs HTTPS uniquement devront essayer une URL HTTP ou utiliser l'URL directe du portail captif |

:::info Limitation HTTPS
L'interception HTTPS pour les domaines publics préchargés HSTS n'est pas faisable : les navigateurs bloquent de telles connexions quelle que soit la valeur du certificat. Le serveur HTTPS est principalement utile pour les domaines VPN internes. Pour les peers en mode tunnel complet, la détection du portail captif par l'OS (qui utilise des sondes HTTP en clair) gère la redirection sans interaction avec les certificats.
:::

## Application de la propriété

Le serveur applique une propriété stricte pendant l'authentification du portail captif :

| Type de peer | Authentifié en tant que | Résultat |
|-------------|------------------------|---------|
| Peer avec propriétaire | Propriétaire du peer | ✅ Mis en liste blanche |
| Peer avec propriétaire | Utilisateur différent | ❌ `access denied: this peer belongs to another user` |
| Peer avec propriétaire | Administrateur | ❌ `access denied: this peer belongs to another user` |
| Peer sans propriétaire (créé par admin) | N'importe quel utilisateur | ❌ `access denied: this peer has no owner and cannot be authenticated via captive portal` |
| N'importe quel peer | N'importe quel utilisateur | ❌ `captive portal is not available when AUTH_ENABLED=false` (si OIDC désactivé) |

Lorsque l'authentification échoue avec une erreur de propriété, la page du portail captif affiche un bouton **"Se connecter avec un autre compte"** qui efface la session actuelle et recharge, permettant au bon utilisateur de s'authentifier.

## Cycle de vie des tokens

| Token | TTL | Objectif |
|-------|-----|---------|
| Token de portail captif (`cpt_…`) | 10 minutes | Token URL intégré dans l'URL de redirection. Conservé actif (non supprimé à la première utilisation) pour gérer la condition de course où l'agent n'a pas encore synchronisé iptables avant que le navigateur suive la redirection post-auth. Expire naturellement. |
| Cache de tokens par peer (agent) | 9 minutes | Cache en mémoire sur l'agent pour éviter de créer un nouveau token DB pour chaque requête HTTP interceptée. |

## Durée de vie des sessions

Les sessions utilisent exclusivement des cookies httpOnly — pas de localStorage. Le cookie est automatiquement envoyé avec chaque requête vers le domaine Wirety, y compris l'endpoint d'authentification du portail captif.

| Mode | TTL de session | Notes |
|------|---------------|-------|
| OIDC | 30 jours | Soutenu par le refresh token OIDC. L'access token est silencieusement rafraîchi par le middleware du serveur. Si le IdP révoque le refresh token, la session est invalidée à la prochaine requête. |
| Auth simple (`AUTH_ENABLED=false`) | 30 jours | Le portail captif est **désactivé** dans ce mode. |

Les sessions expirées sont purgées automatiquement de la base de données (`refresh_token_expires_at < NOW()`).

## Comportement à la déconnexion & reconnexion

### Session utilisateur (navigateur)
Le cookie de navigateur est persistant (TTL de 30 jours). Lorsque l'utilisateur ouvre à nouveau la page du portail captif après une reconnexion, il est déjà considéré comme authentifié et le flux du portail se déroule automatiquement sans nouvelle connexion.

### Liste blanche des peers (iptables)
La liste blanche du portail captif est persistée dans la base de données avec un **TTL de 24 heures**. Lorsque l'agent redémarre ou se reconnecte :

1. Le serveur pousse une mise à jour de politique via WebSocket incluant la liste blanche actuelle (non expirée).
2. L'agent re-synchronise iptables et re-ajoute les règles `ACCEPT`.

Les peers déjà authentifiés n'ont pas besoin de se réauthentifier après un redémarrage de l'agent, tant que leur IP VPN n'a pas changé et que le TTL de 24 heures n'a pas expiré.

:::caution
Si un peer reçoit une nouvelle IP VPN (ex. après une longue absence et que l'IPAM recycle l'adresse), l'ancienne entrée de liste blanche ne correspond plus et le peer doit se réauthentifier.
:::

## Sécurité

### Configuration WireGuard volée
Si la configuration WireGuard d'un utilisateur (clé privée) est volée, l'attaquant se connecte avec la même IP VPN et hériterait normalement de l'entrée de liste blanche. Deux défenses limitent les dommages :

**TTL de liste blanche (24 heures) :** Les entrées de liste blanche expirent après 24 heures. L'accès de l'attaquant se termine quand l'entrée expire, même si le vol n'est pas détecté.

**Révocation automatique lors d'un incident de sécurité :** La détection de changement d'endpoint de Wirety signale une activité suspecte (ex. le peer se connectant depuis une nouvelle IP publique). Lorsqu'un incident est créé et que le peer est mis en quarantaine, l'entrée de liste blanche du portail captif est révoquée immédiatement sur tous les jump peers. La règle iptables `ACCEPT` est supprimée à la prochaine synchronisation de l'agent.

### Configuration WireGuard partagée (intentionnelle)
Si un utilisateur partage sa configuration WireGuard avec une autre personne, cette personne se connectera avec la même IP VPN mais ne pourra pas passer le portail captif : l'authentification vérifie que la session Wirety appartient au propriétaire du peer. Tenter de s'authentifier en tant qu'utilisateur différent — même un administrateur — entraîne une erreur de propriété.

## Gestion de la liste blanche

La liste blanche est par jump peer et stockée dans la table `captive_portal_whitelist`.

| Opération | Quand |
|-----------|-------|
| `AddCaptivePortalWhitelist` | Le peer complète l'authentification du portail captif (upsert avec TTL 24h) |
| `GetCaptivePortalWhitelist` | L'agent demande une synchronisation de politique — filtre les entrées expirées |
| `RemoveCaptivePortalWhitelistByPeerIP` | Incident de sécurité détecté (quarantaine) |
| `ClearCaptivePortalWhitelist` | Désenregistrement du jump peer |
| `CleanupExpiredCaptivePortalWhitelist` | Tâche de fond toutes les heures |

## Dépannage

| Symptôme | Cause probable |
|---------|----------------|
| La page du portail captif dit "not available" | `AUTH_ENABLED=false` — activer OIDC pour utiliser le portail captif. |
| "access denied: this peer belongs to another user" | Connecté avec le mauvais utilisateur Wirety. Cliquer sur "Se connecter avec un autre compte" et se connecter en tant que propriétaire du peer. |
| "access denied: this peer has no owner" | Le peer a été créé par un admin sans assigner de propriétaire. Assigner un propriétaire dans le tableau de bord Wirety. |
| Le peer authentifié perd l'accès après 24 heures | Normal — le TTL de la liste blanche a expiré. Le peer doit se réauthentifier. |
| Le peer authentifié perd l'accès après un redémarrage de l'agent | La liste blanche n'a pas été restaurée — vérifier la connectivité WebSocket entre l'agent et le serveur. |
| Le popup du portail captif OS n'apparaît pas (tunnel partagé) | La configuration WireGuard du peer ne définit peut-être pas `DNS = <ip-wg-jump-peer>`. Sans cela, les domaines de sonde et les requêtes de domaine interne contournent le DNS du tunnel. Vérifier la configuration WireGuard du peer. |
| Le popup du portail captif OS n'apparaît pas (tunnel complet) | CNA/NCSI se déclenche automatiquement pour les peers en tunnel complet. Si cela ne se déclenche pas, essayer de déconnecter et reconnecter WireGuard. |
| Le popup du portail captif OS persiste après l'authentification | Le TTL DNS (5-10s) peut ne pas avoir expiré. Attendre quelques secondes ; la prochaine sonde recevra une réponse de succès. |
| Le domaine interne se résout vers l'IP du portail captif après l'authentification | Cache DNS périmé sur le peer. Le TTL court (5s) devrait expirer rapidement. Vider le cache DNS manuellement si nécessaire (`sudo dscacheutil -flushcache` sur macOS). |
| Port 80 ou 443 déjà utilisé sur le jump peer | Quelque chose d'autre est lié à `<wg-ip>:80` ou `<wg-ip>:443`. L'agent journalise une erreur et le portail captif ne fonctionnera pas. |
| Le navigateur bloque définitivement la redirection HTTPS pour un domaine externe | Normal — les domaines publics préchargés HSTS ne peuvent pas être interceptés. Utiliser l'URL directe du portail captif, ou essayer une URL HTTP ou une URL de domaine VPN interne pour déclencher la redirection. |

## Reverse Proxy et isolation d'hôte virtuel

Lorsque le serveur Wirety est déployé derrière un reverse proxy qui sert également d'autres applications sur la même IP et le même port, les peers non authentifiés pourraient atteindre ces autres applications avant de terminer l'authentification du portail captif.

L'agent atténue cela avec trois couches de filtrage appliquées dans `WIRETY_JUMP` :

| Couche | Règle | Protège contre |
|--------|-------|----------------|
| **IP** | La destination doit correspondre à l'IP du serveur résolue | Serveurs non liés |
| **Port** | `--dport` dérivé du schéma URL du serveur (`443` pour https, `80` pour http, ou explicite) | Autres ports sur le même serveur |
| **Hostname** | Correspondance de chaîne L7 sur le hostname virtuel | Autres vhosts derrière le même reverse proxy |

### Filtrage par hostname

Pour HTTPS, le **SNI** TLS (Server Name Indication) est envoyé en clair dans le TLS ClientHello. L'agent utilise une correspondance de chaîne iptables pour vérifier que le SNI correspond au hostname Wirety avant d'autoriser la connexion.

Pour HTTP, l'en-tête de requête `Host:` est mis en correspondance de la même façon.

Comme la correspondance de chaîne ne fonctionne que sur le premier paquet d'une session TCP, une règle conntrack `ESTABLISHED,RELATED` en tête de `WIRETY_JUMP` permet aux paquets suivants de sessions déjà acceptées de passer sans re-vérifier le hostname.

```
Règle 0:  -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
Règle 1a: -d <serverIP> -p tcp --dport 443 -m string --algo bm --string "wirety.example.com" -j ACCEPT  (HTTPS)
Règle 1b: -d <serverIP> -p tcp --dport 80  -m string --algo bm --string "Host: wirety.example.com" -j ACCEPT  (HTTP)
Règle 2:  -s <whitelistedPeerIP> -j WIRETY_POLICY  (peers authentifiés)
Règle 3:  -j DROP                                   (tous les autres)
```

### Limitations

**Contournement de la correspondance de chaîne :** Le module kernel `xt_string` analyse le contenu brut du paquet. Un peer qui fabrique un paquet contenant le hostname Wirety comme données arbitraires tout en utilisant un SNI TLS différent pourrait passer le filtre. Le routage du reverse proxy utiliserait toujours le bon SNI, mais la connexion au niveau IP serait acceptée. C'est une frontière souple, pas une frontière cryptographique.

**URL serveur en IP brute :** Si `SERVER_URL` est défini sur une IP brute (ex. `http://10.0.0.7`) au lieu d'un hostname, aucune correspondance SNI/Host n'est possible — l'agent revient à un filtrage par port uniquement, et tous les vhosts sur cette IP:port sont accessibles avant l'authentification. Utiliser un hostname dans `SERVER_URL` quand c'est possible. Voir [`SERVER_HOST`](agent#reverse-proxy--accès-sans-dns-server_host) pour la connexion par IP tout en activant le filtrage par hostname.

**Indisponibilité du module :** Si le module kernel `xt_string` n'est pas chargé, l'agent journalise un avertissement et revient automatiquement à un filtrage par port uniquement.

## Exigences des modules kernel

Les règles pare-feu du portail captif dépendent de deux modules kernel :

| Module | Objectif |
|--------|---------|
| `nf_conntrack` | Correspondance d'état conntrack — permet aux sessions TCP en cours de passer sans re-vérifier chaque paquet |
| `nft_compat` | Couche de compatibilité xtables pour `iptables-nft` — permet à `xt_string` d'être utilisé via le backend nf_tables. Sans effet sur iptables legacy. |
| `xt_string` | Correspondance de chaîne de charge utile — isolation vhost SNI / en-tête Host. Fonctionne sur iptables legacy et `iptables-nft` (via `nft_compat`). |

**L'agent charge ces modules automatiquement au démarrage** via `modprobe`. Aucune action manuelle n'est requise sur la plupart des systèmes — les modules sont livrés avec le kernel sur toutes les distributions grand public (Debian, Ubuntu, RHEL, Alpine).

:::info iptables-nft
Sur les systèmes Debian/Ubuntu modernes, `iptables` est `iptables-nft` par défaut. Le module `nft_compat` fait le pont entre l'interface d'extension xtables et nftables, rendant `xt_string` disponible sur les deux backends. Si `nft_compat` ou `xt_string` ne peut pas être chargé, l'agent revient automatiquement à un filtrage par port uniquement.
:::

Si un module échoue à se charger, l'agent journalise un avertissement et continue avec un comportement dégradé :

```
WARN  failed to load kernel module — functionality may be degraded
      module=xt_string purpose="payload string matching (SNI / Host-header vhost isolation)"
```

Pour que les modules persistent entre les redémarrages indépendamment de l'agent :

```bash
# Debian / Ubuntu
echo -e "nf_conntrack\nxt_string" >> /etc/modules

# RHEL / CentOS / Fedora
cat > /etc/modules-load.d/wirety.conf <<EOF
nf_conntrack
xt_string
EOF
```

Sur les kernels minimaux ou embarqués où les modules ne sont pas compilés, installer le paquet extras :

```bash
# Debian / Ubuntu
apt-get install linux-modules-extra-$(uname -r)
```
