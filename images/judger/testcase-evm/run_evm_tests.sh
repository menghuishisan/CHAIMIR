#!/usr/bin/env sh
# 本脚本按项目类型执行 EVM 判题测试。
set -eu

: "${CHAIMIR_SUBMISSION_DIR:?CHAIMIR_SUBMISSION_DIR is required}"
RESULT_STDOUT="/tmp/chaimir-judge-stdout.txt"
: "${HOME:?HOME is required}"
export FOUNDRY_OFFLINE=true

# 工作卷可能禁止执行;缓存只链接只读镜像内的固定编译器,不写私有输入。
for version in 0.8.20 0.8.24 0.8.26; do
  compiler="/usr/local/bin/solc-${version}"
  target="${HOME}/.svm/${version}/solc-${version}"
  test -x "${compiler}"
  mkdir -p "${HOME}/.svm/${version}"
  rm -f "${target}"
  ln -s "${compiler}" "${target}"
done

# 私有归档只读挂载;判题编译产物必须落在 judger 自有工作目录,不能回写隐藏输入。
test -d "${CHAIMIR_SUBMISSION_DIR}"
rm -rf /judge/input
mkdir -p /judge/input
cp -R "${CHAIMIR_SUBMISSION_DIR}/." /judge/input/
CHAIMIR_WORK_DIR=/judge/input

# 判题进入编译/测试前必须确认实际连接到组合提供的 EVM runtime。
if [ -n "${CHAIMIR_EVM_RPC_URL:-}" ]; then
  rpc_body='{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
  rpc_response="$(node /opt/chaimir/testcase-evm/rpc-probe.mjs "${CHAIMIR_EVM_RPC_URL}")"
  case "${rpc_response}" in
    *'"result"'*) ;;
    *) echo "EVM runtime RPC 未返回有效 chainId" >&2; exit 66 ;;
  esac
fi

cd "${CHAIMIR_WORK_DIR}"

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
  ln -s /opt/chaimir/evm-hardhat/node_modules/hardhat node_modules/hardhat
  ln -s /opt/chaimir/evm-hardhat/node_modules/.bin/hardhat node_modules/.bin/hardhat
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
