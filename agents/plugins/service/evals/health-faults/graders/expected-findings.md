---
type: llm
weight: 2
---

PASS si le rapport correspond aux données figées du 26/11/2026 entre 16h30 et 18h00 (+01:00), qui contiennent : une requête SQL lente injectée sur
`inventory-service` de 16:39:31 à 17:31:31 (`pg_sleep`, span `SELECT` d'environ 2 s, 99 requêtes), qui porte le P99 d'`inventory-service` à environ
2,2 s pour un budget de 300 ms, et celui d'`order-service` à environ 2,3 s pour un budget de 500 ms, par propagation de l'appel à
`inventory-service` ; un redémarrage `OOMKilled` du pod `pricing-service-666cb5f467-nwn7r` vers 17:50-17:51, après une fuite mémoire injectée
à 17:46:35 ; pas d'autre redémarrement : les rollouts d'`inventory-service` (16:39:31, 17:31:33) et de `pricing-service` (17:46:35, 17:52:07)
créent de nouveaux pods sans redémarrement ; P99 de `pricing-service` dans son budget (≈ 33 ms pour 50 ms) ; deux 5xx propagées
(`order-service` 502, `inventory-service` 500) à 17:50:36, au moment où le pod pricing est tué.

FAIL si la lenteur d'`order-service` est présentée comme sa cause propre, si `inventory-service` n'est pas désigné comme origine des lenteurs, si un
rollout est compté comme redémarrage, si le redémarrage `OOMKilled` est omis, ou si une valeur citée ne provient pas d'une sortie d'outil.
