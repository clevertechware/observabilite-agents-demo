terraform {
  required_providers {
    scaleway = {
      source  = "scaleway/scaleway"
      version = "~> 2.84"
    }
  }
}

provider "scaleway" {
  zone       = var.zone
  region     = "fr-par"
  project_id = var.project_id
}
