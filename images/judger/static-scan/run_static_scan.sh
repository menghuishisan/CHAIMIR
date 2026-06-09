#!/usr/bin/env sh
# 本脚本执行 Solidity 静态扫描判题。
set -eu

: "${CHAIMIR_SUBMISSION_DIR:=/judge/submission}"
cd "${CHAIMIR_SUBMISSION_DIR}"

if command -v slither >/dev/null 2>&1; then
  set +e
  slither . --json /judge/result.json >/tmp/chaimir-static-scan.log 2>&1
  status=$?
  set -e
  if [ "${status}" -gt 1 ]; then
    exec python /usr/local/bin/normalize-result --mode exit-code --exit-code "${status}" --source slither --stdout /tmp/chaimir-static-scan.log
  fi
  exec python /usr/local/bin/normalize-result --mode slither --report /judge/result.json
fi

echo "静态扫描工具不可用" >&2
exit 65
