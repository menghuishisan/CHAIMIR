# Chaimir 本地开发与部署编排。
# 依据 docs/总-部署架构设计.md:local-dev 使用当前 kubeconfig 指向的 Kubernetes 集群。
# 用法:make dev-up(当前集群部署中间件+应用) / make deps-up(仅部署本地联调依赖)。
#
# 前置:docker、kubectl(内置 kustomize)、helm(仅 harbor-up 需要)。
# 注:本仓库用 kubectl 内置 kustomize(kubectl kustomize / kubectl apply -k)。

SHELL := /bin/bash
OVERLAY := overlays/local-dev
DEPS_OVERLAY := overlays/local-deps
SECRET_FILE := config/secret.env
SECRET_EXAMPLE := config/secret.env.example
INGRESS_NGINX_VERSION := controller-v1.11.3
METRICS_SERVER_VERSION := v0.8.1
# CSI snapshotter 只安装 VolumeSnapshot CRD 与 snapshot-controller;
# 真实快照能力仍取决于生产集群的 CSI 驱动和 VolumeSnapshotClass。
SNAPSHOTTER_VERSION := v8.6.0

.PHONY: help
help: ## 显示可用目标
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ---- 一键起步 ----
.PHONY: dev-up
dev-up: context-check ingress-up metrics-up-local gen-secret middleware-wait apply-dev migrate ## 使用当前 K8s context 拉起完整本地部署(中间件+应用+迁移)
	@echo "本地环境就绪。前端: http://chaimir.local (需将 chaimir.local 指向 127.0.0.1)"

.PHONY: dev-down
dev-down: ## 删除当前 K8s context 中的 Chaimir 本地资源(不删除集群)
	kubectl kustomize --load-restrictor=LoadRestrictionsNone $(OVERLAY) | kubectl delete -f - --ignore-not-found=true

.PHONY: deps-up
deps-up: context-check gen-secret ## 仅部署 PG/Redis/NATS/MinIO/ClamAV,供后端/前端在工作区直跑
	kubectl kustomize --load-restrictor=LoadRestrictionsNone $(DEPS_OVERLAY) | kubectl apply -f -
	kubectl -n chaimir-data rollout status statefulset/postgres --timeout=180s
	kubectl -n chaimir-data rollout status statefulset/redis --timeout=120s
	kubectl -n chaimir-data rollout status statefulset/nats --timeout=120s
	kubectl -n chaimir-data rollout status statefulset/minio --timeout=120s
	kubectl -n chaimir-data rollout status statefulset/clamav --timeout=300s

.PHONY: deps-down
deps-down: ## 删除仅依赖 overlay 资源(不删除集群)
	kubectl kustomize --load-restrictor=LoadRestrictionsNone $(DEPS_OVERLAY) | kubectl delete -f - --ignore-not-found=true

.PHONY: context-check
context-check: ## 显示并校验当前 Kubernetes context
	kubectl config current-context
	kubectl get nodes

# ---- ingress-nginx ----
.PHONY: ingress-up
ingress-up: ## 安装 ingress-nginx 并打 namespace 标签(供 NetworkPolicy 识别)
	kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/$(INGRESS_NGINX_VERSION)/deploy/static/provider/cloud/deploy.yaml
	kubectl wait --namespace ingress-nginx --for=condition=ready pod \
		--selector=app.kubernetes.io/component=controller --timeout=180s

# ---- metrics-server ----
.PHONY: metrics-up
metrics-up: ## 安装官方 metrics-server(生产/标准集群路径,不放宽 kubelet TLS)
	kubectl apply -k components/metrics-server
	kubectl -n kube-system rollout status deployment/metrics-server --timeout=180s

.PHONY: metrics-up-local
metrics-up-local: metrics-up ## Docker Desktop 等本地集群安装 metrics-server 并仅在本地放宽 kubelet TLS
	kubectl -n kube-system patch deployment metrics-server --type=strategic -p '{"spec":{"template":{"spec":{"containers":[{"name":"metrics-server","args":["--cert-dir=/tmp","--secure-port=10250","--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname","--kubelet-use-node-status-port","--metric-resolution=15s","--kubelet-insecure-tls"]}]}}}}'
	kubectl -n kube-system rollout status deployment/metrics-server --timeout=180s

# ---- CSI VolumeSnapshot ----
.PHONY: snapshot-up
snapshot-up: ## 安装 CSI VolumeSnapshot CRD 与 snapshot-controller(不安装具体存储驱动)
	kubectl apply -k components/csi-snapshotter
	kubectl -n kube-system rollout status deployment/snapshot-controller --timeout=180s

.PHONY: snapshot-check
snapshot-check: ## 检查集群是否具备 M2 快照前置能力(CRD/controller/class/storage class)
	kubectl get volumesnapshotclasses.snapshot.storage.k8s.io
	kubectl get volumesnapshots.snapshot.storage.k8s.io --all-namespaces
	kubectl get volumesnapshotcontents.snapshot.storage.k8s.io
	kubectl get storageclass

# ---- 密钥(dev 随机生成)----
.PHONY: gen-secret
gen-secret: ## 校验 config/secret.env 已由运维/开发者按模板填好
	@if [ ! -f $(SECRET_FILE) ]; then \
		echo "缺少 $(SECRET_FILE)。请复制 config/secret.env.example 为 config/secret.env,按模板命令生成并填入真实值。"; \
		exit 1; \
	else echo "$(SECRET_FILE) 已存在"; fi

# ---- 应用部署 ----
.PHONY: apply-dev
apply-dev: gen-secret ## apply local-dev overlay(命名空间/中间件/应用)
	kubectl kustomize --load-restrictor=LoadRestrictionsNone $(OVERLAY) | kubectl apply -f -

.PHONY: middleware-wait
middleware-wait: apply-dev ## 等待中间件就绪(迁移前置)
	kubectl -n chaimir-data rollout status statefulset/postgres --timeout=180s
	kubectl -n chaimir-data rollout status statefulset/redis --timeout=120s
	kubectl -n chaimir-data rollout status statefulset/nats --timeout=120s
	kubectl -n chaimir-data rollout status statefulset/minio --timeout=120s
	kubectl -n chaimir-data rollout status statefulset/clamav --timeout=300s

# ---- 数据库迁移(中间件就绪后)----
.PHONY: migrate
migrate: ## 运行迁移 + RLS 初始化 + seed(删旧 Job 后重建)
	-kubectl -n chaimir-system delete job chaimir-migrate --ignore-not-found
	kubectl kustomize --load-restrictor=LoadRestrictionsNone $(OVERLAY) | kubectl apply -f -
	@echo "migrate Job 已提交(镜像就绪后执行;目录2/3 产出 chaimir/migrate 镜像后生效)"

# ---- 本地镜像加载(目录2/3 产出镜像后)----
.PHONY: load-images
load-images: ## 将本地构建的 chaimir 镜像导入当前集群;Docker Desktop 通常无需额外导入
	@echo "Docker Desktop Kubernetes 直接复用本机 Docker 镜像;其他集群请推送到 overlay 配置的 registry。"

# ---- Harbor(可选,本地私有仓)----
.PHONY: harbor-up
harbor-up: ## 经官方 Helm chart 安装 Harbor 到当前集群(中间件官方安装方式,非我方清单)
	helm repo add harbor https://helm.goharbor.io
	helm repo update
	helm upgrade --install harbor harbor/harbor --namespace harbor --create-namespace \
		--set expose.type=clusterIP --set persistence.enabled=true

# ---- 校验(无需集群)----
.PHONY: render
render: ## 渲染 overlay(检查 kustomize 构建)
	@for env in local-deps local-dev staging prod-saas prod-school; do \
		echo "=== overlays/$$env ==="; \
		kubectl kustomize --load-restrictor=LoadRestrictionsNone overlays/$$env > /dev/null && echo "OK" || exit 1; \
	done

.PHONY: validate
validate: render ## 用 kubeconform 校验渲染产物(需安装 kubeconform)
	@for env in local-deps local-dev staging prod-saas prod-school; do \
		echo "=== validate overlays/$$env ==="; \
		kubectl kustomize --load-restrictor=LoadRestrictionsNone overlays/$$env | kubeconform -strict -ignore-missing-schemas -summary; \
	done
