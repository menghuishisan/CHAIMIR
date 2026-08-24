#!/usr/bin/env sh
# 本脚本把组合 links 提供的稳定 Service 地址转换为 graph-node 原生配置格式。
set -eu

: "${CHAIMIR_GRAPH_RPC_URL:?必须通过组合 links 提供 EVM RPC 地址}"
: "${CHAIMIR_GRAPH_POSTGRES_ADDRESS:?必须通过组合 links 提供图数据存储地址}"
: "${CHAIMIR_GRAPH_IPFS_ADDRESS:?必须通过组合 links 提供 IPFS 地址}"
: "${POSTGRES_PASSWORD:?必须通过组合 Secret 提供图数据存储密码}"

network="${CHAIMIR_GRAPH_NETWORK:-local}"
database="${CHAIMIR_GRAPH_POSTGRES_DB:-graph}"
user="${CHAIMIR_GRAPH_POSTGRES_USER:-postgres}"

case "${network}" in
  *[!A-Za-z0-9_-]*|'')
    echo "CHAIMIR_GRAPH_NETWORK 格式无效" >&2
    exit 22
    ;;
esac
case "${database}" in
  *[!A-Za-z0-9_-]*|'')
    echo "CHAIMIR_GRAPH_POSTGRES_DB 格式无效" >&2
    exit 22
    ;;
esac
case "${user}" in
  *[!A-Za-z0-9_.-]*|'')
    echo "CHAIMIR_GRAPH_POSTGRES_USER 格式无效" >&2
    exit 22
    ;;
esac
case "${POSTGRES_PASSWORD}" in
  *[!A-Za-z0-9_.~-]*|'')
    echo "POSTGRES_PASSWORD 必须使用 URL 安全字符" >&2
    exit 22
    ;;
esac

export POSTGRES_URL="postgresql://${user}:${POSTGRES_PASSWORD}@${CHAIMIR_GRAPH_POSTGRES_ADDRESS}/${database}"
export ETHEREUM_RPC="${network}:${CHAIMIR_GRAPH_RPC_URL}"
export IPFS="${CHAIMIR_GRAPH_IPFS_ADDRESS}"

exec graph-node "$@"
