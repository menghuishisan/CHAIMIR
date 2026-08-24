#!/bin/sh
# 将官方 Chart 的可变镜像引用替换为供应链固定的不可变 digest。
set -eu
: "${EXTERNAL_SECRETS_RELEASE_IMAGE:?缺少 EXTERNAL_SECRETS_RELEASE_IMAGE}"
sed -E "s#ghcr.io/external-secrets/external-secrets:[^[:space:]\"]+#${EXTERNAL_SECRETS_RELEASE_IMAGE}#g"
