# inventory-service

Service Go (Gin, pgx) du dépôt d'exemple de la série sur OpenTelemetry. Il gère le stock dans Postgres et appelle `pricing-service` pour obtenir le prix d'une réservation.

L'exemple montre :

- l'initialisation du SDK OpenTelemetry (ressource, exporters OTLP/HTTP via `autoexport`, propagateur W3C `TraceContext` + `Baggage`, arrêt propre) ;
- `otelgin` (span serveur et `http.server.request.duration` avec `http.route`), `otelhttp` (client sortant), `otelpgx` (SQL paramétré), `otelslog` (logs corrélés) et les métriques runtime ;
- la panne `FAULT_SLOW_QUERY_SECONDS` (`pg_sleep` paramétré) ;
- des tests avec exporters en mémoire, `ManualReader` et un vrai Postgres via testcontainers.

## Prérequis

- Go 1.27.1
- Docker (tests d'intégration et image)
- Postgres 18 avec le schéma de `schema.sql` (données de démonstration optionnelles dans `seed.sql`)

## API

| Méthode et chemin | Réponses |
|---|---|
| `POST /reservations` `{"sku":"...","quantity":n}` | 201 `{"sku","quantity","unitPrice","total"}`, 400 corps invalide, 404 SKU inconnu, 409 stock insuffisant, 500 erreur interne |
| `GET /stock/{sku}` | 200 `{"sku","quantity"}`, 404 SKU inconnu |
| `GET /healthz` | 200, non tracé |

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

Les tests d'intégration du paquet `internal/store` démarrent `postgres:18.6-trixie` avec testcontainers-go et appliquent `schema.sql` : Docker doit être disponible.

Les mocks de `internal/api/mocks` sont générés par [mockery](https://vektra.github.io/mockery/) à partir de `.mockery.yaml` :

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
internal/api                 routeur Gin et handlers
internal/pricing             client HTTP vers pricing-service
internal/store               pool pgx et repository
```
