---
id: network
title: Réseaux
sidebar_position: 3
---

Un réseau représente un segment WireGuard logique (ex: `demo`, `prod-eu`). Chaque réseau possède : ACL, ensemble de peers, métadonnées, incidents associés.

## Création
1. **Create Network** dans l'UI.
2. Fournir `name` + `cidr`.
3. Un peer Jump initial est créé automatiquement (selon implémentation actuelle) pour le routage de base.

## ACL
Structure contrôlant l'accès. Clé principale : `BlockedPeers` (liste d'identifiants de peers à isoler). L'agent retire la connectivité logique en appliquant cette liste.

## Mise à jour
Changement de paramètre réseau (future évolution) déclenche push vers agents.

## Suppression
Non recommandée en production sans purge des peers; retirer d'abord les peers dynamiques pour éviter configurations orphelines.

## Bonnes pratiques
- Utiliser des noms explicites (`demo`, `prod-eu`, `test-lab`).
- Choisir un CIDR suffisamment large pour la croissance future.
- Éviter chevauchement CIDR entre réseaux pour simplifier routage.

## Étapes suivantes
Lire [Peers](./peers.md) pour gérer les différents types.
