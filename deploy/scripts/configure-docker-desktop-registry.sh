#!/usr/bin/env bash
# configure-docker-desktop-registry 文件负责把 canonical Harbor 域名绑定到
# Docker Desktop 节点的宿主机网关,使节点内的 digest 拉取仍走 registry.chaimir.io:443。
set -euo pipefail

registry_host="${1:-registry.chaimir.io}"
if [[ ! "$registry_host" =~ ^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$ ]]; then
  echo "registry 主机名非法" >&2
  exit 2
fi

mapfile -t nodes < <(kubectl get nodes -o jsonpath='{.items[*].metadata.name}' | tr ' ' '\n' | sed '/^$/d')
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
