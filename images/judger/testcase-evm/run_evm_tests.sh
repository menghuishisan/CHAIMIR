#!/usr/bin/env sh
# 本脚本按项目类型执行 EVM 判题测试。
set -eu

: "${CHAIMIR_SUBMISSION_DIR:=/judge/submission}"
RESULT_STDOUT="/tmp/chaimir-judge-stdout.txt"
cd "${CHAIMIR_SUBMISSION_DIR}"

if [ -f foundry.toml ]; then
  set +e
  forge test >"${RESULT_STDOUT}" 2>&1
  status=$?
  set -e
  exec python3 /usr/local/bin/normalize-result --mode exit-code --exit-code "${status}" --source foundry --stdout "${RESULT_STDOUT}"
fi

if [ -f hardhat.config.js ] || [ -f hardhat.config.ts ]; then
  if [ -L node_modules ]; then
    rm -f node_modules
  fi
  mkdir -p node_modules/.bin
  rm -rf node_modules/hardhat node_modules/.bin/hardhat
  ln -s /opt/chaimir/testcase-evm/node_modules/hardhat node_modules/hardhat
  ln -s /opt/chaimir/testcase-evm/node_modules/.bin/hardhat node_modules/.bin/hardhat
  set +e
  ./node_modules/.bin/hardhat test >"${RESULT_STDOUT}" 2>&1
  status=$?
  set -e
  exec python3 /usr/local/bin/normalize-result --mode exit-code --exit-code "${status}" --source hardhat --stdout "${RESULT_STDOUT}"
fi

echo "未发现 Foundry 或 Hardhat 测试项目" >&2
exit 64
