#!/usr/bin/env bash
# configure-docker-desktop-registry 文件负责把统一 Harbor 域名绑定到
# Docker Desktop 节点的宿主机网关,使节点内的 digest 拉取仍走 HTTPS 443 入口。
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
config_path="${2:-$script_dir/../config/chaimir.env}"
if [[ ! -f "$config_path" ]]; then
  echo "缺少统一部署配置: $config_path" >&2
  exit 2
fi

declare -A config_values
while IFS= read -r line || [[ -n "$line" ]]; do
  line="${line%$'\r'}"
  trimmed="$(printf '%s' "$line" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
  if [[ -z "$trimmed" || "$trimmed" == \#* ]]; then
    continue
  fi
  if [[ ! "$trimmed" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]]; then
    echo "统一部署配置包含非法行" >&2
    exit 2
  fi
  key="${BASH_REMATCH[1]}"
  value="${BASH_REMATCH[2]}"
  if [[ ${config_values[$key]+_} ]]; then
    echo "统一部署配置包含重复键: $key" >&2
    exit 2
  fi
  config_values[$key]="$value"
done < "$config_path"

for key in SUPPLY_CHAIN_REGISTRY SUPPLY_CHAIN_REGISTRY_ENDPOINT IMAGE_REGISTRY SUPPLY_CHAIN_HARBOR_EXTERNAL_URL; do
  if [[ -z "${config_values[$key]:-}" ]]; then
    echo "统一部署配置缺少必填键: $key" >&2
    exit 2
  fi
done

registry_host="${config_values[SUPPLY_CHAIN_REGISTRY]}"
registry_endpoint="${config_values[SUPPLY_CHAIN_REGISTRY_ENDPOINT]}"
image_registry="${config_values[IMAGE_REGISTRY]}"
external_url="${config_values[SUPPLY_CHAIN_HARBOR_EXTERNAL_URL]}"
if [[ ! "$registry_host" =~ ^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$ ]]; then
  echo "registry 主机名非法" >&2
  exit 2
fi
if [[ "$registry_endpoint" != "$registry_host" || "$image_registry" != "$registry_host" || "$external_url" != "https://$registry_host" ]]; then
  echo "统一部署配置中的 Registry 地址不一致或不是 HTTPS 根地址" >&2
  exit 2
fi

mapfile -t nodes < <(kubectl --request-timeout=30s get nodes -o jsonpath='{.items[*].metadata.name}' | tr ' ' '\n' | sed '/^$/d')
if ((${#nodes[@]} == 0)); then
  echo "当前 Kubernetes 集群没有节点" >&2
  exit 1
fi

for node in "${nodes[@]}"; do
  # Docker Desktop may return an IPv6 record first; registry split-DNS must use
  # the reachable IPv4 host gateway on the node's IPv4 bridge.
  gateway="$(docker exec "$node" getent ahostsv4 host.docker.internal | awk 'NR == 1 {print $1}')"
  if [[ ! "$gateway" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
    echo "节点 $node 无法解析 host.docker.internal" >&2
    exit 1
  fi
  docker exec "$node" bash -ceu '
    host="$1"
    gateway="$2"
    tmp="$(mktemp)"
    awk -v host="$host" '\''$2 != host {print}'\'' /etc/hosts >"$tmp"
    printf "%s\t%s\n" "$gateway" "$host" >>"$tmp"
    cat "$tmp" >/etc/hosts
    rm -f "$tmp"
  ' bash "$registry_host" "$gateway"
  echo "$node: $registry_host -> $gateway"
done

for node in "${nodes[@]}"; do
  gateway="$(docker exec "$node" getent ahostsv4 host.docker.internal | awk 'NR == 1 {print $1}')"
  resolved="$(docker exec "$node" getent ahostsv4 "$registry_host" | awk 'NR == 1 {print $1}')"
  if [[ "$resolved" != "$gateway" ]]; then
    echo "节点 $node 的 $registry_host split-DNS 校验失败" >&2
    exit 1
  fi
done
