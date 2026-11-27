---
type: llm
weight: 2
---

PASS si le rapport correspond aux données figées du 26/11/2026 entre 11h00 et 13h30 (+01:00), qui contiennent : des 5xx en chaîne d'origine
`pricing-service` — `GET /prices/{sku}` en 500 (`injected failure`), propagées en 500 sur `inventory-service` `POST /reservations` et en 502 sur
`order-service` `POST /orders`, 8 par service sur la fenêtre, en deux épisodes : 12:01-12:02 et 13:20:50-13:30 ; des redémarrements du pod
`order-service-6f698dcb7f-zqb4c` (raison `Error`, 4 dans la fenêtre, vers 11:18-11:22) ; pas d'autre redémarrement : les rollouts de
`pricing-service` (12:01:17, 12:01:59, 13:20:50) créent de nouveaux pods sans redémarrement ; P99 dans le budget (order ≈ 127 ms, inventory ≈ 64 ms,
pricing ≈ 1,8 ms).

FAIL si la cause des 5xx est attribuée à `inventory-service` ou `order-service` comme origine, si `order-service` n'est pas présenté comme conséquence,
si un rollout est compté comme redémarrement, si le crashloop d'`order-service` est omis, si un seul des deux épisodes d'erreurs est signalé
(12:01 doit apparaître, ainsi que 13:20 ou plus tard), ou si un champs/valeur cité ne provient pas d'une sortie d'outil.
