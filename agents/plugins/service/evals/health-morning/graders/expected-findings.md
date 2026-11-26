---
type: llm
weight: 2
---

PASS si le rapport correspond aux données figées de la journée du 25/11/2026 (+01:00), qui contiennent : zéro réponse 5xx sur les trois services ; un seul
redémarrement de conteneur hors rollout, le pod `order-service-6f698dcb7f-vb65z` (raison `Error`, 2 redémarrages dans la fenêtre, vers 10h, logs
`org.springframework.beans.factory.UnsatisfiedDependencyException`) ; P99 dans le budget partout (order-service < 500 ms ≈ 230 ms,
inventory-service < 300 ms ≈ 121 ms, pricing-service < 50 ms ≈ 2,6 ms).

FAIL si : un 5xx est signalé alors que les requêtes n'en montrent aucun, un redémarrement est attribué à un rollout ou à un pod sans saut de compteur,
le redémarrement réel d'`order-service` est omis, la latence est déclarée hors budget, ou un champs/valeur cité ne provient pas d'une sortie d'outil.
