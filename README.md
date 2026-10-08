# observabilite-agents-demo

Dépôt d'exemple de la série d'articles sur l'observabilité d'une application instrumentée avec OpenTelemetry et OpenObserve
sur Kubernetes (Scaleway Kapsule). Trois services en chaîne, une base Postgres et un générateur de trafic :

```text
load-generator -> order-service (Java) -> inventory-service (Go) -> pricing-service (Rust)
                       |                        |
                  base orders              base inventory     (une instance Postgres 18)
```

Ce dépôt contient le socle de déploiement (`deploy/`) et la publication des images (`.github/`). Le code des services est
dans `services/`.

## Arborescence

```text
deploy/
  k8s/
    namespace-demo.yaml          namespace demo
    otel-env-configmap.yaml      ConfigMap otel-env, variables OTEL_* communes
    postgres/
      initdb-configmap.yaml      ConfigMap postgres-initdb : rôles, bases, table stock et données initiales
      statefulset.yaml           StatefulSet + Service headless postgres
      secret.yaml.example        modèle du Secret postgres-credentials (valeurs CHANGE_ME)
    inventory-service.yaml       Deployment + Service
    pricing-service.yaml         Deployment + Service
    order-service.yaml           Deployment + Service
    load-generator.yaml          Deployment, une commande par seconde vers order-service
  helm/
    otel-cluster-values.yaml     release otel-cluster (clusterMetrics + kubernetesEvents)
    otel-values-patch.yaml       complément à la release otel (DaemonSet) : RBAC nodes/pods et métriques *_limit_utilization
services/
  inventory-service/             Go + Gin
  pricing-service/               Rust + Axum
  order-service/                 Java 25 + Spring Boot + agent OpenTelemetry
.github/workflows/images.yml     build et push des images vers GHCR
```

## Prérequis

- Un cluster Kubernetes avec la classe de stockage `scw-bssd-retain` (Kapsule), `kubectl` et `helm`.
- Le cluster, OpenObserve et le Collector `otel` (DaemonSet) en place, comme dans les articles 3a et 3b. Le Secret
  `otel-collector-auth` existe dans le namespace `otel`.
- Les images publiées sur GHCR (voir plus bas) et un jeton personnel GitHub limité à `read:packages`.

## Déploiement

Toutes les commandes se lancent depuis la racine du dépôt.

### 1. Namespace

```bash
kubectl apply -f deploy/k8s/namespace-demo.yaml
```

### 2. Secrets

Aucun secret n'est versionné. Le Secret `postgres-credentials` se crée par commande ; les mots de passe générés sont
hexadécimaux, car le script d'initialisation les insère entre apostrophes dans du SQL.

```bash
kubectl -n demo create secret generic postgres-credentials \
  --from-literal=postgres-password="$(openssl rand -hex 16)" \
  --from-literal=inventory-password="$(openssl rand -hex 16)" \
  --from-literal=orders-password="$(openssl rand -hex 16)"
```

`deploy/k8s/postgres/secret.yaml.example` documente la forme du Secret. Son extension l'exclut de
`kubectl apply -f deploy/k8s/postgres/`.

Le Secret de tirage des images GHCR `ghcr-pull` se crée avec un jeton `read:packages`, saisi à l'invite :

```bash
read -rs GHCR_PAT
kubectl -n demo create secret docker-registry ghcr-pull \
  --docker-server=ghcr.io \
  --docker-username=<utilisateur GitHub> \
  --docker-password="$GHCR_PAT"
unset GHCR_PAT
```

### 3. Release Collector `otel-cluster`

```bash
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm install otel-cluster open-telemetry/opentelemetry-collector \
  --version 0.173.1 \
  --namespace otel \
  --values deploy/helm/otel-cluster-values.yaml
```

### 4. Droits et métriques supplémentaires sur la release `otel`

`otel-values-patch.yaml` s'ajoute aux values de l'article 3b (`otel-daemonset-values.yaml`). Les listes `clusterRole.rules`
du patch s'ajoutent à celles du chart, et les métriques s'ajoutent à la configuration du receiver `kubeletstats`.

```bash
helm upgrade otel open-telemetry/opentelemetry-collector \
  --version 0.173.1 \
  --namespace otel \
  --values otel-daemonset-values.yaml \
  --values deploy/helm/otel-values-patch.yaml
```

### 5. Postgres, services et générateur de trafic

```bash
kubectl apply -f deploy/k8s/otel-env-configmap.yaml
kubectl apply -f deploy/k8s/postgres/
kubectl -n demo rollout status statefulset/postgres
kubectl apply -f deploy/k8s/inventory-service.yaml -f deploy/k8s/pricing-service.yaml -f deploy/k8s/order-service.yaml
kubectl apply -f deploy/k8s/load-generator.yaml
kubectl -n demo get pods
```

Les scripts de `postgres-initdb` ne s'exécutent que sur un répertoire de données vide. Pour les rejouer, supprimer le PVC
`data-postgres-0` ; la classe `scw-bssd-retain` conserve le volume sous-jacent, qu'il faut supprimer séparément.

## Images

Le workflow `.github/workflows/images.yml` construit les trois images avec le contexte `services/<service>` et les pousse
vers `ghcr.io/clevertechware/observabilite-agents-demo/<service>`.

| Événement | Tags |
|---|---|
| push sur `main` | SHA complet du commit |
| tag git `v0.1.0` | `0.1.0` et SHA complet |

Les manifestes référencent le tag `0.1.0` : créer le tag git `v0.1.0` avant le premier déploiement. Les paquets GHCR d'un
dépôt privé restent privés ; le Secret `ghcr-pull` donne au cluster le droit de les lire.

## Configuration des services

| Service | Variables propres au déploiement |
|---|---|
| `inventory-service` | `DATABASE_URL` (construite avec le mot de passe du Secret), `PRICING_URL` |
| `pricing-service` | aucune |
| `order-service` | `SPRING_DATASOURCE_URL`, `SPRING_DATASOURCE_USERNAME`, `SPRING_DATASOURCE_PASSWORD`, `INVENTORY_URL`, `OTEL_SEMCONV_STABILITY_OPT_IN=database` |

Tous chargent la ConfigMap `otel-env` (`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`,
`OTEL_TRACES_SAMPLER=parentbased_always_on`) et définissent `OTEL_SERVICE_NAME` et `OTEL_RESOURCE_ATTRIBUTES`
(`service.namespace=shop`, `service.version`, `deployment.environment.name=demo`). Les options de la JVM
(`JAVA_TOOL_OPTIONS`) sont posées dans l'image d'`order-service`.

## Piloter les pannes

Chaque panne est une variable d'environnement, à 0 par défaut. `kubectl set env` modifie le Deployment et relance le pod.

| Panne | Commande |
|---|---|
| Requête SQL lente (inventory) | `kubectl -n demo set env deploy/inventory-service FAULT_SLOW_QUERY_SECONDS=2` |
| 30 % d'erreurs 500 (pricing) | `kubectl -n demo set env deploy/pricing-service FAULT_ERROR_RATE=0.3` |
| Fuite mémoire jusqu'à OOMKilled (pricing) | `kubectl -n demo set env deploy/pricing-service FAULT_LEAK_BYTES_PER_REQUEST=1048576` |
| Fuite de heap jusqu'à `OutOfMemoryError` (order) | `kubectl -n demo set env deploy/order-service FAULT_HEAP_LEAK_BYTES_PER_REQUEST=1048576` |

Retour à la normale : remettre la variable à `0`.

```bash
kubectl -n demo set env deploy/pricing-service FAULT_ERROR_RATE=0 FAULT_LEAK_BYTES_PER_REQUEST=0
```

## Validation des manifestes

```bash
kubeconform -strict -kubernetes-version 1.36.0 deploy/k8s/*.yaml deploy/k8s/postgres/*.yaml
helm template otel-cluster open-telemetry/opentelemetry-collector --version 0.173.1 \
  --namespace otel --values deploy/helm/otel-cluster-values.yaml
actionlint .github/workflows/images.yml
```
