---
max_turns: 40
timeout_seconds: 900
allowed_tools: [Read, Glob, Grep, Skill]
---

Un utilisateur a reporté une erreur lorsqu'il a essayé de passer commande aujourd'hui vers 12h, regarde ce qui se passe et fais-moi un rapport détaillé.

Contexte : le signalement date du 26/11/2026, fuseau Europe/Paris (+01:00). L'incident est figé dans la fenêtre absolue 2026-11-26T12:01:17+01:00 → 2026-11-26T12:01:59+01:00, soit 11:01:17Z–11:01:59Z (1795690877000000–1795690919000000 en microsecondes epoch).

Accès aux données d'observabilité : utilise UNIQUEMENT les outils MCP du serveur « o2 » exposés à toi (mcp_o2.StreamList, mcp_o2.StreamSchema, mcp_o2.SearchSQL, mcp_o2.GetLatestTraces, mcp_o2.PrometheusRangeQuery). N'utilise PAS la CLI ctwctl. Pour SearchSQL, enveloppe la requête dans `request_body.query` avec `sql`, `start_time` et `end_time` en microsecondes epoch (fenêtre absolue ci-dessus) et `size` pour borner le nombre de lignes.

Rapport attendu : le même rapport détaillé que le cas order-error-text — résumé avec la cause racine, timeline, chaîne d'appels entre services, preuves (lignes de logs d'erreur, trace ids), liens vers les logs d'erreur et vers une trace dans l'UI OpenObserve (https://o2.obs.scw.clevertechware.fr/web/...), et recommandations.
