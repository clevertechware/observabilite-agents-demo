variable "zone" {
  type    = string
  default = "fr-par-1"
}

variable "kubernetes_version" {
  type    = string
  default = "1.35.3"
}

variable "node_type" {
  type    = string
  default = "DEV1-M"
}

variable "node_count" {
  type    = number
  default = 2
}
