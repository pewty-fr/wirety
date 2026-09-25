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
             Le DNS renvoie la VRAIE IP de la ressource (pas l'IP du portail)
                  │
                  ├── HTTP  → routé, puis redirigé (DNAT) sur :80 vers le portail
                  └── HTTPS → routé, bloqué par un TCP reset
                              (non intercepté — pas de certificat, pas d'erreur HSTS)
        │
        ▼
Serveur HTTP du portail captif (écoute sur <wg-ip>:80)
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

Le serveur HTTP du portail captif démarre automatiquement lorsque l'agent reçoit sa première politique. Aucune configuration supplémentaire n'est requise au-delà de ce qui est déjà nécessaire pour le fonctionnement du jump peer.

Le seul flag optionnel est `-portal-url` (ou `CAPTIVE_PORTAL_URL` env), qui prend par défaut la valeur `<SERVER_URL>/captive-portal`.

```bash
# Par défaut — l'URL du portail est dérivée de l'URL du serveur
wirety-agent -server https://wirety.example.com -token <TOKEN>

# URL de portail explicite (ex. si le portail captif est sur un domaine différent)
wirety-agent -server https://wirety.example.com -token <TOKEN> \
  -portal-url https://wirety.example.com/captive-portal
```

L'agent écoute directement sur l'IP de l'interface WireGuard sur le port 80 (ex. `10.255.0.1:80`). Aucune règle DNAT vers localhost ni sysctl `route_localnet` n'est nécessaire. Le portail captif ne fait **pas** tourner de serveur HTTPS — le HTTPS non authentifié vers une ressource interne est bloqué par un TCP reset plutôt qu'intercepté (voir [Gestion du HTTPS](#gestion-du-https)).

:::caution Disponibilité du port
L'agent se lie au **port 80** sur l'IP de l'interface WireGuard. S'assurer que rien d'autre n'écoute déjà sur cette combinaison adresse/port sur l'hôte jump peer.
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

#### Résolution des domaines VPN internes

Les requêtes DNS pour les **noms de domaine VPN internes** (hostnames de peers, FQDNs de routes) sont toujours résolues vers la **vraie IP** de la ressource — la même réponse pour les peers authentifiés et non authentifiés. Le DNS n'est **pas** la frontière de contrôle d'accès ; c'est l'iptables du jump peer. Un peer non authentifié qui apprend la vraie IP ne peut toujours pas atteindre la ressource :

- **HTTPS** (`:443`) → routé → la chaîne `WIRETY_JUMP` la rejette par un TCP reset. La connexion échoue immédiatement, sans aucun certificat impliqué — il n'y a donc jamais d'erreur HSTS, même pour les applications HTTPS-only.
- **HTTP** (`:80`) → routé → une règle DNAT `PREROUTING` (nat) la redirige vers le portail captif local, qui sert la `302` vers la page d'authentification.

Résoudre la vraie IP (jamais l'IP du portail) signifie que le navigateur ne met jamais en cache l'IP du portail pour un hostname interne : dès que le peer s'authentifie, la ressource est joignable **immédiatement**, sans fenêtre de cache DNS obsolète à attendre (des navigateurs comme Firefox mettent en cache ~60 s quel que soit le TTL).

Pour les **peers full-tunnel**, l'agent est plus agressif : chaque requête A/AAAA externe d'un peer full-tunnel non authentifié est redirigée vers l'IP du portail captif. C'est nécessaire car les peers full-tunnel routent chaque connexion externe par le jump peer — sans cela, leur navigateur résoudrait les vraies IP et verrait ses connexions abandonnées silencieusement par la chaîne FORWARD, sans qu'aucune redirection vers le portail ne se déclenche. L'agent apprend les `AllowedIPs` de chaque peer via le heartbeat (`local_allowed_ips`) pour n'appliquer cela qu'aux peers concernés. Les peers split-tunnel utilisent le DNS externe normalement — leur trafic externe ne traverse pas le jump peer.

```
Peer non authentifié résout server1.wg.example.com
  → DNS retourne 10.255.0.2 (vraie IP du peer)
  → HTTP  → redirigé (DNAT) vers le portail captif → redirection vers l'auth
  → HTTPS → TCP reset (bloqué par iptables, non intercepté)

Peer authentifié résout server1.wg.example.com
  → DNS retourne 10.255.0.2 (vraie IP du peer)
  → La connexion va directement vers la ressource privée
```

:::info Exigence DNS
L'interception des sondes et l'interception du domaine interne ne fonctionnent que lorsque la configuration WireGuard définit `DNS = <ip-wg-jump-peer>` pour que le peer utilise le serveur DNS du jump peer.
:::

### Réponses aux sondes HTTP

Le serveur HTTP (`:80`) traite les requêtes interceptées avec cette logique :

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

## Gestion du HTTPS

Le portail captif est **exclusivement HTTP** — l'agent ne fait pas tourner de listener HTTPS et n'intercepte jamais la connexion TLS d'un peer. Un peer non authentifié qui tente du HTTPS vers une ressource interne voit sa connexion **coupée** (`WIRETY_JUMP` rejette `:443` par un TCP reset). Le navigateur échoue immédiatement, sans échange de certificat.

C'est un choix de conception délibéré. Injecter un portail captif dans une session HTTPS pour le hostname de l'application elle-même exigerait de servir un certificat auquel le client fait confiance pour ce hostname — c'est-à-dire un man-in-the-middle TLS. Une version antérieure faisait cela avec un certificat auto-signé généré en mémoire, mais c'était irrémédiablement cassé :

- Pour les hôtes **préchargés HSTS** (tous les grands fournisseurs SSO, et toute application qui envoie `Strict-Transport-Security`), le navigateur bloque le certificat non concordant **sans aucun contournement** — une page d'erreur irrécupérable.
- Pour les applications HTTPS-only, cela forçait un retour en `http://` après authentification, que ces applications refusent.

Supprimer l'interception élimine les **impasses HSTS** et laisse fonctionner les applications HTTPS-only. La découverte du portail pour les peers non authentifiés passe entièrement par HTTP :

- **Détection du portail captif par l'OS** — les sondes de l'OS (HTTP en clair) sont interceptées par DNS vers le jump peer et traitées sur `:80`, faisant apparaître la bannière native « Se connecter au réseau ». Le TCP reset sur `:443` incite en plus iOS/Android à lancer leur détection.
- **Le pop-up de connexion du tableau de bord** — l'application web Wirety interroge l'état de chaque appareil et, quand l'un a besoin de se connecter, propose un lien à la demande vers le portail (en HTTP).

Une fois le peer authentifié, le DNS résout l'application vers sa **vraie IP** et le HTTPS fonctionne sans altération — le portail n'est jamais dans le chemin TLS.

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

**Liaison stricte à l'endpoint :** Chaque entrée de liste blanche est liée à l'endpoint public complet du peer (`ip:port`) au moment de l'authentification. L'agent du jump peer compare l'endpoint live retourné par `wg show endpoints` à l'endpoint stocké à chaque requête du portail captif et à chaque resync iptables (toutes les 300 ms). Toute différence — IP différente, port NAT renégocié, ou même une nouvelle session WireGuard du même utilisateur légitime — supprime la règle `ACCEPT` iptables et force une nouvelle authentification via le portail captif. Une configuration volée utilisée depuis un autre réseau échoue donc à la vérification immédiatement, sans attendre l'expiration TTL.

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
| Port 80 déjà utilisé sur le jump peer | Quelque chose d'autre est lié à `<wg-ip>:80`. L'agent journalise une erreur et le portail captif ne fonctionnera pas. |
| Le HTTPS vers une ressource interne échoue avant l'authentification | Normal — le portail captif n'intercepte pas le HTTPS ; le `:443` non authentifié est coupé (TCP reset). Déclencher le portail en HTTP, ou utiliser le pop-up de connexion du tableau de bord. Après authentification, le HTTPS fonctionne normalement. |

## Reverse Proxy et isolation d'hôte virtuel

Lorsque le serveur Wirety est déployé derrière un reverse proxy qui sert également d'autres applications sur la même IP et le même port, les peers non authentifiés pourraient atteindre ces autres applications avant de terminer l'authentification du portail captif.

Pour un serveur HTTPS, l'agent ferme cette brèche avec un **proxy SNI** sur le jump peer :

1. Dans `nat PREROUTING`, les connexions des peers **non authentifiés** vers l'IP:port du serveur sont redirigées (chaînes `WIRETY_SNI` / `WIRETY6_SNI`) vers le proxy sur `<wg-ip>:3129` (`HTTPS_PROXY_PORT`). Les peers authentifiés sont exclus et continuent d'atteindre le serveur directement.
2. Le proxy lit le ClientHello TLS et compare son **SNI** (Server Name Indication, envoyé en clair) aux noms d'hôte autorisés : `SERVER_HOST`, les hôtes de `SERVER_URL` et `CAPTIVE_PORTAL_URL`, et l'hôte de l'issuer OIDC (l'IdP partage souvent le même ingress).
3. Une connexion autorisée est relayée octet par octet vers le serveur. Tout le reste — autre vhost, pas de SNI, trafic non TLS — est fermé.

Le proxy **ne déchiffre jamais rien** : TLS reste de bout en bout entre le peer et le serveur, et le peer voit le certificat du serveur lui-même.

```
nat WIRETY_SNI:   -s <IPPeerAuthentifié> -j RETURN
                  -d <serverIP> -p tcp --dport 443 -j REDIRECT --to-ports 3129
filter WIRETY_JUMP:
  Règle 0:  -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
  Règle 1:  -d <serverIP> -p tcp --dport 443 -j ACCEPT   (peers authentifiés — les non authentifiés ont été redirigés ci-dessus)
  Règle 2:  -s <IPPeerAuthentifié> -j WIRETY_POLICY
  Règle 3:  -j DROP                                       (tous les autres)
```

Avant authentification, le peer peut donc se connecter sur `https://wirety.example.com`, tandis que `https://docs.example.com` sur la même IP d'ingress est refusé. Après authentification, la policy s'applique normalement.

### Limitations

**Serveur en HTTP simple ou sans nom d'hôte :** le SNI n'existe qu'en TLS. Avec un `SERVER_URL` en `http://`, ou si aucun nom d'hôte n'est connu (`SERVER_URL` en IP brute sans `SERVER_HOST` ni hostname dans `CAPTIVE_PORTAL_URL`), le proxy est désactivé et tous les vhosts de l'IP:port du serveur sont joignables avant authentification. Dans ce dernier cas, l'agent journalise un avertissement au démarrage.

**Encrypted Client Hello (ECH) :** un client utilisant ECH masque le vrai SNI ; le proxy ne voit alors que le nom public et refuse la connexion, sauf si ce nom est autorisé. Les hôtes internes ne publient pas de configuration ECH, les navigateurs leur envoient donc un SNI en clair.

## Exigences des modules kernel

Les règles pare-feu du portail captif dépendent de ces modules kernel :

| Module | Objectif |
|--------|---------|
| `nf_conntrack` | Correspondance d'état conntrack — permet aux sessions TCP en cours de passer sans re-vérifier chaque paquet |
| `nft_compat` | Couche de compatibilité xtables pour `iptables-nft` (matches xtables via le backend nf_tables). Sans effet sur iptables legacy. |

L'isolation des hôtes virtuels ne nécessite aucun module kernel : elle est assurée par le proxy SNI de l'agent, en espace utilisateur.

**L'agent charge ces modules automatiquement au démarrage** via `modprobe`. Aucune action manuelle n'est requise sur la plupart des systèmes — les modules sont livrés avec le kernel sur toutes les distributions grand public (Debian, Ubuntu, RHEL, Alpine).

Si un module échoue à se charger, l'agent journalise un avertissement et continue avec un comportement dégradé :

```
WARN  failed to load kernel module — functionality may be degraded
      module=nf_conntrack purpose="conntrack state matching (ESTABLISHED/RELATED)"
```

Pour que les modules persistent entre les redémarrages indépendamment de l'agent :

```bash
# Debian / Ubuntu
echo -e "nf_conntrack\nnft_compat" >> /etc/modules

# RHEL / CentOS / Fedora
cat > /etc/modules-load.d/wirety.conf <<EOF
nf_conntrack
nft_compat
EOF
```

Sur les kernels minimaux ou embarqués où les modules ne sont pas compilés, installer le paquet extras :

```bash
# Debian / Ubuntu
apt-get install linux-modules-extra-$(uname -r)
```
