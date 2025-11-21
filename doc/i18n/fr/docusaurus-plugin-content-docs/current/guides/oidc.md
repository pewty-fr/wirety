---
id: oidc
title: Intégration OIDC
---

Configurer l'authentification fédérée via OpenID Connect pour Wirety.

## Variables serveur
| Variable | Description |
|----------|-------------|
| AUTH_ENABLED | Mettre à `true` pour activer OIDC |
| AUTH_ISSUER_URL | URL de l'issuer OIDC (dex, keycloak, etc.) |
| AUTH_CLIENT_ID | Client ID enregistré |
| AUTH_CLIENT_SECRET | Secret associé |
| AUTH_JWKS_CACHE_TTL | Cache de clés publiques (secondes) |

## Étapes
1. Créer client dans votre fournisseur OIDC.
2. Définir redirect URI: `https://wirety.example.com/auth/callback` (selon implémentation front).
3. Exporter variables d'environnement sur le serveur.
4. Redémarrer le serveur Wirety.

## Vérification
Accéder au frontend → bouton Login → redirection vers l'IdP puis retour avec session établie.

## Dépannage
- 401 sur `/api/v1/users/me` : vérifier token ou configuration client.
- Erreur de découverte : l'URL issuer doit pointer vers le document `.well-known/openid-configuration`.
