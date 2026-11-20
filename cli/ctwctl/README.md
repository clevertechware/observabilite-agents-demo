# ctwctl

CLI en Go des articles 6a et 6b de la série sur l'observabilité. Elle lance `kubectl` pour les applications, appelle
l'API HTTP d'OpenObserve pour les logs et l'API d'`order-service` par l'hôte public de la démo.

## Prérequis

- Go 1.27 ou supérieur
- `kubectl` dans le `PATH` pour `apps`, avec un contexte qui pointe sur le cluster de la démo
- un compte OpenObserve (e-mail et jeton) pour `o2`
- une adresse IP autorisée par l'allowlist de Traefik pour `orders`

## Build, exécution, tests

```bash
make build        # go build -o bin/ctwctl .
bin/ctwctl --help
make test         # go test ./...
make check        # go generate, go vet, go test, puis échec si un mock généré est absent du dépôt ou diffère
```

Les mocks sont générés par mockery v3.9.1 (`go run github.com/vektra/mockery/v3@v3.9.1`, configuration dans
`.mockery.yml`). Aucune installation préalable n'est nécessaire. Ils sont committés dans
`internal/commands/mocks/`.

## Configuration

Quatre niveaux se superposent, le dernier l'emportant : valeurs par défaut, fichier
`$XDG_CONFIG_HOME/ctwctl/config.toml` (sinon `~/.config/ctwctl/config.toml`), variables `CTWCTL_*`, options de la ligne
de commande posées explicitement.

| Clé              | Variable                | Défaut                        | Option        |
|------------------|-------------------------|-------------------------------|---------------|
| `o2.url`         | `CTWCTL_O2_URL`         | `https://o2.example.com`      |               |
| `o2.org`         | `CTWCTL_O2_ORG`         | `default`                     |               |
| `o2.user`        | `CTWCTL_O2_USER`        |                               |               |
| `o2.token`       | `CTWCTL_O2_TOKEN`       |                               |               |
| `shop.url`       | `CTWCTL_SHOP_URL`       | `https://shop.example.com`    |               |
| `kube.namespace` | `CTWCTL_KUBE_NAMESPACE` | `demo`                        | `--namespace` |
| `kube.context`   | `CTWCTL_KUBE_CONTEXT`   | contexte courant de `kubectl` | `--context`   |
| `output`         | `CTWCTL_OUTPUT`         | `text`                        | `-o`          |

Les hôtes par défaut sont des exemples : `o2.url` et `shop.url` se renseignent par le fichier ou par `CTWCTL_*`.

Une clé tient en un mot par segment : `_` sépare les niveaux dans les noms de variables.

```bash
ctwctl config set o2.url https://o2.example.com
ctwctl config set shop.url https://shop.example.com
ctwctl config set o2.user ctwctl@sa.internal
```

Le jeton passe de préférence par l'environnement. `config set o2.token "<jeton>"` le laisserait dans l'historique du
shell et dans les arguments du processus.

```bash
read -rs CTWCTL_O2_TOKEN && export CTWCTL_O2_TOKEN
ctwctl o2 streams
```

`config view` affiche la configuration résolue, jeton masqué.

`config set` écrit le fichier en 0600 (répertoire en 0700) et ne retient que les valeurs du fichier : les variables
d'environnement et les options n'y sont jamais copiées.

## Compte OpenObserve et Secret `o2-ctwctl`

Le dépôt ne crée pas le compte. Dans OpenObserve, IAM > Service Accounts crée un compte (par exemple
`ctwctl@sa.internal`) et affiche son jeton une seule fois. `PUT /api/default/service_accounts/{email}?rotateToken=true`
en génère un nouveau. Dans la démo, les deux valeurs sont rangées dans un Secret Kubernetes :

```bash
read -rs O2_TOKEN
kubectl create secret generic o2-ctwctl -n openobserve \
  --from-literal=email=ctwctl@sa.internal --from-literal=token="$O2_TOKEN"
unset O2_TOKEN
```

Pour lancer la CLI sans écrire le jeton dans un fichier :

```bash
export CTWCTL_O2_USER=$(kubectl -n openobserve get secret o2-ctwctl -o jsonpath='{.data.email}' | base64 -d)
export CTWCTL_O2_TOKEN=$(kubectl -n openobserve get secret o2-ctwctl -o jsonpath='{.data.token}' | base64 -d)
```

## Commandes

```text
ctwctl config set <clé> <valeur> | view
ctwctl apps list | restart <nom>
ctwctl orders create --sku <sku> [--quantity <n>] | get <id>
ctwctl o2 streams [--type logs|metrics|traces]
ctwctl o2 logs [--service s] [--grep texte] [--level error] [--since 1h] [--limit 100] [--follow]
ctwctl o2 query "<SQL>" [--since 1h] [--limit 100]
options globales : -o text|json, --context, --namespace
```

- `--namespace` filtre aussi `o2 logs` sur `k8s_namespace_name`, seulement quand l'option est posée.
- `--level` s'appuie sur `severity_number` : `trace` 1, `debug` 5, `info` 9, `warn` 13, `error` 17, `fatal` 21.
- `o2 streams` sans `--type` liste les streams de logs et de traces.
- `--follow` relit les logs toutes les 2 secondes sur une fenêtre qui recouvre le tour précédent de 10 secondes, et
  écarte les lignes déjà affichées. `Ctrl+C` termine avec le code 0.

## Sortie JSON

- `apps list` et `o2 streams` : un tableau JSON.
- `o2 logs` et `o2 query` : un objet JSON compact par ligne (JSONL), avec les noms de champs d'OpenObserve. Le format
  s'écrit au fil de l'eau avec `--follow`, se lit ligne à ligne et passe dans `jq` sans tableau englobant.
- `orders create` et `orders get` : l'objet commande, complété du champ `location` à la création.
- Les erreurs vont sur la sortie d'erreur, avec le code de sortie 1.

## Erreurs d'order-service

`orders` convertit les réponses `ProblemDetail` en erreurs typées du package `internal/shop` : `ErrInvalidRequest` (400),
`ErrNotFound` (404), `ErrInsufficientStock` (409), `ErrUpstreamFailure` (502), `ErrRateLimited` (429). Un autre statut,
par exemple le 403 de l'allowlist, donne une `StatusError`.

## Organisation

```text
main.go                       point d'entrée, interruption par Ctrl+C
internal/commands/            commandes urfave/cli, configuration koanf, interfaces côté consommateur
internal/commands/mocks/      mocks mockery v3 (kubectlRunner, logSearcher, orderClient)
internal/o2/                  client HTTP d'OpenObserve : _search, streams, retries sur 429 et 5xx
internal/shop/                client HTTP d'order-service
```
