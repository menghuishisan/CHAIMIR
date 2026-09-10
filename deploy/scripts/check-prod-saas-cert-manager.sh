#!/usr/bin/env bash
# 校验 prod-saas 应用前的 cert-manager、IngressClass 与生产 ClusterIssuer 前置。
set -euo pipefail

namespace="cert-manager"
issuer_name="chaimir-prod-issuer"
timeout_seconds=300
deadline=$((SECONDS + timeout_seconds))

fail() {
  echo "prod-saas cert-manager 前置校验失败: $1" >&2
  exit 1
}

if ! command -v kubectl >/dev/null 2>&1; then
  fail "缺少 kubectl"
fi

remaining_seconds() {
  local remaining=$((deadline - SECONDS))
  if (( remaining <= 0 )); then
    fail "超过 ${timeout_seconds} 秒总预算"
    return 1
  fi
  printf '%s' "$remaining"
}

request_timeout() {
  local remaining
  remaining="$(remaining_seconds)" || return 1
  if (( remaining < 10 )); then
    printf '%ss' "$remaining"
  else
    printf '10s'
  fi
}

run_kubectl() {
  local timeout
  timeout="$(request_timeout)" || return 1
  kubectl --request-timeout="$timeout" "$@"
}

run_kubectl get namespace "$namespace" >/dev/null 2>&1 || fail "缺少 cert-manager 命名空间或 Kubernetes API 不可用"
run_kubectl get ingressclass nginx >/dev/null 2>&1 || fail "缺少 IngressClass/nginx 或 Kubernetes API 不可用"
run_kubectl get crd clusterissuers.cert-manager.io certificates.cert-manager.io certificaterequests.cert-manager.io >/dev/null 2>&1 || fail "cert-manager CRD 未完整就绪"

for deployment in cert-manager cert-manager-cainjector cert-manager-webhook; do
  remaining="$(remaining_seconds)" || exit 1
  run_kubectl -n "$namespace" wait --for=condition=Available "deployment/$deployment" --timeout="${remaining}s" >/dev/null || fail "$deployment 未达到 Available"
  images="$(run_kubectl -n "$namespace" get deployment "$deployment" -o jsonpath='{range .spec.template.spec.containers[*]}{.image}{"\n"}{end}')" || fail "读取 $deployment 配置失败"
  [ -n "$images" ] || fail "$deployment 没有容器镜像"
  if printf '%s\n' "$images" | grep -Evq '^[^[:space:]@]+@sha256:[0-9a-f]{64}$'; then
    fail "$deployment 存在非 digest 镜像引用"
  fi
done

generation="$(run_kubectl get clusterissuer "$issuer_name" -o jsonpath='{.metadata.generation}' 2>/dev/null)" || fail "读取 ClusterIssuer/$issuer_name 失败"
observed_generation="$(run_kubectl get clusterissuer "$issuer_name" -o jsonpath='{.status.observedGeneration}' 2>/dev/null)" || fail "读取 ClusterIssuer/$issuer_name 状态失败"
[ -n "$generation" ] || fail "缺少 ClusterIssuer/$issuer_name"
[ "$generation" = "$observed_generation" ] || fail "ClusterIssuer/$issuer_name 尚未观测当前配置"

ready="$(run_kubectl get clusterissuer "$issuer_name" -o jsonpath='{range .status.conditions[?(@.type=="Ready")]}{.status}{"\n"}{end}')" || fail "读取 ClusterIssuer/$issuer_name Ready 状态失败"
printf '%s\n' "$ready" | grep -qx 'True' || fail "ClusterIssuer/$issuer_name 未 Ready"

echo "prod-saas cert-manager 前置已就绪: ClusterIssuer/$issuer_name Ready=True"
