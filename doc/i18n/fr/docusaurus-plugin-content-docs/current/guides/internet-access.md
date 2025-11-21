---
id: internet-access
title: Accès Internet via Jump
---

Ce guide explique comment permettre aux peers regular d'accéder à Internet via un peer Jump.

## Principe
Le peer Jump agit comme passerelle (routing + NAT) pour le trafic sortant des peers regular.

## Étapes
1. Activer routing/NAT sur le Jump (iptables ou nftables).
2. Ajuster la configuration WireGuard pour inclure `AllowedIPs = 0.0.0.0/0` sur les peers regular ciblés.
3. Vérifier que la politique ACL n'isole pas le Jump.

## Exemple iptables
```bash
sysctl -w net.ipv4.ip_forward=1
iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
```

## Vérification
Sur un peer regular :
```bash
curl https://ifconfig.me
```
L'adresse IP publique doit être celle du Jump.
