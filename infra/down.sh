#!/usr/bin/env bash
set -euo pipefail

infra_dir="$(cd "$(dirname "$0")" && pwd)"

helm uninstall traefik --namespace traefik
tofu -chdir="$infra_dir" apply -auto-approve -var running=false
