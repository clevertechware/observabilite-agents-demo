# Infrastructure Scaleway

Terraform du cluster Kapsule décrit dans l'article « Installer OpenObserve sur Kubernetes Kapsule, à la demande » :
un Private Network, un cluster Kapsule, un pool de nœuds et une IP de load balancer réservée pour Traefik.

## Prérequis

- Terraform ou [OpenTofu](https://opentofu.org) (les commandes `tofu` sont identiques à celles de `terraform`), et la CLI `scw`
- Les identifiants Scaleway dans l'environnement :

```bash
export SCW_ACCESS_KEY=<SCW_ACCESS_KEY>
export SCW_SECRET_KEY=<SCW_SECRET_KEY>
export SCW_DEFAULT_PROJECT_ID=<SCW_PROJECT_ID>
```

## Créer le cluster

```bash
scw k8s version list
terraform init
terraform apply
terraform output -raw kubeconfig > kubeconfig.yaml
chmod 600 kubeconfig.yaml
export KUBECONFIG=$PWD/kubeconfig.yaml
```

`kubeconfig.yaml` et l'état Terraform contiennent un token : ils sont ignorés par git.

## Variables

| Variable             | Défaut     | Rôle                                                |
|----------------------|------------|-----------------------------------------------------|
| `zone`               | `fr-par-1` | Zone du pool et de l'IP (les volumes sont zonaux)   |
| `kubernetes_version` | `1.35.3`   | Version de Kubernetes                               |
| `node_type`          | `DEV1-M`   | Type des nœuds (`DEV1-L` pour la démo complète)     |
| `node_count`         | `2`        | Nombre de nœuds                                     |

## Sorties

`lb_ip_address` sert à l'enregistrement DNS. `lb_ip_id`, `private_network_cidr` et `pod_cidr` alimentent les values
Helm de Traefik.

Ne détruisez pas le cluster : `delete_additional_resources = false` laisse les volumes orphelins.
