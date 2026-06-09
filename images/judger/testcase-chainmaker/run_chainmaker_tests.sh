#!/usr/bin/env sh
# 本脚本执行长安链合约判题脚本。
set -eu

: "${CHAIMIR_TEST_SCRIPT:=/judge/tests/run.sh}"
RESULT_STDOUT="/tmp/chaimir-judge-stdout.txt"
if [ ! -x "${CHAIMIR_TEST_SCRIPT}" ]; then
  echo "长安链判题脚本不存在或不可执行" >&2
  exit 64
fi

set +e
"${CHAIMIR_TEST_SCRIPT}" >"${RESULT_STDOUT}" 2>&1
status=$?
set -e
exec python /usr/local/bin/normalize-result --mode exit-code --exit-code "${status}" --source chainmaker --stdout "${RESULT_STDOUT}"
