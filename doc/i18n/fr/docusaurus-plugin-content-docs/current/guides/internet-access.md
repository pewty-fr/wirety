---
id: internet-access
title: Peer - Accès Internet
sidebar_position: 1
---

Objectif : Router tout le trafic d'un peer regular à travers un jump peer pour fournir une sortie centralisée.

:::caution Ancienne approche supprimée
Le flag `full_encapsulation` sur les peers a été **supprimé**. Utilisez le système Groupes & Politiques à la place (voir ci-dessous).
:::

## Étapes (approche actuelle — Politiques)

1. S'assurer que le jump peer a une interface NAT configurée.
2. Créer (ou réutiliser) un **Groupe** contenant le peer regular.
3. Créer une **Politique** sur ce groupe avec une règle de route qui définit le jump peer comme passerelle par défaut (`0.0.0.0/0`).
4. Attacher la politique au groupe.
5. Les peers basés sur agent recevront la configuration mise à jour automatiquement ; les peers statiques doivent télécharger une nouvelle configuration.

Consultez [Groupes, Politiques & Routes](../groups-policies-routes-overview) pour tous les détails.

## Vérification
- La route par défaut du peer regular pointe maintenant vers l'interface tunnel.
- L'IP externe correspond à l'IP publique du jump peer.

## Portail captif

Lorsqu'un jump peer a le portail captif activé, les nouveaux peers sont bloqués jusqu'à ce qu'ils s'authentifient via l'interface web Wirety. Consultez [Portail Captif](../captive-portal) pour le flux complet, la durée de vie de la session et le comportement à la reconnexion.

## Remarques
Les performances dépendent de la bande passante et de la latence du jump peer. Envisagez de faire évoluer plusieurs jump peers.
