# Chaimir 本地开发与部署编排。
# 依据 docs/总-部署架构设计.md:local-dev 用 kind + Kustomize 一键拉起。
# 用法:make dev-up(建集群+中间件+应用) / make dev-down(销毁)。
#
# 前置:docker、kubectl(内置 kustomize)、kind、helm(仅 harbor-up 需要)。
# 注:本仓库用 kubectl 内置 kustomize(kubectl kustomize / kubectl apply -k)。

SHELL := /bin/bash
CLUSTER_NAME := chaimir
OVERLAY := overlays/local-dev
SECRET_FILE := $(OVERLAY)/secret.env
SECRET_EXAMPLE := config/secret.env.example
INGRESS_NGINX_VERSION := controller-v1.11.3
# kind 节点镜像版本(显式固定;镜像源缺该 tag 时可覆盖:make kind-up NODE_IMAGE=kindest/node:vX.Y.Z)。
NODE_IMAGE := kindest/node:v1.32.2

.PHONY: help
help: ## 显示可用目标
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ---- 一键起步 ----
.PHONY: dev-up
dev-up: kind-up ingress-up gen-secret middleware-wait apply-dev migrate ## 一键拉起本地环境(集群+中间件+应用+迁移)
	@echo "本地环境就绪。前端: http://chaimir.local (需将 chaimir.local 指向 127.0.0.1)"

.PHONY: dev-down
dev-down: ## 销毁本地 kind 集群
	kind delete cluster --name $(CLUSTER_NAME)

# ---- kind 集群 ----
.PHONY: kind-up
kind-up: ## 创建 kind 集群(若不存在)
	@kind get clusters | grep -qx $(CLUSTER_NAME) || \
		kind create cluster --name $(CLUSTER_NAME) --image $(NODE_IMAGE) --config $(OVERLAY)/kind-cluster.yaml

# ---- ingress-nginx ----
.PHONY: ingress-up
ingress-up: ## 安装 ingress-nginx 并打 namespace 标签(供 NetworkPolicy 识别)
	kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/$(INGRESS_NGINX_VERSION)/deploy/static/provider/kind/deploy.yaml
	kubectl wait --namespace ingress-nginx --for=condition=ready pod \
		--selector=app.kubernetes.io/component=controller --timeout=180s

# ---- 密钥(dev 随机生成)----
.PHONY: gen-secret
gen-secret: ## 首次生成 local-dev 随机密钥(secret.env,被 .gitignore 忽略)
	@if [ ! -f $(SECRET_FILE) ]; then \
		echo "生成 dev 密钥 -> $(SECRET_FILE)"; \
		{ \
		  echo "JWT_SIGNING_KEY=$$(openssl rand -hex 32)"; \
		  echo "APP_ENCRYPTION_KEY=$$(openssl rand -hex 16)"; \
		  echo "APP_HMAC_KEY=$$(openssl rand -hex 32)"; \
		  echo "PG_PASSWORD=$$(openssl rand -hex 16)"; \
		  echo "PG_PRIV_USER=postgres"; \
		  echo "PG_PRIV_PASSWORD=$$(openssl rand -hex 16)"; \
		  echo "REDIS_PASSWORD=$$(openssl rand -hex 16)"; \
		  echo "MINIO_ACCESS_KEY=chaimir-dev"; \
		  echo "MINIO_SECRET_KEY=$$(openssl rand -hex 20)"; \
		  echo "NATS_TOKEN=$$(openssl rand -hex 16)"; \
		  echo "BOOTSTRAP_ADMIN_PHONE=13800138000"; \
		  echo "BOOTSTRAP_ADMIN_NAME=首个管理员"; \
		  echo "BOOTSTRAP_ADMIN_PASSWORD=$$(openssl rand -hex 16)A1"; \
		  echo "BOOTSTRAP_PLATFORM_ADMIN_PASSWORD=$$(openssl rand -hex 16)A1"; \
		} > $(SECRET_FILE); \
	else echo "$(SECRET_FILE) 已存在,跳过"; fi

# ---- 应用部署 ----
.PHONY: apply-dev
apply-dev: gen-secret ## apply local-dev overlay(命名空间/中间件/应用)
	kubectl apply -k $(OVERLAY)

.PHONY: middleware-wait
middleware-wait: apply-dev ## 等待中间件就绪(迁移前置)
	kubectl -n chaimir-data rollout status statefulset/postgres --timeout=180s
	kubectl -n chaimir-data rollout status statefulset/redis --timeout=120s
	kubectl -n chaimir-data rollout status statefulset/nats --timeout=120s
	kubectl -n chaimir-data rollout status statefulset/minio --timeout=120s

# ---- 数据库迁移(中间件就绪后)----
.PHONY: migrate
migrate: ## 运行迁移 + RLS 初始化 + seed(删旧 Job 后重建)
	-kubectl -n chaimir-system delete job chaimir-migrate --ignore-not-found
	kubectl apply -k $(OVERLAY) --prune=false
	@echo "migrate Job 已提交(镜像就绪后执行;目录2/3 产出 chaimir/migrate 镜像后生效)"

# ---- 本地镜像加载(目录2/3 产出镜像后)----
.PHONY: load-images
load-images: ## 将本地构建的 chaimir 镜像 load 进 kind(目录2/3 后使用)
	kind load docker-image chaimir/backend:dev --name $(CLUSTER_NAME)
	kind load docker-image chaimir/frontend:dev --name $(CLUSTER_NAME)
	kind load docker-image chaimir/migrate:dev --name $(CLUSTER_NAME)

# ---- Harbor(可选,本地私有仓)----
.PHONY: harbor-up
harbor-up: ## 经官方 Helm chart 安装 Harbor 到 kind(中间件官方安装方式,非我方清单)
	helm repo add harbor https://helm.goharbor.io
	helm repo update
	helm upgrade --install harbor harbor/harbor --namespace harbor --create-namespace \
		--set expose.type=clusterIP --set persistence.enabled=true

# ---- 校验(无需集群)----
.PHONY: render
render: ## 渲染四个 overlay(检查 kustomize 构建)
	@for env in local-dev staging prod-saas prod-school; do \
		echo "=== overlays/$$env ==="; \
		kubectl kustomize overlays/$$env > /dev/null && echo "OK" || exit 1; \
	done

.PHONY: validate
validate: render ## 用 kubeconform 校验渲染产物(需安装 kubeconform)
	@for env in local-dev staging prod-saas prod-school; do \
		echo "=== validate overlays/$$env ==="; \
		kubectl kustomize overlays/$$env | kubeconform -strict -ignore-missing-schemas -summary; \
	done
