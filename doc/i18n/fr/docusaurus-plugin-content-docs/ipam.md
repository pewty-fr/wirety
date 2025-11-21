---
id: ipam
title: IPAM
sidebar_position: 6
---

IPAM assigne des adresses IP aux peers dynamiques et propose des CIDRs disponibles.

## Suggestion de CIDR
Endpoint API: `/api/v1/ipam/available-cidrs` avec paramètres `base_cidr`, `max_peers`, `count`.

## Attribution
Lors de l'inscription dynamique, le serveur alloue IP unique dans le CIDR réseau.

## Bonnes pratiques
- Anticiper croissance (réserves)
- Éviter fragmentation excessive
- Surveiller utilisation pour re-dimensionner
