---
id: identity-providers
title: Fournisseurs d'identité
sidebar_position: 4
---

Wirety supporte tout **fournisseur OIDC standard** ainsi que **GitHub OAuth** comme source d'identité. Une fois configuré, les utilisateurs se connectent via le fournisseur d'identité de leur organisation ; Wirety crée un compte lors de la première connexion, et un administrateur peut ensuite assigner les rôles et les accès réseaux depuis l'interface.

| Fournisseur | Protocole | Proxy requis ? |
|-------------|-----------|----------------|
| Keycloak | OIDC | Non |
| Azure Entra ID | OIDC | Non |
| Slack | OIDC | Non |
| GitHub | OAuth 2.0 | Non |

> Tout autre fournisseur OIDC standard (Authentik, Zitadel, Okta, Google Workspace, …) fonctionne sans configuration supplémentaire.

---

## Configuration OIDC générique

Tous les fournisseurs OIDC standard partagent les mêmes quatre variables d'environnement :

```bash
AUTH_ENABLED=true
AUTH_ISSUER_URL=https://votre-fournisseur.example.com   # URL de l'émetteur OIDC
AUTH_CLIENT_ID=wirety                                   # ID du client / application OAuth
AUTH_CLIENT_SECRET=votre-secret-client
```

Le serveur récupère `{AUTH_ISSUER_URL}/.well-known/openid-configuration` au démarrage et met le résultat en cache pendant une heure (`AUTH_JWKS_CACHE_TTL`, défaut `3600`).

L'URI de redirection OAuth à enregistrer auprès de votre fournisseur est :

```
https://<votre-domaine-wirety>/
```

---

## Keycloak

### 1. Créer un client

Dans votre realm, allez dans **Clients → Créer un client** :

| Champ | Valeur |
|-------|--------|
| Type de client | OpenID Connect |
| ID du client | `wirety` (ou tout autre nom) |
| Authentification du client | Activée (client confidentiel) |

Sous **URIs de redirection valides**, ajoutez `https://<votre-domaine-wirety>/*`.

### 2. Copier les identifiants

Ouvrez l'onglet **Identifiants** et copiez le secret client.

### 3. Configurer Wirety

```bash
AUTH_ENABLED=true
AUTH_ISSUER_URL=https://keycloak.example.com/realms/<votre-realm>
AUTH_CLIENT_ID=wirety
AUTH_CLIENT_SECRET=<secret-client>
```

### Notes

- Keycloak inclut `email`, `name` et `sub` dans le token ID par défaut — aucune configuration supplémentaire n'est nécessaire.
- Pour pré-assigner automatiquement des utilisateurs au rôle `administrator`, utilisez un mapper de client Keycloak pour ajouter une revendication personnalisée, puis gérez les rôles dans l'interface Wirety après la première connexion.

---

## Azure Entra ID

### 1. Enregistrer une application

Dans le portail Azure, allez dans **Inscriptions d'applications → Nouvelle inscription** :

| Champ | Valeur |
|-------|--------|
| Types de comptes pris en charge | Comptes dans cet annuaire organisationnel uniquement |
| URI de redirection (Web) | `https://<votre-domaine-wirety>/` |

### 2. Créer un secret client

Allez dans **Certificats et secrets → Nouveau secret client**. Copiez immédiatement la valeur du secret.

### 3. Trouver votre ID de locataire

L'ID de locataire se trouve dans **Vue d'ensemble → ID de l'annuaire (locataire)**.

### 4. Configurer Wirety

```bash
AUTH_ENABLED=true
AUTH_ISSUER_URL=https://login.microsoftonline.com/<tenant-id>/v2.0
AUTH_CLIENT_ID=<application-client-id>
AUTH_CLIENT_SECRET=<secret-client>
```

### Notes

- Azure Entra ID n'inclut pas la revendication `email` dans le token ID par défaut. Wirety se rabat automatiquement sur l'endpoint userinfo et, si nécessaire, sur la revendication `upn` (user principal name).
- Azure retourne `expires_in` sous forme de chaîne entre guillemets (`"3600"`) plutôt qu'un entier. Wirety gère cela de manière transparente.
- Assurez-vous que l'application dispose de la permission déléguée **User.Read**.

---

## Slack

### 1. Créer une application Slack

Rendez-vous sur [api.slack.com/apps](https://api.slack.com/apps) → **Créer une nouvelle application → Depuis zéro**.

### 2. Configurer OAuth

Sous **OAuth & Permissions** :

- Ajoutez l'URL de redirection : `https://<votre-domaine-wirety>/`
- Ajoutez les **User Token Scopes** suivants : `openid`, `email`, `profile`

### 3. Installer l'application

Cliquez sur **Installer dans l'espace de travail** et autorisez. Copiez l'**ID client** et le **Secret client** depuis **Informations de base**.

### 4. Configurer Wirety

```bash
AUTH_ENABLED=true
AUTH_ISSUER_URL=https://slack.com
AUTH_CLIENT_ID=<client-id>
AUTH_CLIENT_SECRET=<secret-client>
```

### Notes

- Slack est un fournisseur OIDC complet — aucun proxy ni outil supplémentaire n'est requis.
- Slack ne supporte pas la déconnexion initiée par la partie de confiance (`end_session_endpoint`). Cliquer sur **Déconnexion** dans Wirety invalide uniquement la session Wirety ; la session de l'espace de travail Slack n'est pas affectée.
- L'accès est naturellement limité aux utilisateurs qui ont l'application installée dans leur espace de travail. Si votre application Slack est distribuée sur plusieurs espaces de travail, tout membre d'un espace de travail peut se connecter — il n'y a pas de restriction par espace de travail au niveau de Wirety.

---

## GitHub

GitHub est un fournisseur OAuth 2.0, pas OIDC. Wirety gère les différences en interne — la configuration et l'expérience de connexion sont identiques aux fournisseurs OIDC.

### 1. Créer une application OAuth

Allez dans **GitHub → Paramètres → Paramètres développeur → Applications OAuth → Nouvelle application OAuth** (ou l'équivalent de votre organisation sous **Paramètres de l'organisation → Paramètres développeur**).

| Champ | Valeur |
|-------|--------|
| Nom de l'application | Wirety |
| URL de la page d'accueil | `https://<votre-domaine-wirety>/` |
| URL de rappel d'autorisation | `https://<votre-domaine-wirety>/` |

Copiez l'**ID client**. Générez un **Secret client** et copiez-le.

### 2. Configurer Wirety

```bash
AUTH_ENABLED=true
AUTH_ISSUER_URL=https://github.com
AUTH_CLIENT_ID=<client-id>
AUTH_CLIENT_SECRET=<secret-client>
```

### Notes

- La valeur `AUTH_ISSUER_URL=https://github.com` est le déclencheur qui active le chemin de code spécifique à GitHub. N'ajoutez pas de chemin supplémentaire.
- GitHub utilise les scopes OAuth `read:user` et `user:email` — ceux-ci sont demandés automatiquement.
- Si l'email d'un utilisateur est défini comme privé sur GitHub, Wirety le récupère automatiquement depuis l'endpoint API `/user/emails`. L'utilisateur doit avoir au moins une adresse email vérifiée.
- Les tokens d'accès GitHub n'expirent pas. Les sessions Wirety créées via GitHub durent **30 jours**, après quoi l'utilisateur doit se reconnecter.
- GitHub ne supporte pas la déconnexion initiée par la partie de confiance. Cliquer sur **Déconnexion** dans Wirety invalide uniquement la session Wirety.
- **Tout utilisateur GitHub peut se connecter**, que l'application OAuth soit créée sous un compte personnel ou un compte d'organisation. La propriété d'une application OAuth par une organisation GitHub ne contrôle que son *administration* — elle ne restreint pas la page d'autorisation aux membres de l'organisation. L'URL d'autorisation est publique et l'appartenance à une organisation n'est jamais vérifiée dans le flux OAuth GitHub standard. Pour un contrôle d'accès au niveau de l'organisation, utilisez un proxy tel que [Dex](https://dexidp.io) avec GitHub comme connecteur et une liste d'organisations autorisées.

---

## Attribution des rôles

Quel que soit le fournisseur, les rôles sont gérés dans Wirety — le fournisseur d'identité fournit uniquement l'identité (email, nom, identifiant unique). L'attribution des rôles suit cette logique :

1. Le **premier utilisateur** à se connecter devient automatiquement `administrator`.
2. Les **utilisateurs suivants** reçoivent le rôle par défaut configuré sous **Paramètres → Valeurs par défaut des utilisateurs** (`user` par défaut).
3. Un administrateur peut promouvoir ou rétrograder n'importe quel utilisateur à tout moment depuis **Admin → Utilisateurs**.

---

## Dépannage

| Symptôme | Cause probable | Solution |
|----------|---------------|----------|
| Boucle de redirection après connexion | Incompatibilité d'URI de redirection | Vérifiez que l'URI enregistrée auprès de votre fournisseur correspond exactement à `https://<votre-domaine-wirety>/` |
| Erreur `email claim is empty` | Le fournisseur n'envoie pas l'email | Ajoutez le scope `email` ; pour Azure, vérifiez la permission User.Read |
| `Failed to discover OIDC endpoints` | URL d'émetteur incorrecte | Vérifiez que `AUTH_ISSUER_URL` pointe vers la racine de l'émetteur (sans barre oblique finale) |
| Erreurs JWT liées à l'horloge | Décalage d'horloge du serveur | Synchronisez l'horloge du serveur Wirety avec NTP |
| GitHub : `empty access token` | Mauvais secret client | Régénérez et mettez à jour `AUTH_CLIENT_SECRET` |
| GitHub : aucun email retourné | Tous les emails GitHub sont privés et non vérifiés | L'utilisateur doit vérifier au moins une adresse email sur GitHub |
