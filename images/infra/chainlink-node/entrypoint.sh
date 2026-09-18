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
: "${CHAINLINK_GAS_PRICE_MAX_WEI:?必须通过组合参数提供 EVM gas price 上限(wei)}"
: "${CHAINLINK_VRF_GAS_LIMIT:?必须通过组合参数提供 VRF fulfill gas 上限}"
: "${POSTGRES_PASSWORD:?必须通过数据库 Secret 提供 PostgreSQL 密码}"
: "${CHAINLINK_KEYSTORE_PASSWORD:?必须通过 Secret 提供 Chainlink keystore 密码}"
: "${CHAINLINK_API_EMAIL:?必须通过 Secret 提供 Chainlink API 邮箱}"
: "${CHAINLINK_API_PASSWORD:?必须通过 Secret 提供 Chainlink API 密码}"
: "${CHAINLINK_TLS_CERT_B64:?必须通过 Secret 提供 Chainlink TLS 证书}"
: "${CHAINLINK_TLS_KEY_B64:?必须通过 Secret 提供 Chainlink TLS 私钥}"

case "$ETH_URL" in
  http://*|https://*) ;;
  *) echo "ETH_URL 必须使用 HTTP(S) 协议" >&2; exit 2 ;;
esac
eth_target="${ETH_URL#*://}"
case "$eth_target" in
  *@*|*/*|*\?*|*\#*)
    echo "ETH_URL 不得包含凭据、路径、查询或片段" >&2
    exit 2
    ;;
esac
eth_host="${eth_target%:*}"
eth_port="${eth_target##*:}"
case "$eth_host" in
  ''|*[!A-Za-z0-9.-]*) echo "ETH_URL host 无效" >&2; exit 2 ;;
esac
if [ "$eth_host" = "$eth_target" ] || [ -z "$eth_port" ]; then
  echo "ETH_URL 必须包含显式 host 和端口" >&2
  exit 2
fi
case "$eth_port" in
  ''|*[!0-9]*) echo "ETH_URL 端口无效" >&2; exit 2 ;;
esac
if [ "$eth_port" -lt 1 ] || [ "$eth_port" -gt 65535 ]; then
  echo "ETH_URL 端口必须位于 1-65535" >&2
  exit 2
fi
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
case "$CHAINLINK_TLS_CERT_B64" in
  *[!A-Za-z0-9+/=]*|'') echo "CHAINLINK_TLS_CERT_B64 必须是 Base64 证书" >&2; exit 2 ;;
esac
case "$CHAINLINK_TLS_KEY_B64" in
  *[!A-Za-z0-9+/=]*|'') echo "CHAINLINK_TLS_KEY_B64 必须是 Base64 私钥" >&2; exit 2 ;;
esac
case "$CHAINLINK_CHAIN_ID" in
  *[!0-9]*|'') echo "CHAINLINK_CHAIN_ID 必须是正整数" >&2; exit 2 ;;
esac
if [ "$CHAINLINK_CHAIN_ID" -lt 1 ]; then
  echo "CHAINLINK_CHAIN_ID 必须是正整数" >&2
  exit 2
fi
case "$CHAINLINK_GAS_PRICE_MAX_WEI" in
  ''|*[!0-9]*) echo "CHAINLINK_GAS_PRICE_MAX_WEI 必须是正十进制整数" >&2; exit 2 ;;
esac
case "$CHAINLINK_GAS_PRICE_MAX_WEI" in
  *[1-9]*) ;;
  *) echo "CHAINLINK_GAS_PRICE_MAX_WEI 必须大于 0" >&2; exit 2 ;;
esac
case "$CHAINLINK_VRF_GAS_LIMIT" in
  ''|*[!0-9]*) echo "CHAINLINK_VRF_GAS_LIMIT 必须是正十进制整数" >&2; exit 2 ;;
esac
case "$CHAINLINK_VRF_GAS_LIMIT" in
  *[1-9]*) ;;
  *) echo "CHAINLINK_VRF_GAS_LIMIT 必须大于 0" >&2; exit 2 ;;
esac

runtime_root=/runtime-state/chainlink
config_path="$runtime_root/config.toml"
secrets_path="$runtime_root/secrets.toml"
api_path="$runtime_root/api-credentials"
tls_root="$runtime_root/tls"
tls_cert_path="$tls_root/server.crt"
tls_key_path="$tls_root/server.key"
mkdir -p "$runtime_root"
umask 077
mkdir -p "$tls_root"
printf '%s' "$CHAINLINK_TLS_CERT_B64" | base64 -d > "$tls_cert_path"
printf '%s' "$CHAINLINK_TLS_KEY_B64" | base64 -d > "$tls_key_path"
test -s "$tls_cert_path" || { echo "Chainlink TLS 证书为空" >&2; exit 2; }
test -s "$tls_key_path" || { echo "Chainlink TLS 私钥为空" >&2; exit 2; }

# Chainlink 2.61 的 EVM LogPoller 通过 HTTP RPC 轮询日志；
# 旧 LogBroadcaster 依赖 WS 订阅，当前 runtime binding 不提供 WS，因此必须关闭。
cat > "$config_path" <<EOF
Log.Level = 'info'
[Feature]
FeedsManager = true
LogPoller = true
UICSAKeys = true
[OCR2]
Enabled = true
[P2P]
[P2P.V2]
Enabled = true
ListenAddresses = ['0.0.0.0:6690']
AnnounceAddresses = ['0.0.0.0:6690']
[WebServer]
HTTPPort = 0
# Cookie 必须带 Secure 属性,避免认证凭据在非加密请求中被浏览器发送。
SecureCookies = true
[WebServer.TLS]
CertPath = '$tls_cert_path'
KeyPath = '$tls_key_path'
HTTPSPort = 6688
ForceRedirect = false
[[EVM]]
ChainID = '$CHAINLINK_CHAIN_ID'
Enabled = true
AutoCreateKey = true
MinContractPayment = 0
BlockBackfillDepth = 100
FinalityDepth = 1
FinalityTagEnabled = false
LogBroadcasterEnabled = false
[EVM.GasEstimator]
PriceMax = '$CHAINLINK_GAS_PRICE_MAX_WEI wei'
[EVM.GasEstimator.LimitJobType]
VRF = $CHAINLINK_VRF_GAS_LIMIT
[EVM.NodePool]
NewHeadsPollInterval = '1s'
[[EVM.Nodes]]
Name = 'execution-chain'
HTTPURL = '$ETH_URL'
EOF

# Secret TOML 只存在于 runtime-state,并以最小权限保存。
cat > "$secrets_path" <<EOF
[Database]
URL = 'postgresql://$CHAIMIR_CHAINLINK_DATABASE_USER:$POSTGRES_PASSWORD@$CHAIMIR_CHAINLINK_DATABASE_ADDRESS/$CHAIMIR_CHAINLINK_DATABASE_NAME?sslmode=require'
[Password]
Keystore = '$CHAINLINK_KEYSTORE_PASSWORD'
EOF
printf '%s\n%s\n' "$CHAINLINK_API_EMAIL" "$CHAINLINK_API_PASSWORD" > "$api_path"
chmod 600 "$config_path" "$secrets_path" "$api_path"

# Chainlink v2 使用 TOML 时会拒绝旧式 ETH_URL 环境配置;
# binding 已在上方写入 config.toml。
unset ETH_URL
exec /usr/local/bin/chainlink --config "$config_path" --secrets "$secrets_path" node start --api "$api_path"
