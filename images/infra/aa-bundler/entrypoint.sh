#!/usr/bin/env sh
# 本脚本按运行期注入的配置启动 ERC-4337 bundler,缺少配置时显式失败。
set -eu

: "${CHAIMIR_AA_BUNDLER_CONFIG:?必须通过运行时配置提供 bundler 配置文件路径}"

exec stackup-bundler --config "${CHAIMIR_AA_BUNDLER_CONFIG}"
