#!/usr/bin/env sh
# 本脚本按项目类型执行 EVM 判题测试。
set -eu

: "${CHAIMIR_SUBMISSION_DIR:=/judge/submission}"
RESULT_STDOUT="/tmp/chaimir-judge-stdout.txt"
export HOME="${HOME:-/judge}"
export FOUNDRY_OFFLINE=true

# 判题进入编译/测试前必须确认实际连接到组合提供的 EVM runtime。
if [ -n "${CHAIMIR_EVM_RPC_URL:-}" ]; then
  rpc_body='{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
  rpc_response="$(wget -qO- --header='Content-Type: application/json' --post-data="${rpc_body}" "${CHAIMIR_EVM_RPC_URL}" 2>/dev/null || true)"
  case "${rpc_response}" in
    *'"result"'*) ;;
    *) echo "EVM runtime RPC 未返回有效 chainId" >&2; exit 66 ;;
  esac
fi

cd "${CHAIMIR_SUBMISSION_DIR}"

if [ -f foundry.toml ]; then
  if ! grep -Eq '^[[:space:]]*(solc|solc_version)[[:space:]]*=' foundry.toml; then
    echo "Foundry 项目必须显式声明 solc 或 solc_version" >&2
    exit 64
  fi
  set +e
  forge test >"${RESULT_STDOUT}" 2>&1
  status=$?
  set -e
  exec /usr/local/bin/normalize-result --mode exit-code --exit-code "${status}" --source foundry --stdout "${RESULT_STDOUT}"
fi

if [ -f hardhat.config.js ] || [ -f hardhat.config.ts ]; then
  if [ -L node_modules ]; then
    rm -f node_modules
  fi
  mkdir -p node_modules/.bin
  rm -rf node_modules/hardhat node_modules/.bin/hardhat
  ln -s /opt/chaimir/testcase-evm/node_modules/hardhat node_modules/hardhat
  ln -s /opt/chaimir/testcase-evm/node_modules/.bin/hardhat node_modules/.bin/hardhat
  hardhat_config="$(pwd)/$(if [ -f hardhat.config.js ]; then printf '%s' hardhat.config.js; else printf '%s' hardhat.config.ts; fi)"
  hardhat_wrapper="$(pwd)/.chaimir-hardhat-offline-wrapper.mjs"
  cp /opt/chaimir/testcase-evm/hardhat-offline-wrapper.mjs "${hardhat_wrapper}"
  set +e
  CHAIMIR_HARDHAT_ORIGINAL_CONFIG="${hardhat_config}" \
    ./node_modules/.bin/hardhat --config "${hardhat_wrapper}" test >"${RESULT_STDOUT}" 2>&1
  status=$?
  set -e
  rm -f "${hardhat_wrapper}"
  exec /usr/local/bin/normalize-result --mode exit-code --exit-code "${status}" --source hardhat --stdout "${RESULT_STDOUT}"
fi

echo "未发现 Foundry 或 Hardhat 测试项目" >&2
exit 64
