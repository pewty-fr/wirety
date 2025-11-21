---
id: internet-access
title: Accès Internet via Jump
---

Utiliser un peer Jump pour fournir sortie vers Internet aux peers regular.

## Étapes
1. Configurer NAT sur le Jump (iptables ou nftables).
2. Vérifier que les peers regular ont route par l'interface WireGuard.
3. Surveiller latence et charge CPU.

## Sécurité
Limiter exposition : firewall strict sur le Jump.
