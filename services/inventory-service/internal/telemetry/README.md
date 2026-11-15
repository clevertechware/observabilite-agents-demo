# internal/telemetry

Ce package initialise le SDK OpenTelemetry d'`inventory-service`. Il publie aussi quatre métriques du runtime Go que la
bibliothèque contrib `runtime` v0.72.0 n'émet pas.

| Fichier | Rôle |
|---|---|
| `telemetry.go` | `Setup` : ressource, propagateur, traces, métriques, logs et fonction d'arrêt |
| `runtime_gc.go` | Métriques du GC et de l'ordonnanceur lues dans `runtime/metrics` |
| `telemetry_test.go` | Propagateur, exporter inconnu, échantillonnage lu dans l'environnement |
| `runtime_gc_test.go` | Cycles, découpage du temps CPU, histogrammes et conversion de format |

## `Setup`

`main.go` appelle `Setup` au démarrage, puis `runtime.Start()` de contrib pour les métriques runtime de base. Il appelle la
fonction rendue par `Setup` à l'arrêt.

`Setup` enchaîne :

1. **La ressource.** Elle est lue dans `OTEL_SERVICE_NAME` et `OTEL_RESOURCE_ATTRIBUTES`, avec les attributs du SDK.
2. **Le propagateur.** Il compose `TraceContext` (en-tête W3C `traceparent`) et `Baggage`. Sans cet appel, le
   propagateur global d'OpenTelemetry Go ne fait rien et les traces ne se recollent pas d'un service à l'autre.
3. **Les traces.** `autoexport` choisit l'exporter d'après `OTEL_TRACES_EXPORTER`, OTLP par défaut. Le
   `TracerProvider` envoie les spans par lots.
4. **Les métriques.** `autoexport` construit le *reader* et le `MeterProvider` s'appuie dessus. Les métriques du GC
   sont branchées à cette étape (voir plus bas).
5. **Les logs.** Un `LoggerProvider` exporte par lots. `slog` écrit à la fois en JSON sur la sortie standard et vers
   OpenTelemetry, via `otelslog`.

Si une étape échoue, les providers déjà créés sont arrêtés avant de rendre l'erreur. La fonction d'arrêt vide les trois
providers.

## Pourquoi `runtime_gc.go` existe

Le runtime Go mesure lui-même son activité. Le package standard `runtime/metrics` l'expose sous des noms comme
`/gc/cycles/total:gc-cycles`. On lui passe une liste de noms, et `metrics.Read` remplit les valeurs.

Contrib `runtime` v0.72.0 convertit une partie de ces valeurs : mémoire, goroutines, allocations. Il en manque quatre
pour suivre le ramasse-miettes et l'ordonnanceur :

| Métrique OpenTelemetry | Source `runtime/metrics` | Type | Statut dans contrib v0.72.0 |
|---|---|---|---|
| `go.memory.gc.cycles` | `/gc/cycles/total:gc-cycles` | compteur entier | opt-in dans la convention, non émise |
| `go.cpu.time` | `/cpu/classes/…:cpu-seconds` | compteur flottant | opt-in dans la convention, non émise |
| `go.memory.gc.pause.duration` | `/sched/pauses/total/gc:seconds` | histogramme | opt-in dans la convention, non émise |
| `go.schedule.duration` | `/sched/latencies:seconds` | histogramme | émise seulement par un `Producer` à brancher |

Les quatre métriques reprennent les noms, unités, descriptions et attributs des conventions sémantiques v1.43.0, à
travers le package généré `go.opentelemetry.io/otel/semconv/v1.43.0/goconv`. La branche principale de contrib
ajoute `OTEL_GO_X_RUNTIME_METRICS_OPTIN`, qui n'est pas encore publiée. Une fois qu'elle le sera, ce fichier pourra
être supprimé sans toucher au dashboard OpenObserve.

### Deux mécanismes OpenTelemetry

OpenTelemetry Go offre deux façons de publier une valeur calculée ailleurs :

- **Un instrument observable avec un callback.** Le SDK appelle une fonction à chaque collecte, et cette fonction donne
  la valeur courante. Ce mécanisme sert aux compteurs et aux jauges. L'API n'a pas d'histogramme observable.
- **Un `Producer`.** Ce crochet du SDK rend directement les structures prêtes à exporter (`metricdata`). Il permet de
  publier des histogrammes.

`runtime_gc.go` utilise les deux : `registerGCMetrics` pour les deux compteurs et `runtimeHistogramProducer` pour les
deux histogrammes.

## Les compteurs : `registerGCMetrics`

### La table `cpuClasses`

Chaque ligne associe :

- un compteur du runtime, par exemple `/cpu/classes/gc/mark/assist:cpu-seconds` ;
- l'attribut `go.cpu.state`, qui vaut `user`, `gc`, `scavenge` ou `idle` ;
- l'attribut `go.cpu.detailed_state`, qui garde la classe fine, par exemple `gc/mark/assist`.

On obtient une seule métrique `go.cpu.time` avec huit séries. Le panneau regroupe ensuite les quatre classes du GC avec
`sum by (go_cpu_state)`.

La table ne contient que les classes feuilles. Le runtime expose aussi des totaux (`/cpu/classes/gc/total:cpu-seconds`,
`/cpu/classes/total:cpu-seconds`) : les inclure ferait compter deux fois le même temps dans une somme.

Le runtime surestime toutes ces classes. Une pause du GC, par exemple, compte `GOMAXPROCS` fois sa durée. Les valeurs
servent donc à comparer les classes entre elles, pas à mesurer la consommation CPU réelle du conteneur.

### La création des instruments

`goconv.NewMemoryGCCyclesObservable` et `goconv.NewCPUTimeObservable` créent un `Int64ObservableCounter` et un
`Float64ObservableCounter` avec le nom, l'unité et la description de la convention. Les valeurs rendues sont des
enveloppes, et `Inst()` donne l'instrument OpenTelemetry qu'elles contiennent.

Les ensembles d'attributs des huit classes sont construits une seule fois. `attribute.NewSet` trie et déduplique les
attributs, ce qui alloue : le faire au démarrage évite de le refaire à chaque collecte.

### Le callback

`meter.RegisterCallback` reçoit la fonction et la liste des instruments qu'elle a le droit d'observer. Le SDK ignore
une observation sur un instrument absent de cette liste.

À chaque collecte, le callback :

1. prépare neuf `metrics.Sample` : le nombre de cycles en case 0, les huit classes CPU en cases 1 à 8 ;
2. appelle `metrics.Read` une seule fois, ce qui lit les neuf valeurs au même instant ;
3. observe le nombre de cycles avec `Value.Uint64()`, puis chaque classe CPU avec `Value.Float64()` et ses attributs.

La méthode de lecture doit correspondre au type de la valeur dans le runtime. Un appel à `Uint64()` sur une valeur
flottante provoque une panique.

Un compteur observable reçoit le total depuis le démarrage du processus, pas l'incrément. Le backend calcule la
variation : les panneaux utilisent `increase(go_memory_gc_cycles[10m])` et `rate(go_cpu_time[5m])`.

## Les histogrammes : `runtimeHistogramProducer`

### L'interface `Producer`

```go
type Producer interface {
    Produce(ctx context.Context) ([]metricdata.ScopeMetrics, error)
}
```

À chaque collecte, le reader récupère les métriques du `MeterProvider`, y compris celles du callback. Il appelle
ensuite `Produce` sur chaque `Producer` branché, puis exporte le tout dans le même envoi OTLP.

### La structure

`runtimeHistogramProducer` enveloppe le `Producer` de contrib (`otelruntime.NewProducer()`), qui produit déjà
`go.schedule.duration`. Il y ajoute l'histogramme des pauses du GC. `startTime` marque le début de la fenêtre
cumulative.

### `Produce`

1. Appelle le `Producer` de contrib et garde ses `ScopeMetrics`.
2. Lit `/sched/pauses/total/gc:seconds` avec `metrics.Read`.
3. Prend le nom, l'unité et la description sur `goconv.MemoryGCPauseDuration{}`. Sa valeur zéro suffit pour ces trois
   méthodes.
4. Ajoute un `ScopeMetrics` au nom du package. Il contient une métrique, un histogramme en temporalité cumulative et un
   seul point de données sans attributs.

```
ScopeMetrics              scope …/inventory-service/internal/telemetry
 └─ Metrics               go.memory.gc.pause.duration, unité s
     └─ Histogram         temporalité cumulative
         └─ DataPoint     StartTime, Time, Count, Sum, Bounds, BucketCounts
```

L'histogramme du runtime compte toutes les pauses depuis le démarrage du processus. Chaque export envoie donc le total
depuis `startTime`. Le panneau reconstruit l'intervalle avec `increase(…_bucket[10m])` avant d'appeler
`histogram_quantile`.

`var _ sdkmetric.Producer = (*runtimeHistogramProducer)(nil)` est une vérification faite à la compilation. Si le type
ne respecte plus l'interface, le build échoue à cette ligne.

### `toHistogramDataPoint` : la conversion de format

Les deux formats ne décrivent pas les seaux de la même façon :

| | `metrics.Float64Histogram` (runtime) | `HistogramDataPoint` (OpenTelemetry) |
|---|---|---|
| Bornes | `Buckets` : N+1 bornes pour N compteurs, la première est une borne basse | `Bounds` : bornes hautes seulement |
| Nombre de compteurs | `len(Buckets) - 1` | `len(Bounds) + 1` |
| Seau `i` | `[Buckets[i], Buckets[i+1])` | `(Bounds[i-1], Bounds[i]]` |
| Infini | La dernière borne vaut souvent `+Inf`, de façon explicite | `+Inf` est implicite, après la dernière borne |
| Somme | Absente | Obligatoire |

La fonction procède en trois étapes.

**Étape 1, retirer la borne basse du premier seau :** `bounds := h.Buckets[1:]`.

**Étape 2, traiter la dernière borne.**

- Si elle vaut `+Inf`, on la retire, puisque OpenTelemetry la sous-entend :

  ```
  runtime  Buckets = [0, 1, 2, +Inf]   Counts = [3, 5, 1]
  OTel     Bounds  = [1, 2]            BucketCounts = [3, 5, 1]
  ```

- Si elle est finie, on ajoute un seau de débordement vide :

  ```
  runtime  Buckets = [0, 1, 2, 4]      Counts = [3, 5, 1]
  OTel     Bounds  = [1, 2, 4]         BucketCounts = [3, 5, 1, 0]
  ```

  `append(counts[:len(counts):len(counts)], 0)` fixe la capacité à la longueur avant l'ajout. `append` doit alors
  copier dans un nouveau tableau. Sans cette précaution, il pourrait écrire dans le tableau de `h.Counts`, qui
  appartient à l'échantillon du runtime.

**Étape 3, calculer `Count` et `Sum`.** `Count` est la somme des compteurs. Le runtime ne fournit pas de somme. Elle
est estimée en comptant chaque observation à la borne basse de son seau, `bounds[i-1]` pour le seau `i`. Le seau 0 est
ignoré, car sa borne basse est celle retirée à l'étape 1 et peut valoir `-Inf`. La somme obtenue est donc une
estimation par défaut. Les panneaux n'utilisent que `histogram_quantile`, qui ne lit pas la somme.

Le runtime ferme ses seaux à gauche, `[a, b)`, et OpenTelemetry à droite, `(a, b]`. Une valeur égale à une borne change
donc de seau. Sur environ 160 seaux, l'écart reste négligeable.

## Le branchement dans `Setup`

```go
autoexport.WithFallbackMetricProducer(func(context.Context) (sdkmetric.Producer, error) {
    return newRuntimeHistogramProducer(), nil
})
metricReader, err := autoexport.NewMetricReader(ctx)
```

`autoexport` construit le reader d'après `OTEL_METRICS_EXPORTER` et `OTEL_METRICS_PRODUCERS`.
`WithFallbackMetricProducer` déclare le `Producer` à brancher quand `OTEL_METRICS_PRODUCERS` est vide. Si la variable
est définie, ce `Producer` n'est pas utilisé et les deux histogrammes disparaissent.

Cette option modifie un état global du package `autoexport` : elle doit être appelée avant `NewMetricReader`, qui la
lit à la construction du reader.

`registerGCMetrics` a besoin d'un `Meter`. Elle est donc appelée juste après la création du `MeterProvider`, avec le
`Meter` nommé `scopeName`.

## Le déroulé d'une collecte

Toutes les 60 secondes, l'intervalle par défaut du reader périodique (`OTEL_METRIC_EXPORT_INTERVAL`) :

```
PeriodicReader
 ├─ MeterProvider → callback registerGCMetrics
 │     metrics.Read(9 noms) → go.memory.gc.cycles (1 série), go.cpu.time (8 séries)
 ├─ runtimeHistogramProducer.Produce
 │     ├─ Producer contrib → go.schedule.duration
 │     └─ metrics.Read(pauses) → toHistogramDataPoint → go.memory.gc.pause.duration
 └─ export OTLP → Collector → OpenObserve
```

OpenObserve remplace les points par des tirets bas : `go_memory_gc_cycles`, `go_cpu_time`,
`go_memory_gc_pause_duration_bucket`, `go_schedule_duration_bucket`. Les panneaux de l'onglet Go du dashboard
`deploy/o2/dashboards/apm.dashboard.json` interrogent ces noms en PromQL.

## Tests

```bash
go test ./internal/telemetry/...
```

Les tests de `runtime_gc_test.go` lisent les métriques avec un `ManualReader`. Ils vérifient que :

- le nombre de cycles augmente après un `runtime.GC()` forcé ;
- chaque classe CPU porte l'état et l'état détaillé prévus par la convention ;
- le `Producer` exporte les deux histogrammes en temporalité cumulative ;
- la conversion retire la borne basse et le `+Inf` explicite ;
- la conversion ajoute un seau de débordement vide quand la dernière borne est finie.
