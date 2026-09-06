#!/usr/bin/env sh
# 本脚本为 Chainlink 生成运行期配置并启动官方节点,不把连接串、凭据或密钥写入镜像层。
set -eu

# 镜像自检/预拉取命令显式绕过运行期配置生成,保持 selftest 可在无 Secret 环境复现。
if [ "$#" -gt 0 ]; then
  case "$1" in
    chainlink)
      shift
      exec /usr/local/bin/chainlink "$@"
      ;;
    --version)
      exec /usr/local/bin/chainlink --version
      ;;
  esac
fi

: "${ETH_URL:?必须通过 execution_chain binding 提供 EVM HTTP RPC}"
: "${CHAIMIR_CHAINLINK_DATABASE_ADDRESS:?必须通过 database binding 提供 PostgreSQL 地址}"
: "${CHAIMIR_CHAINLINK_DATABASE_NAME:?必须提供 Chainlink 数据库名}"
: "${CHAIMIR_CHAINLINK_DATABASE_USER:?必须提供 Chainlink 数据库用户}"
: "${CHAINLINK_CHAIN_ID:?必须通过组合参数提供 EVM chain id}"
: "${POSTGRES_PASSWORD:?必须通过数据库 Secret 提供 PostgreSQL 密码}"
: "${CHAINLINK_KEYSTORE_PASSWORD:?必须通过 Secret 提供 Chainlink keystore 密码}"
: "${CHAINLINK_API_EMAIL:?必须通过 Secret 提供 Chainlink API 邮箱}"
: "${CHAINLINK_API_PASSWORD:?必须通过 Secret 提供 Chainlink API 密码}"

case "$ETH_URL" in
  http://*|https://*) ;;
  *) echo "ETH_URL 必须使用 HTTP(S) 协议" >&2; exit 2 ;;
esac
case "$POSTGRES_PASSWORD" in
  *[!A-Za-z0-9_.~-]*|'') echo "POSTGRES_PASSWORD 必须使用 URL 安全字符" >&2; exit 2 ;;
esac
if [ "${#POSTGRES_PASSWORD}" -lt 16 ] || [ "${#POSTGRES_PASSWORD}" -gt 50 ]; then
  echo "POSTGRES_PASSWORD 长度必须在 16 到 50 个字符之间" >&2
  exit 2
fi
if [ "${#CHAINLINK_KEYSTORE_PASSWORD}" -lt 16 ] || [ "${#CHAINLINK_KEYSTORE_PASSWORD}" -gt 50 ]; then
  echo "CHAINLINK_KEYSTORE_PASSWORD 长度必须在 16 到 50 个字符之间" >&2
  exit 2
fi
case "$CHAINLINK_API_EMAIL" in
  *[!A-Za-z0-9._%+-@]*|'') echo "CHAINLINK_API_EMAIL 格式无效" >&2; exit 2 ;;
esac
case "$CHAINLINK_API_PASSWORD" in
  *[!A-Za-z0-9_.~-]*|'') echo "CHAINLINK_API_PASSWORD 必须使用 URL 安全字符" >&2; exit 2 ;;
esac
if [ "${#CHAINLINK_API_PASSWORD}" -lt 16 ] || [ "${#CHAINLINK_API_PASSWORD}" -gt 50 ]; then
  echo "CHAINLINK_API_PASSWORD 长度必须在 16 到 50 个字符之间" >&2
  exit 2
fi
case "$CHAINLINK_CHAIN_ID" in
  *[!0-9]*|'') echo "CHAINLINK_CHAIN_ID 必须是正整数" >&2; exit 2 ;;
esac

runtime_root=/runtime-state/chainlink
config_path="$runtime_root/config.toml"
secrets_path="$runtime_root/secrets.toml"
api_path="$runtime_root/api-credentials"
mkdir -p "$runtime_root"
umask 077

# Chainlink 2.61 使用 TOML 配置;HTTP-only runtime 通过受控轮询代替 WS 订阅,
# 不关闭 EVM log poller,也不伪造一个不存在的 WebSocket 地址。
cat > "$config_path" <<EOF
Log.Level = 'info'
[WebServer]
HTTPPort = 6688
SecureCookies = false
AuthenticationMethod = 'local'
[WebServer.TLS]
HTTPSPort = 0
[[EVM]]
ChainID = '$CHAINLINK_CHAIN_ID'
Enabled = true
LogBroadcasterEnabled = false
[EVM.NodePool]
NewHeadsPollInterval = '1s'
[[EVM.Nodes]]
Name = 'execution-chain'
HTTPURL = '$ETH_URL'
EOF

# Secret TOML 只存在于 runtime-state,并以最小权限保存。
cat > "$secrets_path" <<EOF
[Database]
URL = 'postgresql://$CHAIMIR_CHAINLINK_DATABASE_USER:$POSTGRES_PASSWORD@$CHAIMIR_CHAINLINK_DATABASE_ADDRESS/$CHAIMIR_CHAINLINK_DATABASE_NAME?sslmode=disable'
[Password]
Keystore = '$CHAINLINK_KEYSTORE_PASSWORD'
EOF
printf '%s\n%s\n' "$CHAINLINK_API_EMAIL" "$CHAINLINK_API_PASSWORD" > "$api_path"
chmod 600 "$config_path" "$secrets_path" "$api_path"

# Chainlink v2 rejects legacy ETH_URL environment configuration when TOML is used;
# the binding has already been materialized into config.toml above.
unset ETH_URL
exec /usr/local/bin/chainlink --config "$config_path" --secrets "$secrets_path" node start --api "$api_path"
