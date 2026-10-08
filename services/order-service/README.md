# order-service

Service Java 25 + Spring Boot 4.1.1 de la démo « observabilité des agents ». Racine de la chaîne
`client -> order-service -> inventory-service -> pricing-service`.

Le code ne contient aucune dépendance OpenTelemetry. La télémétrie est produite par l'agent
`opentelemetry-javaagent` 2.32.0, attaché à la JVM par `-javaagent`.

## Prérequis

- JDK 25 ou supérieur (le build compile avec `--release 25`)
- Docker (tests Testcontainers et build de l'image)
- Maven n'est pas nécessaire : le Maven Wrapper (`./mvnw`, Maven 3.9.16) est inclus

## Build et tests

```bash
./mvnw verify
```

Les tests lancent PostgreSQL 18.6 avec Testcontainers et simulent `inventory-service` avec WireMock.
L'agent OpenTelemetry n'est pas chargé pendant les tests.

## Lancer en local avec l'agent

```bash
./mvnw -DskipTests package
curl -fsSLo /tmp/opentelemetry-javaagent.jar \
  https://repo1.maven.org/maven2/io/opentelemetry/javaagent/opentelemetry-javaagent/2.32.0/opentelemetry-javaagent-2.32.0.jar

SPRING_DATASOURCE_URL=jdbc:postgresql://localhost:5432/orders \
SPRING_DATASOURCE_USERNAME=orders SPRING_DATASOURCE_PASSWORD=orders \
INVENTORY_URL=http://localhost:8081 \
OTEL_SERVICE_NAME=order-service \
OTEL_SEMCONV_STABILITY_OPT_IN=database \
OTEL_TRACES_EXPORTER=logging OTEL_METRICS_EXPORTER=logging OTEL_LOGS_EXPORTER=logging \
java -javaagent:/tmp/opentelemetry-javaagent.jar -jar target/order-service.jar
```

Sans `OTEL_*_EXPORTER=logging`, l'agent exporte en OTLP (`OTEL_EXPORTER_OTLP_ENDPOINT`).

## Image

```bash
docker build -t order-service:dev .
```

L'agent est téléchargé depuis Maven Central. Le build échoue si son empreinte SHA-256 diffère de
`OTEL_AGENT_SHA256` (valeur par défaut épinglée dans le Dockerfile, surchargeable avec `--build-arg`).
Les options JVM (`-javaagent`, `-XX:MaxRAMPercentage=75`, `-XX:+ExitOnOutOfMemoryError`) sont dans l'`ENTRYPOINT`.
L'image s'exécute avec l'utilisateur 10001.

## API

| Méthode | Chemin | Réponse |
|---|---|---|
| `POST` | `/orders` `{"sku": "...", "quantity": n}` | 201 + commande (`id`, `sku`, `quantity`, `unitPrice`, `total`) |
| `GET` | `/orders/{id}` | 200 + commande, 404 si absente |
| `GET` | `/healthz` | 200 |

`POST /orders` appelle `POST {INVENTORY_URL}/reservations`. Réponses d'erreur :

| Cas | Statut |
|---|---|
| Corps invalide (`sku` vide, `quantity` <= 0) | 400 |
| Inventory répond 409 (stock insuffisant) | 409 |
| Inventory répond une autre erreur, est injoignable ou dépasse le délai de lecture (5 s) | 502 |

Le schéma de la table `orders` est dans `schema.sql`. Il est appliqué au démarrage (`CREATE TABLE IF NOT EXISTS`).
Le service attend que la base `orders` et son rôle existent.

## Configuration

| Variable | Défaut | Rôle |
|---|---|---|
| `SPRING_DATASOURCE_URL` | `jdbc:postgresql://localhost:5432/orders` | JDBC de la base `orders` |
| `SPRING_DATASOURCE_USERNAME` | `orders` | rôle PostgreSQL |
| `SPRING_DATASOURCE_PASSWORD` | `orders` | mot de passe (à fournir par un Secret) |
| `INVENTORY_URL` | `http://localhost:8081` | URL de base d'`inventory-service` |
| `FAULT_HEAP_LEAK_BYTES_PER_REQUEST` | `0` | octets retenus dans le heap à chaque requête (hors `/healthz`) |
| `OTEL_*` | | configuration de l'agent |

## Panne de heap

Avec `FAULT_HEAP_LEAK_BYTES_PER_REQUEST=1048576`, chaque requête retient 1 Mio. Le heap se remplit, la JVM lève
`OutOfMemoryError` puis s'arrête (`-XX:+ExitOnOutOfMemoryError`, code de sortie 3). Avec une limite de conteneur de
768 Mi et `MaxRAMPercentage=75`, le heap est plafonné à 576 Mi : la JVM s'arrête d'elle-même avant que le noyau
atteigne la limite du conteneur.
