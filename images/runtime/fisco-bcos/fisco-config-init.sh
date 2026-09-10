#!/usr/bin/env sh
# 本脚本在受控 runtime-state 卷中离线生成 FISCO BCOS 单节点配置与证书材料。
set -eu

root=/runtime-state/fisco
config_path="${root}/nodes/127.0.0.1/node0/config.ini"
build_chain="/usr/local/lib/chaimir/fisco/build_chain.sh"
tassl="/usr/local/lib/chaimir/fisco/tassl-1.1.1b"

if [ "${CHAIMIR_SELFTEST:-0}" = "1" ]; then
    test -x "${build_chain}"
    test -x "${tassl}"
    exec /usr/local/bin/fisco-bcos --version
fi

if [ -f "${config_path}" ]; then
    test -r "${config_path}"
    exit 0
fi

mkdir -p "${root}/home/.fisco" "${root}/tmp"
install -m 0555 "${tassl}" "${root}/home/.fisco/tassl-1.1.1b"
export HOME="${root}/home"
export TMPDIR="${root}/tmp"

cd "${root}"
"${build_chain}" -p 30300,20200 -l 127.0.0.1:1 -o "${root}/nodes" -e /usr/local/bin/fisco-bcos -a 0x0000000000000000000000000000000000000001
test -r "${config_path}"
