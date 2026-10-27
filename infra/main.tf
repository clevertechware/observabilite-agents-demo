resource "scaleway_vpc_private_network" "main" {
  name = "observabilite"
}

resource "scaleway_k8s_cluster" "main" {
  name                        = "observabilite"
  type                        = "kapsule"
  version                     = var.kubernetes_version
  cni                         = "cilium"
  private_network_id          = scaleway_vpc_private_network.main.id
  delete_additional_resources = false
}

resource "scaleway_k8s_pool" "main" {
  count      = var.running ? 1 : 0
  cluster_id = scaleway_k8s_cluster.main.id
  name       = "main"
  node_type  = var.node_type
  size       = var.node_count
  zone       = var.zone
}

resource "scaleway_lb_ip" "traefik" {
  zone = var.zone
}

resource "scaleway_lb_ip" "traefik_v6" {
  zone    = var.zone
  is_ipv6 = true
}

resource "scaleway_k8s_acl" "api" {
  count      = length(var.api_allowed_cidrs) > 0 ? 1 : 0
  cluster_id = scaleway_k8s_cluster.main.id

  dynamic "acl_rules" {
    for_each = var.api_allowed_cidrs
    content {
      ip = acl_rules.value
    }
  }
}
