output "kubeconfig" {
  value     = scaleway_k8s_cluster.main.kubeconfig[0].config_file
  sensitive = true
}

output "lb_ip_id" {
  value = split("/", scaleway_lb_ip.traefik.id)[1]
}

output "lb_ip_address" {
  value = scaleway_lb_ip.traefik.ip_address
}

output "pod_cidr" {
  value = scaleway_k8s_cluster.main.pod_cidr
}

output "private_network_cidr" {
  value = scaleway_vpc_private_network.main.ipv4_subnet[0].subnet
}
