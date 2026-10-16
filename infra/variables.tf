variable "project_id" {
  type    = string
  default = null
}

variable "zone" {
  type    = string
  default = "fr-par-1"
}

variable "kubernetes_version" {
  type    = string
  default = "1.37.1"
}

variable "node_type" {
  type    = string
  default = "DEV1-M"
}

variable "node_count" {
  type    = number
  default = 2
}
