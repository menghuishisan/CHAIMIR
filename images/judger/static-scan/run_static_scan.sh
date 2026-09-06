#!/usr/bin/env sh
# 本脚本执行 Solidity 静态扫描判题。
set -eu

: "${CHAIMIR_SUBMISSION_DIR:=/judge/submission}"
if [ -z "${CHAIMIR_SOLIDITY_VERSION:-}" ]; then
  echo "必须由组合配置声明 CHAIMIR_SOLIDITY_VERSION" >&2
  exit 64
fi

case "${CHAIMIR_SOLIDITY_VERSION}" in
  0.8.20|0.8.24|0.8.26) ;;
  *) echo "未声明的 Solidity 编译器版本: ${CHAIMIR_SOLIDITY_VERSION}" >&2; exit 64 ;;
esac
SOLC_PATH="/usr/local/bin/solc-${CHAIMIR_SOLIDITY_VERSION}"
test -x "${SOLC_PATH}"
case "$(${SOLC_PATH} --version)" in
  *"Version: ${CHAIMIR_SOLIDITY_VERSION}"*) ;;
  *) echo "Solidity 编译器版本校验失败: ${SOLC_PATH}" >&2; exit 65 ;;
esac

# judge 卷会覆盖镜像内同路径目录,运行期必须在声明的可写域中重建 Slither 缓存目录。
mkdir -p /judge/.solc-select/artifacts /tmp/slither

# 静态分析也必须绑定并验证实际 runtime,否则只能证明工具能运行。
if [ -n "${CHAIMIR_EVM_RPC_URL:-}" ]; then
  rpc_body='{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
  rpc_response="$(wget -qO- --header='Content-Type: application/json' --post-data="${rpc_body}" "${CHAIMIR_EVM_RPC_URL}" 2>/dev/null || true)"
  case "${rpc_response}" in
    *'"result"'*) ;;
    *) echo "EVM runtime RPC 未返回有效 chainId" >&2; exit 66 ;;
  esac
fi

cd "${CHAIMIR_SUBMISSION_DIR}"

if command -v slither >/dev/null 2>&1; then
  # 每次判题必须从空报告开始,避免复用 judge 卷时把上一次样本的报告误判为本次结果。
  rm -f /judge/result.json
  set +e
  slither . --solc "${SOLC_PATH}" --json /judge/result.json >/tmp/chaimir-static-scan.log 2>&1
  status=$?
  set -e
  if [ -s /judge/result.json ]; then
    exec /usr/local/bin/normalize-result --mode slither --report /judge/result.json
  fi
  if [ "${status}" -ne 0 ]; then
    exec /usr/local/bin/normalize-result --mode exit-code --exit-code "${status}" --source slither --stdout /tmp/chaimir-static-scan.log
  fi
  exec /usr/local/bin/normalize-result --mode slither --report /judge/result.json
fi

echo "静态扫描工具不可用" >&2
exit 65
