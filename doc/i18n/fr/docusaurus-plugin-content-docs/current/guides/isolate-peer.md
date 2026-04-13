---
id: isolate-peer
title: Peer - Isolation
sidebar_position: 2
---

Objectif : Empêcher la communication latérale entre un peer et les autres tout en conservant la connectivité avec le jump peer.

:::caution Ancienne approche supprimée
Le flag `is_isolated` sur les peers a été **supprimé**. Utilisez le système Groupes & Politiques à la place (voir ci-dessous).
:::

## Étapes (approche actuelle — Politiques)

1. Créer (ou réutiliser) un **Groupe** contenant uniquement le peer à isoler.
2. Créer une **Politique** avec des règles qui refusent le trafic peer-à-peer tout en autorisant le trafic vers/depuis le jump peer.
3. Attacher la politique au groupe.
4. Les peers basés sur agent se mettent à jour automatiquement ; les peers statiques doivent télécharger une nouvelle configuration.

Consultez [Groupes, Politiques & Routes](../groups-policies-routes-overview) pour tous les détails.

## Vérification
- Le ping depuis le peer isolé vers un autre peer regular échoue.
- Le ping vers le jump peer réussit.

## Cas d'utilisation
- Appareil non fiable.
- Hôte d'environnement de staging.
- Accès invité.
