#!/usr/bin/env sh
# 本脚本执行 Fabric 链码测试脚本。
set -eu

: "${CHAIMIR_TEST_SCRIPT:=/judge/tests/run.sh}"
RESULT_STDOUT="/tmp/chaimir-judge-stdout.txt"
if [ ! -x "${CHAIMIR_TEST_SCRIPT}" ]; then
  echo "Fabric 判题脚本不存在或不可执行" >&2
  exit 64
fi

set +e
"${CHAIMIR_TEST_SCRIPT}" >"${RESULT_STDOUT}" 2>&1
status=$?
set -e
exec python /usr/local/bin/normalize-result --mode exit-code --exit-code "${status}" --source fabric --stdout "${RESULT_STDOUT}"
