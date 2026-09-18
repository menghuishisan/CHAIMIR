#!/bin/sh
# 将官方 Chart 的镜像引用替换为供应链固定的不可变 digest。
set -eu
: "${EXTERNAL_SECRETS_RELEASE_IMAGE:?缺少 EXTERNAL_SECRETS_RELEASE_IMAGE}"

if ! printf '%s' "$EXTERNAL_SECRETS_RELEASE_IMAGE" | grep -Eq '^[A-Za-z0-9._/-]+@sha256:[0-9a-f]{64}$'; then
  echo "External Secrets 镜像必须是 repository@sha256:<64 hex>" >&2
  exit 2
fi

rendered="$(cat)"
if ! printf '%s' "$rendered" | grep -Eq 'ghcr\.io/external-secrets/external-secrets:[^[:space:]\"]+'; then
  echo "External Secrets Chart 未找到官方可变镜像引用,拒绝静默放行" >&2
  exit 1
fi
output="$(printf '%s' "$rendered" | sed -E "s#ghcr\.io/external-secrets/external-secrets:[^[:space:]\"]+#$EXTERNAL_SECRETS_RELEASE_IMAGE#g")"
if ! printf '%s' "$output" | grep -Fq "$EXTERNAL_SECRETS_RELEASE_IMAGE"; then
  echo "External Secrets Chart 镜像替换未生效" >&2
  exit 1
fi
if printf '%s' "$output" | grep -Eq 'ghcr\.io/external-secrets/external-secrets:[^[:space:]\"]+'; then
  echo "External Secrets Chart 仍包含可变镜像引用" >&2
  exit 1
fi
printf '%s' "$output"
