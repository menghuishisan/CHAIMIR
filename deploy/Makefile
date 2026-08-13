# Chaimir 本地开发与部署编排。
# 依据 docs/总-部署架构设计.md:local-dev 使用当前 kubeconfig 指向的 Kubernetes 集群。
# 用法:make dev-up(首次 bootstrap);make dev-refresh(已 bootstrap 后刷新应用/迁移)。
#
# 前置:docker、kubectl(内置 kustomize)。Harbor/Trivy/Cosign/Helm 统一走容器化工具入口。
# 注:本仓库用 kubectl 内置 kustomize(kubectl kustomize / kubectl apply -k)。

SHELL := /bin/bash
OVERLAY := overlays/local-dev
SECRET_FILE := config/secret.env
SECRET_EXAMPLE := config/secret.env.example
SUPPLY_CHAIN_CONFIG := config/chaimir.env
SUPPLY_CHAIN_SECRET_FILE := config/supply-chain.secret.env
SUPPLY_CHAIN_SECRET_EXAMPLE := config/supply-chain.secret.env.example
SUPPLY_CHAIN_COMPOSE := image-supply-chain.compose.yaml
SUPPLY_CHAIN_PROJECT := chaimir-supply-chain
TLS_DIR := config/chaimir-tls
TLS_SECRET := chaimir-tls
INGRESS_NGINX_VERSION := controller-v1.11.3
METRICS_SERVER_VERSION := v0.8.1
POLICY_CONTROLLER_VERSION := 0.10.6
# CSI snapshotter 只安装 VolumeSnapshot CRD 与 snapshot-controller;
# 真实快照能力仍取决于生产集群的 CSI 驱动和 VolumeSnapshotClass。
SNAPSHOTTER_VERSION := v8.6.0
# gVisor 发布源:本地节点安装 runsc + shim(生产用云厂商 gVisor 节点池,不走本目标)。
GVISOR_RELEASE_URL := https://storage.googleapis.com/gvisor/releases/release/latest/x86_64

.PHONY: help
help: ## 显示可用目标
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ---- 一键起步 ----
.PHONY: dev-up
dev-up: context-check gvisor-up-local ingress-up metrics-up-local policy-controller-up gen-secret tls middleware-wait apply-dev migrate ## 使用当前 K8s context 拉起完整部署(中间件+应用+迁移)
	@echo "环境就绪。前端: https://www.chaimir.io (需将 www.chaimir.io 指向 127.0.0.1 并信任 config/chaimir-tls/tls.crt)"

.PHONY: dev-refresh
dev-refresh: context-check gen-secret tls middleware-wait migrate ## 已完成 bootstrap 后刷新应用与迁移,不重复安装节点/集群组件
	@echo "环境已刷新。前端: https://www.chaimir.io"

.PHONY: dev-down
dev-down: ## 删除当前 K8s context 中的 Chaimir 本地资源(不删除集群)
	kubectl kustomize --load-restrictor=LoadRestrictionsNone $(OVERLAY) | kubectl delete -f - --ignore-not-found=true

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
	# LoadBalancer 必须在所有节点转发到真实 Ingress endpoint;本地 Docker Desktop/cloud-provider-kind
	# 无法保证入口 Pod 与暴露节点一致,使用 Cluster 避免正式 443 被无 endpoint 节点重置。
	kubectl -n ingress-nginx patch service ingress-nginx-controller --type=merge \
		-p '{"spec":{"externalTrafficPolicy":"Cluster"}}'

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

# ---- gVisor 隔离运行时(沙箱强制) ----
# 生产/私有化用云厂商 gVisor 节点池;下述目标仅供 Docker Desktop/kind 本地集群安装 runsc(不持久,集群重建需重跑)。
.PHONY: gvisor-up-local
gvisor-up-local: ## 本地在各节点安装 runsc 并注册 containerd 运行时,再装 RuntimeClass(Linux/WSL 下的 kind/Docker Desktop;需 gVisor 可运行的节点)
	@set -euo pipefail; \
	tmp=$$(mktemp -d); \
	echo "宿主下载 runsc 与 shim 及官方校验和(需可访问 gVisor 发布源)..."; \
	for f in runsc runsc.sha512 containerd-shim-runsc-v1 containerd-shim-runsc-v1.sha512; do \
		curl -fsSL -o $$tmp/$$f $(GVISOR_RELEASE_URL)/$$f; \
	done; \
	echo "校验下载完整性(损坏即中止,绝不安装坏二进制)..."; \
	( cd $$tmp && sha512sum -c runsc.sha512 && sha512sum -c containerd-shim-runsc-v1.sha512 ) \
		|| { echo "runsc/shim 校验和不匹配——下载被网络损坏或被篡改,已中止。请换可靠网络/镜像重试。"; rm -rf $$tmp; exit 1; }; \
	chmod +x $$tmp/runsc $$tmp/containerd-shim-runsc-v1; \
	for node in $$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}'); do \
		echo "==> 节点 $$node 安装 gVisor"; \
		docker cp $$tmp/runsc $$node:/usr/local/bin/runsc; \
		docker cp $$tmp/containerd-shim-runsc-v1 $$node:/usr/local/bin/containerd-shim-runsc-v1; \
		docker exec $$node bash -c 'set -e; chmod +x /usr/local/bin/runsc /usr/local/bin/containerd-shim-runsc-v1; grep -q "containerd.runtimes.runsc" /etc/containerd/config.toml || printf "\n        [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.runsc]\n          runtime_type = \"io.containerd.runsc.v1\"\n" >> /etc/containerd/config.toml; systemctl restart containerd'; \
	done; \
	rm -rf $$tmp; \
	kubectl apply -k components/gvisor; \
	echo "gVisor 本地安装完成,运行 make gvisor-check 验证。"

.PHONY: gvisor-check
gvisor-check: ## 校验 RuntimeClass 就绪并起一个 gVisor 测试 Pod 验证内核隔离
	kubectl get runtimeclass gvisor
	kubectl run gvisor-check --rm -i --restart=Never --image=busybox:1.36 \
		--overrides='{"spec":{"runtimeClassName":"gvisor"}}' -- dmesg | grep -i gvisor \
		&& echo "gVisor 隔离生效" || { echo "未检测到 gVisor,请确认节点已装 runsc"; exit 1; }

# ---- Sigstore policy-controller ----
.PHONY: policy-controller-up
policy-controller-up: supply-chain-tools-pull ## 安装 Sigstore policy-controller CRD/controller 并注入 Cosign 公钥
	@test -f config/cosign/cosign.pub || { echo "缺少 config/cosign/cosign.pub。请先用供应链流程生成 Cosign 公私钥,私钥不得提交。"; exit 1; }
	kubectl get namespace cosign-system >/dev/null 2>&1 || kubectl create namespace cosign-system
	kubectl -n cosign-system create secret generic cosign-public-key --from-file=cosign.pub=config/cosign/cosign.pub --dry-run=client -o yaml | kubectl apply -f -
	docker compose --project-name $(SUPPLY_CHAIN_PROJECT) --env-file $(SUPPLY_CHAIN_CONFIG) -f $(SUPPLY_CHAIN_COMPOSE) run --rm --entrypoint sh helm -lc 'set -eu; helm repo add sigstore https://sigstore.github.io/helm-charts; helm repo update; helm upgrade --install policy-controller sigstore/policy-controller --version "$(POLICY_CONTROLLER_VERSION)" --namespace cosign-system --create-namespace --wait'
	kubectl wait --for=condition=Established crd/clusterimagepolicies.policy.sigstore.dev --timeout=120s

# ---- 密钥(dev 随机生成)----
.PHONY: gen-secret
gen-secret: ## 校验 config/secret.env 已由运维/开发者按模板填好
	@if [ ! -f $(SECRET_FILE) ]; then \
		echo "缺少 $(SECRET_FILE)。请复制 $(SECRET_EXAMPLE) 为 $(SECRET_FILE),按模板命令生成并填入真实值。"; \
		exit 1; \
	else echo "$(SECRET_FILE) 已存在"; fi

.PHONY: supply-chain-secret-check
supply-chain-secret-check: ## 校验供应链专用密钥文件存在,不注入应用 Pod
	@if [ ! -f $(SUPPLY_CHAIN_SECRET_FILE) ]; then \
		echo "缺少 $(SUPPLY_CHAIN_SECRET_FILE)。请复制 $(SUPPLY_CHAIN_SECRET_EXAMPLE) 为 $(SUPPLY_CHAIN_SECRET_FILE),按模板填入真实值。"; \
		exit 1; \
	else echo "$(SUPPLY_CHAIN_SECRET_FILE) 已存在"; fi

# ---- 应用部署 ----
.PHONY: apply-dev
apply-dev: gen-secret tls ## 应用 local-dev overlay(命名空间/中间件/应用)
	kubectl kustomize --load-restrictor=LoadRestrictionsNone $(OVERLAY) | kubectl apply -f -

.PHONY: tls
tls: ## 使用持久化 HTTPS 证书并写入当前集群的 TLS Secret
	@set -euo pipefail; \
	mkdir -p "$(TLS_DIR)"; \
	if [ -s "$(TLS_DIR)/tls.key" ] && [ -s "$(TLS_DIR)/tls.crt" ] && ! command -v openssl >/dev/null 2>&1; then \
		echo "复用已有 TLS 证书(宿主机无 openssl,不执行生成)"; \
	elif [ -s "$(TLS_DIR)/tls.key" ] && [ -s "$(TLS_DIR)/tls.crt" ] && openssl x509 -in "$(TLS_DIR)/tls.crt" -checkend 86400 -noout >/dev/null 2>&1 && openssl x509 -in "$(TLS_DIR)/tls.crt" -noout -ext subjectAltName | grep -q 'DNS:www.chaimir.io' && openssl x509 -in "$(TLS_DIR)/tls.crt" -noout -ext subjectAltName | grep -q 'DNS:tools.chaimir.io' && openssl x509 -in "$(TLS_DIR)/tls.crt" -noout -ext subjectAltName | grep -q 'DNS:registry.chaimir.io'; then \
		echo "复用现有 canonical TLS 证书"; \
	else \
		command -v openssl >/dev/null 2>&1 || { echo "缺少 openssl,无法生成 HTTPS 证书"; exit 1; }; \
		openssl req -x509 -nodes -newkey rsa:2048 -days 825 \
			-keyout "$(TLS_DIR)/tls.key" -out "$(TLS_DIR)/tls.crt" \
			-subj "/CN=www.chaimir.io" \
			-addext "subjectAltName=DNS:www.chaimir.io,DNS:tools.chaimir.io,DNS:registry.chaimir.io" >/dev/null 2>&1; \
	fi; \
	kubectl create namespace chaimir-system --dry-run=client -o yaml | kubectl apply -f -; \
	kubectl -n chaimir-system create secret tls "$(TLS_SECRET)" \
		--cert="$(TLS_DIR)/tls.crt" --key="$(TLS_DIR)/tls.key" \
		--dry-run=client -o yaml | kubectl apply -f -

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

# ---- 镜像供应链工具与 Harbor(可选,本地/私有化私有仓)----
.PHONY: supply-chain-tools-pull
supply-chain-tools-pull: ## 拉取容器化 Trivy/Cosign/Helm 工具镜像
	docker compose --project-name $(SUPPLY_CHAIN_PROJECT) --env-file $(SUPPLY_CHAIN_CONFIG) -f $(SUPPLY_CHAIN_COMPOSE) pull trivy cosign helm

.PHONY: supply-chain-tools-check
supply-chain-tools-check: ## 通过 Docker 容器检查 Trivy/Cosign/Helm 可执行
	docker compose --project-name $(SUPPLY_CHAIN_PROJECT) --env-file $(SUPPLY_CHAIN_CONFIG) -f $(SUPPLY_CHAIN_COMPOSE) run --rm trivy --version
	docker compose --project-name $(SUPPLY_CHAIN_PROJECT) --env-file $(SUPPLY_CHAIN_CONFIG) -f $(SUPPLY_CHAIN_COMPOSE) run --rm cosign version
	docker compose --project-name $(SUPPLY_CHAIN_PROJECT) --env-file $(SUPPLY_CHAIN_CONFIG) -f $(SUPPLY_CHAIN_COMPOSE) run --rm helm version

.PHONY: harbor-up
harbor-up: supply-chain-secret-check ## 经容器化 Helm 安装 Harbor 到当前集群(官方 chart,非我方平台清单)
	@test -n "$$(sed -n 's/^SUPPLY_CHAIN_KUBECONFIG_HOST_PATH=//p' $(SUPPLY_CHAIN_CONFIG))" || { echo "请先在 $(SUPPLY_CHAIN_CONFIG) 填写 SUPPLY_CHAIN_KUBECONFIG_HOST_PATH"; exit 1; }
	@test -n "$$(sed -n 's/^HARBOR_ADMIN_PASSWORD=//p' $(SUPPLY_CHAIN_SECRET_FILE))" || { echo "请先在 $(SUPPLY_CHAIN_SECRET_FILE) 填写 HARBOR_ADMIN_PASSWORD"; exit 1; }
	HARBOR_ADMIN_PASSWORD="$$(sed -n 's/^HARBOR_ADMIN_PASSWORD=//p' $(SUPPLY_CHAIN_SECRET_FILE))" docker compose --project-name $(SUPPLY_CHAIN_PROJECT) --env-file $(SUPPLY_CHAIN_CONFIG) -f $(SUPPLY_CHAIN_COMPOSE) run --rm --entrypoint sh helm -lc 'set -eu; helm repo add harbor "$$SUPPLY_CHAIN_HARBOR_CHART_REPOSITORY"; helm repo update; helm upgrade --install "$$SUPPLY_CHAIN_HARBOR_RELEASE_NAME" harbor/harbor --namespace "$$SUPPLY_CHAIN_HARBOR_NAMESPACE" --create-namespace -f "$$SUPPLY_CHAIN_HARBOR_VALUES_PATH" --set-string externalURL="$$SUPPLY_CHAIN_HARBOR_EXTERNAL_URL" --set-string harborAdminPassword="$$HARBOR_ADMIN_PASSWORD"'

.PHONY: harbor-projects-ensure
harbor-projects-ensure: supply-chain-secret-check ## 按镜像分类创建 Harbor project 和供应链 robot
	@test -n "$$(sed -n 's/^SUPPLY_CHAIN_HARBOR_EXTERNAL_URL=//p' $(SUPPLY_CHAIN_CONFIG))" || { echo "请先在 $(SUPPLY_CHAIN_CONFIG) 填写 SUPPLY_CHAIN_HARBOR_EXTERNAL_URL"; exit 1; }
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/harbor-projects-ensure.ps1 -ConfigPath "$(SUPPLY_CHAIN_CONFIG)" -SecretPath "$(SUPPLY_CHAIN_SECRET_FILE)"

.PHONY: image-attestations-generate
image-attestations-generate: supply-chain-secret-check ## 扫描/签名/验签镜像并回写 deploy 与 backend env 的沙箱准入证明
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/image-attestations-generate.ps1 -RepoRoot "$$(cd .. && pwd)" -ConfigPath "$(SUPPLY_CHAIN_CONFIG)" -SecretPath "$(SUPPLY_CHAIN_SECRET_FILE)" -GenerateKeyIfMissing

# ---- 校验(无需集群)----
.PHONY: render
render: ## 渲染 overlay(检查 kustomize 构建)
	@for env in local-dev staging prod-saas prod-school; do \
		echo "=== overlays/$$env ==="; \
		kubectl kustomize --load-restrictor=LoadRestrictionsNone overlays/$$env > /dev/null && echo "OK" || exit 1; \
	done

.PHONY: validate
validate: render ## 用 kubeconform 校验渲染产物(需安装 kubeconform)
	@for env in local-dev staging prod-saas prod-school; do \
		echo "=== validate overlays/$$env ==="; \
		kubectl kustomize --load-restrictor=LoadRestrictionsNone overlays/$$env | kubeconform -strict -ignore-missing-schemas -summary; \
	done
