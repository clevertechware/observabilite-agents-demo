# observabilite-agents-demo

Dépôt d'exemple de la série d'articles sur l'observabilité d'une application instrumentée avec OpenTelemetry et OpenObserve
sur Kubernetes (Scaleway Kapsule). Trois services en chaîne, une base Postgres et un générateur de trafic :

```text
load-generator -> order-service (Java) -> inventory-service (Go) -> pricing-service (Rust)
                       |                        |
                  base orders              base inventory     (une instance Postgres 18)
```

Ce dépôt contient l'infrastructure Scaleway (`infra/`), le socle de déploiement (`deploy/`) et la publication des
images (`.github/`). Le code des services est dans `services/`.

## Arborescence

```text
infra/                           OpenTofu / Terraform : réseau, cluster Kapsule, pool de nœuds, IP du load balancer
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
  o2/
    dashboards/
      apm.dashboard.json         dashboard APM d'OpenObserve (articles 5b à 5g), schéma v8, rendu dans helm/workspace/
  helm/
    config.env.example               modèle de config.env (ignoré par git) : hôtes, e-mail ACME, plages d'adresses autorisées
    render-values.sh                 valide config.env et rend les manifestes à valeurs dans workspace/ (ignoré par git)
    o2/
      openobserve-values.yaml        release openobserve (3a)
      openobserve-ingress.yaml       Ingress d'OpenObserve (3a), rendu dans workspace/
    traefik/
      traefik-values.yaml            release traefik (3a), rendu dans workspace/
      ip-allowlist.yaml              Middleware ip-allowlist du namespace openobserve (3a), rendu dans workspace/
      order-service-ingress.yaml     Ingress d'order-service limité à /orders (4e), rendu dans workspace/
      order-service-rate-limit.yaml  Middleware rate-limit du namespace demo (4e)
      traefik-values-tracing.yaml    export OTLP des traces de Traefik vers le Collector (4e)
    otel/
      otel-daemonset-values.yaml     release otel (3b) : logs de tous les pods vers OpenObserve
      otel-values-metrics.yaml       release otel (3c) : métriques kubelet des nœuds, pods et conteneurs
      otel-cluster-values.yaml       release otel-cluster (3c) : clusterMetrics + kubernetesEvents
      otel-values-traefik-logs.yaml  release otel (3d) : parsing des access logs JSON de Traefik
      otel-values-patch.yaml         release otel (4a) : Service OTLP, traces, exclusion du namespace demo du filelog
      otel-values-filter-probes.yaml release otel (4d) : retire les spans et métriques des sondes Actuator d'order-service
      otel-values-disk-io.yaml       release otel (5b) : scrape cAdvisor pour les débits disque en lecture et en écriture
      otel-values-workload.yaml      release otel (5c) : attribut k8s.workload.name sur les métriques et les traces
compose.yaml                     point d'entrée Compose : inclut le socle et les trois services
services/
  compose.yaml                   socle mutualisé : Postgres (bases inventory et orders) + Collector OTLP/HTTP
  otel-collector.yaml            config du Collector (exporte la télémétrie en logs, exporter debug)
  postgres/initdb/               initialisation des rôles, bases et schémas au premier démarrage
  inventory-service/             Go + Gin
  order-service/                 Java 27 + Spring Boot + agent OpenTelemetry
  pricing-service/               Rust + Axum
.github/workflows/images.yml     build et push des images vers GHCR
```

## Développement local avec Docker Compose

Le socle `services/compose.yaml` est mutualisé par les trois services : une instance Postgres 18 avec
les bases `inventory` et `orders` (schémas et données de démonstration chargés au premier démarrage,
comme le `postgres-initdb` de `deploy/k8s/postgres`) et un Collector OpenTelemetry OTLP/HTTP sur
`http://localhost:4318` qui affiche la télémétrie reçue dans ses logs. Chaque service a son propre
`services/<service>/compose.yaml` qui inclut ce socle via `include` ; le `compose.yaml` racine les
inclut tous. Docker Compose v2.20 ou supérieur est requis.

Depuis la racine du dépôt :

```bash
docker compose up --build                                  # toute la chaîne + le socle
docker compose up --build order-service                     # un seul service + le socle
docker compose up --build order-service inventory-service   # un sous-ensemble
```

Depuis le dossier d'un service, son `compose.yaml` suffit (`docker compose up --build` lance le
service et le socle) :

```bash
docker compose -f services/pricing-service/compose.yaml up --build
```

Ports publiés sur `127.0.0.1` : order-service `8080`, inventory-service `8081`, pricing-service `8082`,
Postgres `5432`, OTLP/HTTP `4318`.

```bash
curl --fail http://localhost:8080/orders \
  -H 'Content-Type: application/json' -d '{"sku":"SKU-001","quantity":2}'
curl --fail http://localhost:8081/stock/SKU-001
curl --fail 'http://localhost:8082/prices/SKU-001?quantity=2'
docker compose logs -f otel-collector
```

`docker compose down` arrête les conteneurs et conserve les données Postgres ;
`docker compose down -v` **supprime toutes les données locales**. Aucun fichier `.env` n'est nécessaire.

En conteneur, les services se joignent par leurs noms DNS Compose (`inventory-service:8080`,
`pricing-service:8080`). Pour faire appeler un service conteneurisé à un service lancé sur la machine
hôte (`go run`, `cargo run`, `java -jar`), surchargez `INVENTORY_URL` ou `PRICING_URL`, par exemple :

```bash
PRICING_URL=http://host.docker.internal:8081 docker compose up --build inventory-service
```

## Prérequis

- Un cluster Kubernetes avec la classe de stockage `scw-bssd-retain` (Kapsule), `kubectl` et `helm`. Le dossier
  [`infra/`](infra/README.md) le crée.
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
  --values deploy/helm/otel/otel-cluster-values.yaml
```

### 4. Chemin OTLP sur la release `otel`

`otel-values-patch.yaml` s'ajoute aux values des articles 3b, 3c et 3d. Il expose le Collector aux services en OTLP,
ajoute la pipeline de traces et exclut le namespace `demo` du receiver `filelog`. Une release Helm n'a qu'un jeu de values
effectif : chaque `helm upgrade` doit repasser tous les fichiers, dans cet ordre.

```bash
helm upgrade otel open-telemetry/opentelemetry-collector \
  --version 0.173.1 \
  --namespace otel \
  --values deploy/helm/otel/otel-daemonset-values.yaml \
  --values deploy/helm/otel/otel-values-metrics.yaml \
  --values deploy/helm/otel/otel-values-traefik-logs.yaml \
  --values deploy/helm/otel/otel-values-patch.yaml \
  --values deploy/helm/otel/otel-values-filter-probes.yaml \
  --values deploy/helm/otel/otel-values-disk-io.yaml \
  --values deploy/helm/otel/otel-values-workload.yaml
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

### 6. Exposer `order-service` par Traefik (article 4e)

Traefik est installé comme dans l'article 3a, avec l'allowlist du namespace `openobserve` et le Collector `otel` en
place. Les manifestes qui contiennent un nom d'hôte, une adresse ou un e-mail sont rendus dans `deploy/helm/workspace/`
à partir de `config.env`. `SHOP_HOST` doit avoir un enregistrement `A` et `AAAA` vers le load balancer, ou être couvert
par un wildcard, et `ACME_EMAIL` doit être l'adresse du compte Let's Encrypt de l'installation de Traefik.

```bash
cd deploy/helm
cp -n config.env.example config.env   # renseigner toutes les valeurs, dont SHOP_HOST et ACME_EMAIL
./render-values.sh
kubectl apply -f traefik/order-service-rate-limit.yaml -f workspace/order-service-ingress.yaml
helm upgrade traefik traefik/traefik -n traefik --version 41.6.1 \
  -f workspace/traefik-values.yaml -f traefik/traefik-values-tracing.yaml
```

Le middleware `rate-limit` s'applique avant l'`Ingress`. `helm upgrade --reuse-values` échoue : le schéma du chart rejette
`updateStrategy.rollingUpdate: null`, d'où les deux fichiers de values. `infra/up.sh` réinstalle Traefik à chaque
rallumage : il doit recevoir les mêmes fichiers (`workspace/traefik-values.yaml` et `traefik/traefik-values-tracing.yaml`),
sinon la release perd le tracing. Le `load-generator` continue de viser le Service interne.

## Images

Le workflow `.github/workflows/images.yml` construit les trois images avec le contexte `services/<service>` et les pousse
vers `ghcr.io/clevertechware/observabilite-agents-demo/<service>`.

| Événement | Tags |
|---|---|
| push sur `main` | SHA complet du commit |
| tag git `v0.3.0` | `0.3.0` et SHA complet |

Les manifestes référencent le tag `0.3.0` : créer le tag git `v0.3.0` avant le premier déploiement. Les paquets GHCR d'un
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

## Dashboard APM d'OpenObserve (articles 5b à 5g)

`deploy/o2/dashboards/apm.dashboard.json` est écrit à la main au schéma d'export OpenObserve `version: 8` ; il n'a pas
été comparé à un export réel de l'interface. Il contient six onglets, Overview (4 tuiles), Resources (7 panneaux),
HTTP (7 panneaux), Databases (1 panneau), Go (5 panneaux) et Java (7 panneaux), et trois variables chaînées lues dans le stream `k8s_pod_cpu_usage` : `namespace_name`, `workload`
(champ `k8s_workload_name`, Deployment, StatefulSet ou DaemonSet) puis `instance`. Les panneaux de Overview et de
Resources sont en PromQL. Le panneau « Espace libre des volumes » lit `k8s_volume_available` pour les volumes
`persistentVolumeClaim`.

L'onglet HTTP (article 5d) lit deux sources avec les mêmes variables :

- la métrique `http_server_request_duration`, en PromQL avec la fenêtre `$__percentile_interval` : débit par service,
  latence P50, P90 et P99 (exemplars activés), taux de 5xx (`or vector(0)` pour une courbe à 0 sans erreur) et nombre de
  requêtes 2xx, 4xx et 5xx ;
- le stream des traces (`default`), en SQL sur les spans serveur (`span_kind = '2'`) : les 50 requêtes les plus lentes,
  les 50 dernières 5xx et le top 20 des routes en P50, P90, P95 et P99.

L'onglet Databases (article 5e) contient un tableau SQL, « Requêtes SQL les plus lentes », sur les spans clients
(`span_kind = '3'`) qui portent un texte SQL : les 100 exécutions les plus longues, une ligne par exécution, paginées
par 50 (`table_pagination_rows_per_page`). Il utilise les mêmes variables et le même drilldown que l'onglet HTTP.

L'onglet Go (article 5f) lit les métriques `go_*` que `runtime.Start()` émet pour `inventory-service`, en PromQL avec
les mêmes variables : mémoire utilisée par type, objectif du GC et limite `GOMEMLIMIT`, débit d'allocation
(`rate(go_memory_allocated[5m])`) et nombre de changements de l'objectif du GC sur 10 minutes, qui estime le nombre de
cycles. Les panneaux RSS et CPU sont ceux de l'onglet Resources. `go_memory_limit` n'existe que si `GOMEMLIMIT` est
défini : `deploy/k8s/inventory-service.yaml` le fixe à `115MiB`, soit 90 % de la limite mémoire du conteneur (128 Mi).

L'onglet Java (article 5g) lit les métriques `jvm_*` de l'agent OpenTelemetry Java, en PromQL, avec les mêmes variables :
heap par pool avec sa limite, Metaspace, Code Cache et Compressed Class Space en courbes séparées (le Metaspace inclut
déjà la Class Space), RSS et CPU du pod (copies des panneaux de Resources), nombre de pauses du GC par collecteur et par
action, part du temps passé en GC (`rate` de `jvm_gc_duration_sum`) et nombre de classes chargées. Les deux panneaux du
GC utilisent la fenêtre `[30m]` : les GC sont espacés de 62 à 96 minutes sur la démo, et à 60 s d'export une fenêtre de
30 minutes réduit l'extrapolation de `increase` à environ 3 %. L'onglet se lit sur une plage de 12 heures.

Les trois tableaux ont un drilldown « Ouvrir la trace » de type `byUrl`. L'URL doit être absolue : le JSON versionné
contient l'hôte `o2.example.com` dans les trois drilldowns (`config.drilldown[0].data.url`).
`deploy/helm/render-values.sh` le remplace par `O2_HOST` de `config.env` et écrit le dashboard à importer dans
`deploy/helm/workspace/apm.dashboard.json`. Les spans de Traefik ont `traefik` pour namespace et pour workload : le
filtre `namespace_name = demo` les écarte.

Prérequis : OpenObserve v1.1.1 et les métriques `k8s_pod_*`, `container_fs_*` et `http_server_request_duration_*`
(articles 3c et 4), ainsi que `k8s_volume_*` et l'attribut `k8s_workload_name` (article 5c).

Import par l'interface, dans le menu Dashboards :

1. ouvrir « Import Dashboard » ;
2. dans l'onglet « File Upload / JSON », choisir `deploy/helm/workspace/apm.dashboard.json`, rendu par
   `render-values.sh` ;
3. choisir le dossier `default`, puis « Import ».

Seul cet import par fichier a été testé, sur OpenObserve v1.1.1. L'import exige un `dashboardId` dans le JSON et le
remplace par un identifiant généré. Le collage du JSON dans l'éditeur et l'onglet « URL Import » n'ont pas été testés.

## Validation des manifestes

```bash
kubeconform -strict -kubernetes-version 1.36.0 deploy/k8s/*.yaml deploy/k8s/postgres/*.yaml
helm template otel-cluster open-telemetry/opentelemetry-collector --version 0.173.1 \
  --namespace otel --values deploy/helm/otel/otel-cluster-values.yaml
actionlint .github/workflows/images.yml
```
