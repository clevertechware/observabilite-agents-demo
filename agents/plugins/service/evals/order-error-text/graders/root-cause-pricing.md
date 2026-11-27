---
type: llm
weight: 2
---

PASS si le rapport désigne `pricing-service` (erreur injectée, `GET /prices/{sku}` en erreur 500) comme cause racine,
et indique que le message vu par le client (`inventory-service unavailable`) accuse à tort `inventory-service`.
FAIL si la cause racine est attribuée à `inventory-service`, `order-service` ou Traefik, ou si elle est présentée
comme probable sans preuve tirée des logs ou des traces.
