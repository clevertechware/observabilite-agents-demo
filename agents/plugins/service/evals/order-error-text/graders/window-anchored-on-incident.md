---
type: llm
---

PASS si le rapport situe l'incident le 26/11/2026 vers 12:01-12:02 (+01:00), et si les requêtes lancées interrogent
cette fenêtre (options `--from`/`--to` autour de 12:01) et non une fenêtre relative à l'instant courant.
FAIL si le rapport ou les requêtes s'ancrent sur « maintenant » (`--since`, tri sur les entrées récentes).
