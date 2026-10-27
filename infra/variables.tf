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

variable "running" {
  type    = bool
  default = true
}

variable "node_count" {
  type    = number
  default = 2
}

variable "api_allowed_cidrs" {
  type    = list(string)
  default = []

  validation {
    condition     = alltrue([for cidr in var.api_allowed_cidrs : can(cidrhost(cidr, 0))])
    error_message = "api_allowed_cidrs must only contain CIDR blocks, e.g. 203.0.113.42/32."
  }
}
