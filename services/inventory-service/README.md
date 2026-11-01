# inventory-service

Service Go (Gin, pgx) du dépôt d'exemple de la série sur OpenTelemetry. Il gère le stock dans Postgres et appelle `pricing-service` pour obtenir le prix d'une réservation.

L'exemple montre :

- l'initialisation du SDK OpenTelemetry (ressource, exporters OTLP/HTTP via `autoexport`, propagateur W3C `TraceContext` + `Baggage`, arrêt propre) ;
- `otelgin` (span serveur et `http.server.request.duration` avec `http.route`), `otelhttp` (client sortant), `otelpgx` (SQL paramétré), `otelslog` (logs corrélés) et les métriques runtime ;
- la panne `FAULT_SLOW_QUERY_SECONDS` (`pg_sleep` paramétré) ;
- des tests avec exporters en mémoire, `ManualReader` et un vrai Postgres via testcontainers.

## Prérequis

- Go 1.27.3
- Docker (tests d'intégration et image)
- Postgres 18 avec le schéma de `schema.sql` (données de démonstration optionnelles dans `seed.sql`)

## Lancement local avec Docker Compose

Le socle mutualisé (Postgres + Collector OpenTelemetry) est dans `services/compose.yaml` ; ce
dossier ne contient plus que le service Go. Depuis la racine du dépôt, avec Docker Compose
v2.20 ou supérieur :

```bash
docker compose up --build inventory-service
```

Ou depuis ce dossier, le `compose.yaml` local inclut le socle :

```bash
docker compose up --build
```

Compose démarre Postgres sur `127.0.0.1:5432` (base `inventory`, schéma et SKU de `schema.sql`
et `seed.sql` chargés au premier démarrage), le Collector OTLP/HTTP sur `http://localhost:4318`
et le conteneur Go, publié sur `127.0.0.1:8081`. Les identifiants `inventory` / `inventory` sont
réservés au développement local. Aucun fichier `.env` n'est nécessaire. Les ports publiés par
Compose ne sont accessibles que depuis la machine hôte.

`PRICING_URL` pointe par défaut sur le conteneur `pricing-service`
(`docker compose up --build` à la racine lance toute la chaîne, pricing compris). Pour appeler
un `pricing-service` lancé sur la machine hôte (voir ci-dessous) :

```bash
PRICING_URL=http://host.docker.internal:8081 docker compose up --build inventory-service
```

Vérification depuis la machine hôte, une fois la stack démarrée :

```bash
curl --fail http://localhost:8081/stock/SKU-001
curl --fail http://localhost:8081/reservations \
  -H 'Content-Type: application/json' \
  -d '{"sku":"SKU-001","quantity":2}'
docker compose logs -f otel-collector
```

Ou avec [HTTPie](https://httpie.io) :

```bash
http :8081/healthz
http :8081/stock/SKU-001
http :8081/reservations sku=SKU-001 quantity:=2
```

Le Collector affiche la télémétrie reçue dans ses logs ; aucun compte OpenObserve n'est requis.
`docker compose down` arrête les conteneurs et conserve les données. Pour réinitialiser le
stock et rejouer les scripts, `docker compose down -v` **supprime toutes les données locales**.

## Lancer inventory-service directement sur la machine hôte

Lancer d'abord uniquement le socle (Postgres + Collector), depuis la racine du dépôt :

```bash
docker compose -f services/compose.yaml up -d --wait
```

`up --wait` attend que Postgres soit `healthy` avant de rendre la main. Le schéma et les données
de démonstration sont chargés au premier démarrage.

Deux choix pour `pricing-service` :

- en conteneur : `docker compose up --build pricing-service`, puis `export PRICING_URL=http://localhost:8082` ;
- sur la machine hôte : `export LISTEN_ADDR=127.0.0.1:8081` dans `../pricing-service` puis
  `cargo run --release --locked`, et `export PRICING_URL=http://localhost:8081` (Rust 1.95+ requis).

Puis lancer Go depuis `services/inventory-service`, avec ces variables d'environnement
(également utilisables dans la configuration d'exécution de l'IDE, voir `.run/Run.run.xml`) :

```bash
export DATABASE_URL='postgres://inventory:inventory@localhost:5432/inventory?sslmode=disable'
export PRICING_URL=http://localhost:8081
export PORT=8080
export FAULT_SLOW_QUERY_SECONDS=0
export OTEL_SERVICE_NAME=inventory-service
export OTEL_RESOURCE_ATTRIBUTES=service.namespace=shop,deployment.environment.name=local
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
export GIN_MODE=debug
go run .
```

Les commandes `curl` ci-dessus restent valables avec `localhost:8080`. Pour consulter la
télémétrie ou arrêter les dépendances :

```bash
docker compose -f services/compose.yaml logs -f otel-collector
docker compose -f services/compose.yaml down
```

Arrêter Go et Rust séparément avec `Ctrl+C`. L'arrêt conserve les données ;
`docker compose -f services/compose.yaml down -v` **supprime les données du socle**.
Depuis l'hôte, toutes les URLs utilisent `localhost`, sans noms DNS Docker.

## API

| Méthode et chemin | Réponses |
|---|---|
| `POST /reservations` `{"sku":"...","quantity":n}` | 201 `{"sku","quantity","unitPrice","total"}`, 400 erreur de décodage, 422 données invalides, 404 SKU inconnu, 409 stock insuffisant, 500 erreur interne |
| `GET /stock/{sku}` | 200 `{"sku","quantity"}`, 404 SKU inconnu, 500 erreur interne |
| `GET /healthz` | 200, non tracé |

Les erreurs de ces endpoints d'inventaire ont un corps JSON `{"error":"..."}` : `invalid reservation request` (400 pour un corps vide, un JSON mal formé ou un type incorrect ; 422 pour un SKU absent ou vide, ou une quantité absente, nulle ou négative), `unknown sku` (404), `insufficient stock` (409) ou `internal server error` (500). Les détails techniques des erreurs internes restent dans les logs. La quantité réservée doit être strictement positive ; cette règle est vérifiée par le binding HTTP et par le cas d'usage.

`pricing-service` doit répondre à `GET /prices/{sku}?quantity=n` par `{"unitPrice": 12.5, "total": 25}` (200) ou 404.

## Variables d'environnement

| Variable | Défaut | Rôle |
|---|---|---|
| `PRICING_URL` | obligatoire | URL de base de `pricing-service` |
| `DATABASE_URL` | vide | URL de connexion Postgres. Si absente, pgx lit `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDATABASE`, `PGSSLMODE` |
| `PORT` | `8080` | Port d'écoute |
| `FAULT_SLOW_QUERY_SECONDS` | `0` | Durée du `pg_sleep` ajouté à la lecture du stock (décimal accepté) |
| `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES` | | Ressource OpenTelemetry |
| `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL` | | Lus par `autoexport` (`http/protobuf` pour OTLP/HTTP) |
| `GIN_MODE` | `release` dans l'image | Mode de Gin |

## Build, run, tests

```bash
go build ./...
go vet ./... && golangci-lint run ./...
go test ./...

docker build -t inventory-service .

export PRICING_URL=http://localhost:8081
export DATABASE_URL=postgres://inventory:inventory@localhost:5432/inventory
export OTEL_SERVICE_NAME=inventory-service
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
go run .
```

Les tests d'intégration du paquet `internal/inventory/repositories` démarrent `postgres:18.6-trixie` avec testcontainers-go et appliquent `schema.sql` : Docker doit être disponible.

La configuration de `golangci-lint` (v2) est dans `.golangci.yml` : linters standard, `revive` (règles de `golint` hors commentaires de documentation), `errorlint`, `bodyclose` et `misspell`.

Les mocks de `internal/inventory/mocks` sont générés par [mockery](https://vektra.github.io/mockery/) à partir de `.mockery.yaml` :

```bash
go run github.com/vektra/mockery/v2@v2.53.8
```

## Structure

```
main.go                      assemblage, signaux, arrêt
schema.sql                   table stock
seed.sql                     SKU de démonstration
internal/config              variables d'environnement
internal/telemetry           SDK OpenTelemetry
internal/inventory           cas d'usage, erreurs métier, routeur Gin et handlers HTTP
internal/inventory/repositories  pool pgx et repository
internal/inventory/mocks     mocks des interfaces d'inventaire
internal/inventory/gateway   client HTTP vers pricing-service
```
