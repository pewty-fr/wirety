---
id: intro
title: Démarrage
sidebar_position: 1
---

Bienvenue sur Wirety : un maillage WireGuard sécurisé et dynamique avec automatisation basée sur agents, réponse aux incidents pilotée par ACL, et types de pairs flexibles.

## Aperçu
Wirety orchestre un réseau overlay WireGuard en distinguant les Peers Jump (hubs de trafic) et les Peers Regular qui peuvent être soit Dynamiques (inscrits par agent) soit Statiques (configuration manuelle). Les incidents de sécurité sont gérés via le blocage ACL au lieu de la suppression de connexion pour un confinement plus sûr et auditable.

## Points clés de l'architecture
- Les peers jump routent et optionnellement NAT le trafic pour les peers regular.
- Les peers dynamiques s'inscrivent en utilisant un agent + token ; les peers statiques reçoivent une configuration WireGuard générée.
- L'ACL contient `BlockedPeers` pour le confinement des incidents ; la résolution supprime le peer de l'ACL.
- Les clés privées ne quittent jamais les réponses de l'API serveur (`json:"-"`).

## Prérequis
| Composant | Objectif | Statut |
|-----------|---------|--------|
| Cluster Kubernetes | Exécuter le chart Helm Wirety | Requis |
| Entrée DNS | Exposer front + serveur (ex. `wirety.example.com`) | Recommandé |
| WireGuard installé | Pour les pairs statiques (téléphones, ordinateurs) | Conditionnel |
| curl + bash | Pour l'installation de l'agent | Requis sur les hôtes dynamiques |

## Installation (Helm)
```bash
# Installer Wirety en utilisant le registre OCI
helm install wirety oci://rg.fr-par.scw.cloud/wirety/chart/wirety \
  --version <version> \
  --namespace wirety --create-namespace \
  --set ingress.enabled=true \
  --set ingress.host=wirety.example.com
```

Pour des options de déploiement détaillées, consultez le [Guide de déploiement](./deployment).

## Accès au Dashboard

Après l'installation, accédez au dashboard web à `https://wirety.example.com` (ou votre nom d'hôte configuré).

**Authentification par défaut :**
- **Sans OIDC :** Utilisateur par défaut `admin` / mot de passe `admin` (changez-le immédiatement !)
- **Avec OIDC :** Connectez-vous via votre fournisseur d'identité configuré.

## Créer votre premier réseau

1. **Ouvrir Networks** → Cliquez sur **Create Network**.
2. **Entrer le nom** (ex. `demo`) et **CIDR** (ex. `10.200.0.0/24`).
3. **Soumettre** → Le réseau est créé avec un premier peer jump pour le routage.

## Ajouter des peers

### Peer dynamique (avec agent)
1. **Sélectionner le réseau** → Cliquez sur **Add Peer**.
2. **Type:** `dynamic`, **Kind:** `regular`, **Name:** `server1`.
3. **Copier le token d'inscription** et l'installer sur l'hôte cible :
   ```bash
   curl -fsSL https://wirety.example.com/install-agent.sh | bash -s -- --token <TOKEN>
   ```

### Peer statique
1. **Add Peer** → **Type:** `static`, **Kind:** `regular`, **Name:** `laptop1`.
2. Copier la configuration générée (section WireGuard) dans `/etc/wireguard/wg0.conf`.
3. Activer : `sudo wg-quick up wg0`.

## Gestion des incidents

Quand une activité suspecte est détectée (ex. conflit de session, pattern de saut), Wirety ajoute le peer à `BlockedPeers` dans l'ACL du réseau. Le peer est alors isolé mais reste visible pour investigation. Pour résoudre :
1. Ouvrir **Security → Incidents**.
2. Examiner les détails.
3. Cliquer **Resolve** pour débloquer (applique mise à jour ACL).

## Étapes suivantes
- Lire la page [Architecture](./architecture.md).
- Explorer la gestion [ACL et sécurité](./incidents.md).
- Intégrer l'[agent](./agent.md) sur plus de hosts.

Bonne exploration de Wirety !
