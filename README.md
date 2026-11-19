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
    alerts/
      rocketchat.template.json    template « Raw payload » pour le webhook entrant de Rocket.Chat (8)
      rocketchat.destination.json destination webhook vers le Service interne de Rocket.Chat, jeton fictif (8)
      http-5xx-rate.alert.json    alerte PromQL sur le taux de 5xx (8)
  helm/
    config.env.example               modèle de config.env (ignoré par git) : hôtes, e-mail ACME, plages d'adresses autorisées
    render-values.sh                 valide config.env et rend les manifestes à valeurs dans workspace/ (ignoré par git)
    o2/
      openobserve-values.yaml        release openobserve (3a, 8) : ZO_WEB_URL, ZO_SSRF_ALLOWED_HOSTS, rendu dans workspace/
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
    rocketchat/
      mongodb-community.yaml         replica set MongoDB 8.2 à un membre, géré par l'opérateur MongoDB
      rocketchat-values.yaml         release rocketchat, monolithe à une réplique, rendu dans workspace/
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

### 7. Rocket.Chat dans le namespace `rocketchat`

Traefik et le middleware `ip-allowlist` du namespace `openobserve` sont en place, comme pour la section 6. `CHAT_HOST`
doit avoir un enregistrement `A` et `AAAA` vers le load balancer, ou être couvert par un wildcard.

```bash
cd deploy/helm
./render-values.sh   # config.env doit contenir CHAT_HOST
kubectl create namespace rocketchat
helm repo add mongodb https://mongodb.github.io/helm-charts
helm repo add rocketchat https://rocketchat.github.io/helm-charts
helm repo update mongodb rocketchat
helm install mongodb-kubernetes-operator mongodb/mongodb-kubernetes -n rocketchat --version 1.15.0 --wait

kubectl -n rocketchat create secret generic mongodb-rocketchat-password \
  --from-literal=password="$(openssl rand -hex 24)"
kubectl apply -f rocketchat/mongodb-community.yaml
kubectl -n rocketchat wait mongodbcommunity/mongodb --for=jsonpath='{.status.phase}'=Running --timeout=10m

kubectl -n rocketchat create secret generic rocketchat-mongodb --from-literal=mongo-uri="$(
  kubectl -n rocketchat get secret mongodb-rocketchat-rocketchat \
    -o jsonpath='{.data.connectionString\.standard}' | base64 -d)"
kubectl -n rocketchat create secret generic rocketchat-admin \
  --from-literal=ADMIN_PASS="$(openssl rand -hex 16)" \
  --from-literal=ADMIN_EMAIL=admin@example.com \
  --from-literal=OVERWRITE_SETTING_Show_Setup_Wizard=completed

helm install rocketchat rocketchat/rocketchat -n rocketchat --version 7.1.0 -f workspace/rocketchat-values.yaml
kubectl -n rocketchat rollout status deployment/rocketchat-rocketchat --timeout=8m

kubectl -n rocketchat get secret rocketchat-admin -o jsonpath='{.data.ADMIN_PASS}' | base64 -d
```

L'opérateur publie la chaîne de connexion dans le Secret `mongodb-rocketchat-rocketchat`, mais le chart lit la clé
`mongo-uri`, d'où la copie dans `rocketchat-mongodb`. Le compte `admin` est créé au premier démarrage ; son mot de passe
se relit dans le Secret `rocketchat-admin`. `OVERWRITE_SETTING_Show_Setup_Wizard=completed` ferme l'assistant de
configuration, qui exige sinon l'enregistrement de l'espace auprès de Rocket.Chat Cloud. Une instance non enregistrée a
les limites de l'offre Community (aucune app privée activable, 5 apps de la Marketplace).

Après un changement du mot de passe dans `mongodb-rocketchat-password`, l'opérateur met à jour l'utilisateur et
`mongodb-rocketchat-rocketchat`, pas la copie `rocketchat-mongodb` : la recréer, puis redémarrer Rocket.Chat.

### 8. Alertes OpenObserve vers Rocket.Chat

Une alerte `scheduled` d'OpenObserve évalue le taux de réponses HTTP 5xx de la démo et poste dans le salon `#alertes`
de Rocket.Chat par un webhook entrant. Les définitions JSON sont dans `deploy/o2/alerts/` :

| Fichier | Objet OpenObserve |
|---|---|
| `rocketchat.template.json` | template « Raw payload » (`kind: custom`) : `text` et `attachments` du webhook entrant |
| `rocketchat.destination.json` | destination webhook générique, `POST` vers l'URL interne du Service Rocket.Chat |
| `http-5xx-rate.alert.json` | alerte PromQL, période de 5 minutes, évaluation chaque minute, silence de 10 minutes |

Les sections 1 à 7 sont en place et le Collector envoie les métriques `http_server_request_duration`. Les commandes
se lancent depuis la racine du dépôt, dans un même shell : les blocs partagent leurs variables.

**1. OpenObserve : adresse publique et exception SSRF.** OpenObserve refuse les destinations internes, dont les noms
en `.local` comme `*.svc.cluster.local`. `ZO_SSRF_ALLOWED_HOSTS` (absent de la documentation, lu dans le code de v1.1.1)
autorise des noms précis sans désactiver le garde-fou. Le chart ne rend dans sa ConfigMap que les clés qu'il connaît :
`ZO_WEB_URL` passe par `config`, `ZO_SSRF_ALLOWED_HOSTS` par `extraEnv`. `ZO_WEB_URL` fixe l'hôte de `{alert_url}`,
`http://localhost:5080` par défaut. `openobserve-values.yaml` fait partie des fichiers rendus par `render-values.sh`,
d'où le `./render-values.sh` avant le `helm upgrade`. Le pod redémarre, environ 50 secondes.

```bash
(cd deploy/helm && ./render-values.sh)
helm repo add openobserve https://charts.openobserve.ai
helm upgrade o2 openobserve/openobserve-standalone --version 1.0.1 -n openobserve \
  -f deploy/helm/workspace/openobserve-values.yaml
kubectl -n openobserve rollout status statefulset/o2-openobserve-standalone
kubectl -n openobserve wait pod/o2-openobserve-standalone-0 --for=condition=Ready --timeout=3m
```

Sans `ZO_SSRF_ALLOWED_HOSTS`, la création de la destination échoue :
`Destination URL blocked by SSRF guard: Access to internal domain rocketchat-rocketchat.rocketchat.svc.cluster.local is
not allowed for security reasons`. Une destination créée avant la suppression de la variable reste enregistrée, mais
l'envoi échoue avec le même message.

**2. Rocket.Chat : bot, salon et webhook entrant.** L'utilisateur du webhook doit avoir la permission
`message-impersonate`, que le rôle `bot` détient. `BOT_EMAIL` doit appartenir à un domaine de
`Accounts_AllowedDomainsList` quand ce réglage est renseigné : l'adresse doit alors utiliser l'un des domaines
autorisés. Le webhook est créé au nom du
bot, dans `#alertes`. Son URL est la seule authentification : elle contient un identifiant et un jeton, qui ne doivent
pas être versionnés.

```bash
source deploy/helm/config.env
BOT_EMAIL="openobserve-bot@<domaine-autorisé>"
RC_URL="https://$CHAT_HOST"
RC_ADMIN_PASS="$(kubectl -n rocketchat get secret rocketchat-admin -o jsonpath='{.data.ADMIN_PASS}' | base64 -d)"
read -r RC_TOKEN RC_USER_ID < <(printf %s "$RC_ADMIN_PASS" \
  | curl -sS --fail-with-body "$RC_URL/api/v1/login" -d user=admin --data-urlencode "password@-" | jq -r '.data | "\(.authToken) \(.userId)"')
unset RC_ADMIN_PASS
rc() {
  printf %s "${3-}" | curl -sS --fail-with-body -X "$1" "$RC_URL$2" \
    -H @<(printf 'X-Auth-Token: %s\nX-User-Id: %s\n' "$RC_TOKEN" "$RC_USER_ID") \
    -H 'Content-Type: application/json' ${3:+-d @-}
}

rc POST /api/v1/users.create "$(BOT_PASS="$(openssl rand -hex 24)" jq -n --arg email "$BOT_EMAIL" \
  '{name: "OpenObserve", username: "openobserve-bot", email: $email, password: $ENV.BOT_PASS, roles: ["bot"],
    verified: true, requirePasswordChange: false, sendWelcomeEmail: false, joinDefaultChannels: false}')" \
  | jq -c .success
rc POST /api/v1/channels.create '{"name": "alertes", "members": ["openobserve-bot"]}' | jq -c .success
HOOK_PATH="$(rc POST /api/v1/integrations.create '{"type": "webhook-incoming", "name": "OpenObserve", "enabled": true,
  "username": "openobserve-bot", "channel": "#alertes", "scriptEnabled": false, "alias": "OpenObserve",
  "emoji": ":rotating_light:"}' | jq -r '.integration | "/hooks/\(._id)/\(.token)"')"
```

**3. OpenObserve : template, destination et alerte.** L'URL de la destination est celle du Service
(`rocketchat-rocketchat`, port 80), sans passer par Traefik ni par l'allowlist d'adresses. Le fichier versionné porte un
jeton fictif, remplacé à l'envoi. L'utilisateur est celui du Secret `o2-root-user`.

```bash
O2_URL="https://$O2_HOST"
O2_AUTH="$(kubectl -n openobserve get secret o2-root-user -o jsonpath='{.data.ZO_ROOT_USER_EMAIL}' | base64 -d):$(
  kubectl -n openobserve get secret o2-root-user -o jsonpath='{.data.ZO_ROOT_USER_PASSWORD}' | base64 -d)"
o2() { curl -sS --fail-with-body -K <(printf 'user = "%s"\n' "$O2_AUTH") -H 'Content-Type: application/json' "$@"; echo; }

o2 -X POST "$O2_URL/api/default/alerts/templates" -d @deploy/o2/alerts/rocketchat.template.json
jq --arg url "http://rocketchat-rocketchat.rocketchat.svc.cluster.local$HOOK_PATH" '.url = $url' \
  deploy/o2/alerts/rocketchat.destination.json | o2 -X POST "$O2_URL/api/default/alerts/destinations" -d @-
o2 -X POST "$O2_URL/api/v2/default/alerts" -d @deploy/o2/alerts/http-5xx-rate.alert.json
```

Une mise à jour passe par `PUT /api/default/alerts/templates/<nom>` ou `PUT /api/v2/default/alerts/<id>` : un `POST`
sur un nom existant répond `Template with the same name already exists`.

**4. Déclencher l'alerte.** 50 % d'erreurs sur `pricing-service` (voir « Piloter les pannes ») font dépasser le seuil de
5 % dès que les métriques arrivent dans OpenObserve. Le message arrive dans `#alertes` environ trois minutes après la
panne.

```bash
START="$(date -u +%Y-%m-%dT%H:%M:%S.000Z)"
kubectl -n demo set env deploy/pricing-service FAULT_ERROR_RATE=0.5
wait_for_alert() {
  for _ in $(seq 40); do
    rc GET "/api/v1/channels.history?roomName=alertes&count=1" \
      | jq -e --arg start "$START" '.messages[0] | select(.ts > $start) | .msg | test("firing")' > /dev/null \
      && return 0
    sleep 15
  done
  echo "Échec : aucun message « firing » dans #alertes après 10 minutes" >&2
  return 1
}
wait_for_alert; STATUS=$?
kubectl -n demo set env deploy/pricing-service FAULT_ERROR_RATE=0
[ "$STATUS" -eq 0 ] && rc GET "/api/v1/channels.history?roomName=alertes&count=1" | jq '.messages[0] | {msg, attachments}'
```

**Constats sur OpenObserve v1.1.1 et Rocket.Chat 9.0.0**

- `period`, `frequency` et `silence` sont en minutes dans l'API v2 (`frequency_type: minutes`). Avec `frequency: 1`,
  le scheduler évalue l'alerte chaque minute. Après une notification, il n'évalue plus l'alerte pendant `silence`
  minutes, puis notifie de nouveau si la condition tient toujours.
- Pour une alerte PromQL, `{alert_threshold}` rend le `threshold` de `trigger_condition` (ici `1`, le nombre de séries
  attendu), pas le seuil de `promql_condition` (`0.05`). Le template ne l'affiche donc pas ; le seuil figure dans la
  description de l'alerte.
- `{alert_agg_value}` contient la valeur de la requête PromQL (`0.186…`) au tir réel et reste vide au déclenchement
  manuel (`PATCH /api/v2/default/alerts/<id>/trigger`). Rocket.Chat refuse alors le champ d'attachement sans valeur :
  `Invalid attachment field, title and value is required`, et OpenObserve répond `sent error status: 400 Bad Request`.
- `POST /api/default/alerts/destinations/<nom>/test_send` rend un template de type `content`, pas le template `custom` :
  la réponse est `sent status: 200 OK`, mais Rocket.Chat affiche un message vide.
- Les logs d'OpenObserve contiennent la charge JSON envoyée et la réponse de Rocket.Chat quand l'envoi échoue.
- Le tableau des destinations de l'interface et `GET /api/default/alerts/destinations` renvoient l'URL en clair, jeton
  compris.
- Le trafic OpenObserve vers le Service ne passe pas par Traefik. Un appel au webhook par l'URL publique laisse le jeton
  dans le champ `RequestPath` de l'access log JSON de Traefik, donc dans les logs collectés par OpenObserve.

**Nettoyage**

```bash
ALERT_ID="$(o2 -X GET "$O2_URL/api/v2/default/alerts" | jq -r '.list[] | select(.name == "http-5xx-rate") | .alert_id')"
o2 -X DELETE "$O2_URL/api/v2/default/alerts/$ALERT_ID"
o2 -X DELETE "$O2_URL/api/default/alerts/destinations/rocketchat-alertes"
o2 -X DELETE "$O2_URL/api/default/alerts/templates/rocketchat-raw"
INTEGRATION_ID="$(rc GET /api/v1/integrations.list | jq -r '.integrations[] | select(.name == "OpenObserve") | ._id')"
rc POST /api/v1/integrations.remove \
  "{\"type\": \"webhook-incoming\", \"integrationId\": \"$INTEGRATION_ID\"}" | jq -c .success
rc POST /api/v1/channels.delete '{"roomName": "alertes"}' | jq -c .success
rc POST /api/v1/users.delete '{"username": "openobserve-bot"}' | jq -c .success
rc POST /api/v1/logout > /dev/null
unset RC_TOKEN RC_USER_ID O2_AUTH HOOK_PATH
```

## Images

Le workflow `.github/workflows/images.yml` construit les trois images avec le contexte `services/<service>` et les pousse
vers `ghcr.io/clevertechware/observabilite-agents-demo/<service>`.

| Événement | Tags |
|---|---|
| push sur `main` | SHA complet du commit |
| tag git `v0.4.0` | `0.4.0` et SHA complet |

Les manifestes référencent le tag `0.3.0`, sauf `inventory-service` en `0.4.0` (métriques du GC, article 5f) : créer
les tags git `v0.3.0` et `v0.4.0` avant le premier déploiement. Les paquets GHCR d'un
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

L'onglet Go (article 5f) lit les métriques `go_*` d'`inventory-service`, en PromQL avec les mêmes variables : mémoire
utilisée par type, objectif du GC et limite `GOMEMLIMIT`, débit d'allocation (`rate(go_memory_allocated[5m])`), cycles
de GC sur 10 minutes (`increase(go_memory_gc_cycles[10m])`), temps CPU du runtime par état (`go_cpu_time` hors `idle`),
pauses du GC (P50 et P99 de `go_memory_gc_pause_duration`) et attente des goroutines avant exécution (P99 de
`go_schedule_duration`). Les quatre dernières viennent de `internal/telemetry/runtime_gc.go`, à partir de l'image
`0.4.0`. Les panneaux RSS et CPU sont ceux de l'onglet Resources. `go_memory_limit` n'existe que si `GOMEMLIMIT` est
défini : `deploy/k8s/inventory-service.yaml` le fixe à `27MiB`, soit 90 % de la limite mémoire du conteneur (30 Mi).

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
