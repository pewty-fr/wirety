---
id: troubleshooting
title: Dépannage
sidebar_position: 9
---

## Pas d'accès réseau entre deux peers
Vérifier ACL (peer dans `BlockedPeers` ?) et endpoints WireGuard.

## Incident récurrent
Analyser pattern de heartbeat (IP change rapide). Stabiliser réseau ou revoir détection.

## Token refusé
Token expiré ou déjà utilisé (future logique). Regénérer.

## Frontend 404 (locale FR)
Assurez-vous que les fichiers de traduction sont dans `i18n/fr/docusaurus-plugin-content-docs/current/`.

## WebSocket 401
Vérifier authentification OIDC ou cookie session.
