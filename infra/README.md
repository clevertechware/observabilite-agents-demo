# Infrastructure Scaleway

OpenTofu du cluster Kapsule décrit dans l'article « Installer OpenObserve sur Kubernetes Kapsule, à la demande » :
un Private Network, un cluster Kapsule, un pool de nœuds et deux IP de load balancer réservées pour Traefik (IPv4 et
IPv6).

## Prérequis

- [OpenTofu](https://opentofu.org) et la CLI `scw` (Terraform accepte les mêmes fichiers et les mêmes commandes)
- Les identifiants Scaleway dans l'environnement (`SCW_DEFAULT_PROJECT_ID` est facultatif si `project_id` est passé en
  variable) :

```bash
export SCW_ACCESS_KEY=<SCW_ACCESS_KEY>
export SCW_SECRET_KEY=<SCW_SECRET_KEY>
export SCW_DEFAULT_PROJECT_ID=<SCW_PROJECT_ID>
```

## Créer le cluster

```bash
scw k8s version list
tofu init
tofu apply
tofu output -raw kubeconfig > kubeconfig.yaml
chmod 600 kubeconfig.yaml
export KUBECONFIG=$PWD/kubeconfig.yaml
```

`kubeconfig.yaml` et l'état OpenTofu contiennent un token : ils sont ignorés par git.

## Éteindre et rallumer

```bash
./down.sh
./up.sh chemin/vers/traefik-values.yaml
```

`down.sh` désinstalle Traefik, ce qui supprime le load balancer, puis supprime le pool de nœuds (`running=false`). Le
cluster, les volumes et les IP réservées restent. `up.sh` recrée le pool puis réinstalle Traefik avec ses values
(`traefik-values.yaml` dans le répertoire courant par défaut). Les deux scripts attendent `KUBECONFIG` et les
identifiants Scaleway dans l'environnement.

## Variables

| Variable             | Défaut     | Rôle                                                |
|----------------------|------------|-----------------------------------------------------|
| `project_id`         | `null`     | Projet Scaleway ; sinon `SCW_DEFAULT_PROJECT_ID`    |
| `zone`               | `fr-par-1` | Zone du pool et de l'IP (les volumes sont zonaux)   |
| `kubernetes_version` | `1.35.3`   | Version de Kubernetes                               |
| `node_type`          | `DEV1-M`   | Type des nœuds (`DEV1-L` pour la démo complète)     |
| `running`            | `true`     | `false` supprime le pool de nœuds, le cluster reste |
| `node_count`         | `2`        | Nombre de nœuds                                     |

## Sorties

`lb_ip_address` sert à l'enregistrement DNS `A`, `lb_ipv6_address` à l'enregistrement `AAAA`. `lb_ip_id`,
`lb_ipv6_id`, `private_network_cidr` et `pod_cidr` alimentent les values Helm de Traefik. L'annotation
`service.beta.kubernetes.io/scw-loadbalancer-ip-ids` prend les deux identifiants : `<lb_ip_id>,<lb_ipv6_id>`.

Ne détruisez pas le cluster : `delete_additional_resources = false` laisse les volumes orphelins.
