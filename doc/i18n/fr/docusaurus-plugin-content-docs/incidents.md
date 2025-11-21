---
id: incidents
title: Incidents de sécurité
sidebar_position: 5
---

Wirety détecte des conditions qui génèrent des incidents, isolant le peer via l'ACL.

## Cycle
1. Détection (session conflictuelle, saut suspect)
2. Ajout peer → `BlockedPeers`
3. Push mise à jour aux agents
4. Opérateur analyse & résout
5. Peer retiré → déblocage

## Avantages du blocage ACL
- Transparence : peer visible, état retraçable
- Réversibilité : résolution simple
- Audit : historique des incidents

## Résolution
Dans **Security → Incidents** cliquer **Resolve**.

## Types actuels
- Conflit de session
- Activité de saut suspecte

## Futures améliorations
- Règles personnalisables
- Intégration SIEM
