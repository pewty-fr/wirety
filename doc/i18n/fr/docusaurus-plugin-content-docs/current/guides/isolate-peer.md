---
id: isolate-peer
title: Isolation d'un Peer
---

Ce guide décrit comment isoler rapidement un peer en cas de comportement suspect.

## Méthode ACL
Ajouter l'identifiant du peer dans `BlockedPeers` de l'ACL du réseau.

## Via l'interface
1. Aller sur **Networks → ACL**.
2. Ajouter le peer dans la liste `BlockedPeers`.
3. Sauvegarder pour pousser la mise à jour.

## Résultat
Le peer reste visible mais perd la connectivité logique (sessions dégradées).

## Résolution
Retirer l'identifiant de `BlockedPeers` puis sauvegarder.
