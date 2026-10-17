#!/usr/bin/env bash
set -euo pipefail

helm_dir="$(cd "$(dirname "$0")" && pwd)"
config="${1:-$helm_dir/config.env}"
workspace="$helm_dir/workspace"

readonly HOST_PATTERN='^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z0-9]([a-z0-9-]*[a-z0-9])?$'
readonly IPV4_CIDR_PATTERN='^([0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]{1,2}$'
readonly IPV6_CIDR_PATTERN='^[0-9a-fA-F:]+/[0-9]{1,3}$'
readonly UUID_PATTERN='[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}'
readonly LB_IP_IDS_PATTERN="^$UUID_PATTERN,$UUID_PATTERN\$"
readonly TEMPLATES=(
  traefik/ip-allowlist.yaml
  o2/openobserve-ingress.yaml
  traefik/traefik-values.yaml
)

check() {
  local name="$1" value="$2" pattern="$3"
  if [[ ! "$value" =~ $pattern ]]; then
    echo "$name invalide : '$value'" >&2
    exit 1
  fi
}

if [[ ! -f "$config" ]]; then
  echo "config introuvable : $config (voir config.env.example)" >&2
  exit 1
fi

source "$config"

check O2_HOST "${O2_HOST:-}" "$HOST_PATTERN"
check ALLOWED_IPV4 "${ALLOWED_IPV4:-}" "$IPV4_CIDR_PATTERN"
check ALLOWED_IPV6 "${ALLOWED_IPV6:-}" "$IPV6_CIDR_PATTERN"
check PRIVATE_NETWORK_CIDR "${PRIVATE_NETWORK_CIDR:-}" "$IPV4_CIDR_PATTERN"
check PODS_CIDR "${PODS_CIDR:-}" "$IPV4_CIDR_PATTERN"
check LB_IP_IDS "${LB_IP_IDS:-}" "$LB_IP_IDS_PATTERN"

replace() {
  local from="$1" to="$2"
  content="${content//"$from"/$to}"
}

mkdir -p "$workspace"

for template in "${TEMPLATES[@]}"; do
  content="$(<"$helm_dir/$template")"
  replace "o2.example.com" "$O2_HOST"
  replace "203.0.113.0/24" "$ALLOWED_IPV4"
  replace "2001:db8:1234:5678::/64" "$ALLOWED_IPV6"
  replace "<CIDR_PRIVATE_NETWORK>" "$PRIVATE_NETWORK_CIDR"
  replace "<CIDR_PODS>" "$PODS_CIDR"
  replace "<lb-ip-ids-from-scaleway>" "$LB_IP_IDS"
  printf '%s\n' "$content" > "$workspace/$(basename "$template")"
  echo "généré : $workspace/$(basename "$template")"
done
