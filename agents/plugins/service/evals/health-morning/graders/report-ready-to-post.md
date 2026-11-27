---
type: llm
---

PASS si le message final se suffit à lui-même : titre avec la fenêtre complète en toutes lettres et son fuseau ; une ligne de verdict (`sain`,
`dégradé`, `critique`) par service dans l'ordre order-service, inventory-service, pricing-service, aucun service omis ; sections Erreurs,
Redémarrages, Lenteurs ; 5 liens au maximum, chacun issu d'un appel `ctwctl o2 link` ; 5 000 caractères au total ; pas de préambule hors rapport
(le rapport est le message, prêt à poster dans Rocket.Chat).

FAIL si le rapport dépasse 5 000 caractères, si la fenêtre est relative à l'instant courant dans le titre, si un service n'a pas de verdict, ou si un
lien n'a pas été construit par `ctwctl o2 link`.
