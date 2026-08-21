# Chaimir 完整集群部署与验收编排。
# 依据 docs/总-部署架构设计.md:所有运行环境执行统一生产安全基线。
# 用法:make dev-up(首次 bootstrap);make dev-refresh(已 bootstrap 后刷新应用/迁移)。
#
# 前置:docker、kubectl(内置 kustomize)。Harbor/Trivy/Cosign/Helm 统一走容器化工具入口。
# 注:本仓库用 kubectl 内置 kustomize(kubectl kustomize / kubectl apply -k)。

SHELL := /bin/bash
OVERLAY := overlays/acceptance
SECRET_FILE := config/secret.env
SECRET_EXAMPLE := config/secret.env.example
SUPPLY_CHAIN_CONFIG := config/chaimir.env
SUPPLY_CHAIN_SECRET_FILE := config/supply-chain.secret.env
SUPPLY_CHAIN_SECRET_EXAMPLE := config/supply-chain.secret.env.example
SUPPLY_CHAIN_COMPOSE := image-supply-chain.compose.yaml
SUPPLY_CHAIN_PROJECT := chaimir-supply-chain
CILIUM_CLUSTER_SCRIPT := scripts/cilium-cluster.ps1
IMAGE_DIGEST_LOCK := ../images/image-digests.lock
INGRESS_NGINX_DIGEST := $(shell powershell -NoProfile -Command "((Select-String -Path '$(IMAGE_DIGEST_LOCK)' -Pattern '^ingress/ingress-nginx ').Line -split '\\s+')[1]")
GVISOR_CHECK_DIGEST := $(shell powershell -NoProfile -Command "((Select-String -Path '$(IMAGE_DIGEST_LOCK)' -Pattern '^base/chain-tools ').Line -split '\\s+')[1]")
GVISOR_CHECK_IMAGE := registry.chaimir.io/base/chain-tools@$(GVISOR_CHECK_DIGEST)
TLS_DIR := config/chaimir-tls
TLS_SECRET := chaimir-tls
METRICS_SERVER_VERSION := v0.8.1
POLICY_CONTROLLER_VERSION := 0.10.6
# 多镜像预拉取 DaemonSet 会在一次准入请求中校验完整镜像集合,保持 fail-closed 并给签名层拉取保留足够时间。
POLICY_CONTROLLER_WEBHOOK_TIMEOUT_SECONDS := 30
POLICY_CONTROLLER_WEBHOOK_REPLICAS := 2
POLICY_CONTROLLER_WEBHOOK_CPU_REQUEST := 200m
POLICY_CONTROLLER_WEBHOOK_CPU_LIMIT := 500m
# CSI snapshotter 只安装 VolumeSnapshot CRD 与 snapshot-controller;
# 真实快照能力仍取决于生产集群的 CSI 驱动和 VolumeSnapshotClass。
SNAPSHOTTER_VERSION := v8.6.0
# gVisor 节点运行时使用对象版本与 SHA512 双重固定,禁止跟随 latest 内容漂移。
GVISOR_RUNSC_URL := https://storage.googleapis.com/gvisor/releases/release/latest/x86_64/runsc?generation=1787184196516323
GVISOR_RUNSC_SHA512 := 84936438d583ec976800f464e75a83e1515f0890b451b9b4db219c4472b54ca9b106a6772ee683f1e64cce2128871d7637b14d800591f8451b8137f6c39fb2ef
GVISOR_SHIM_URL := https://storage.googleapis.com/gvisor/releases/release/latest/x86_64/containerd-shim-runsc-v1?generation=1787184195226077
GVISOR_SHIM_SHA512 := b60d1c418b841ab046951cc7a91f490a221198fbe81ec55dc364432578fddd44e97063793ce6651be397af2f64ec47170dff77a45db277819c3fb08fec9f3ced

.NOTPARALLEL: dev-up dev-refresh

.PHONY: help
help: ## 显示可用目标
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ---- 一键起步 ----
.PHONY: cilium-cluster-up
cilium-cluster-up: ## 创建或修复无默认 CNI 集群,安装并验收生产基线 Cilium
	powershell -NoProfile -ExecutionPolicy Bypass -File $(CILIUM_CLUSTER_SCRIPT) -Action Up

.PHONY: cilium-cluster-check
cilium-cluster-check: ## 校验 Cilium 独占状态、正式镜像和真实 NetworkPolicy 数据面
	powershell -NoProfile -ExecutionPolicy Bypass -File $(CILIUM_CLUSTER_SCRIPT) -Action Check

.PHONY: cilium-edge-up
cilium-edge-up: ## 将外部 canonical HTTPS 入口连接到 Cilium 集群
	powershell -NoProfile -ExecutionPolicy Bypass -File $(CILIUM_CLUSTER_SCRIPT) -Action Edge

.PHONY: cilium-cluster-down
cilium-cluster-down: ## 删除项目 Kind 集群,不删除外部 Harbor
	powershell -NoProfile -ExecutionPolicy Bypass -File $(CILIUM_CLUSTER_SCRIPT) -Action Down

.PHONY: dev-up
dev-up: cilium-cluster-up context-check gvisor-up gvisor-check ingress-up metrics-up policy-controller-up gen-secret tls apply-acceptance registry-pull-secret-sync middleware-wait acceptance-seed application-wait generated-config-gc cilium-edge-up ## 按统一生产安全基线拉起完整平台和验收夹具
	@echo "环境就绪。前端: https://www.chaimir.io (需将 www.chaimir.io 指向 127.0.0.1 并信任 config/chaimir-tls/tls.crt)"

.PHONY: dev-refresh
dev-refresh: cilium-cluster-up context-check gvisor-check gen-secret tls apply-acceptance registry-pull-secret-sync middleware-wait acceptance-seed application-wait generated-config-gc cilium-edge-up ## 重新验证集群基线后刷新应用、迁移和验收夹具
	@echo "环境已刷新。前端: https://www.chaimir.io"

.PHONY: dev-down
dev-down: context-check ## 删除 Cilium 集群中的 Chaimir 业务资源(不删除集群)
	kubectl kustomize --load-restrictor=LoadRestrictionsNone $(OVERLAY) | kubectl delete -f - --ignore-not-found=true

.PHONY: context-check
context-check: ## 显示并校验当前 Kubernetes context
	@test "$$(kubectl config current-context)" = "kind-chaimir-cilium" || { echo "当前 context 必须是 kind-chaimir-cilium"; exit 1; }
	kubectl get nodes

.PHONY: registry-runtime-dns
registry-runtime-dns: cilium-cluster-up context-check ## 确认 Cilium 节点已配置 canonical Harbor split-DNS 与 CA
	@echo "Cilium 节点 canonical Harbor 配置已由集群入口校验"

.PHONY: registry-pull-secret-bootstrap
registry-pull-secret-bootstrap: context-check ## 在签名准入前创建静态命名空间并同步工作负载 pull 凭据
	kubectl apply -f base/namespaces/namespaces.yaml
	@auth_dir="$$(sed -n 's/^SUPPLY_CHAIN_DOCKER_CONFIG_HOST_DIR=//p' "$(SUPPLY_CHAIN_CONFIG)")"; \
	auth_file="$$auth_dir/config.json"; \
	test -f "$$auth_file" || { echo "缺少 $$auth_file;请先完成 Harbor robot 凭据校验"; exit 1; }; \
	for namespace in chaimir-system chaimir-data chaimir-prepull; do \
		kubectl -n $$namespace create secret generic chaimir-harbor-pull \
			--from-file=.dockerconfigjson="$$auth_file" \
			--type=kubernetes.io/dockerconfigjson \
			--dry-run=client -o yaml | kubectl apply -f -; \
	done

.PHONY: registry-pull-secret-sync
registry-pull-secret-sync: registry-runtime-dns harbor-projects-ensure apply-acceptance ## 同步唯一 Harbor pull 凭据并刷新签名准入控制器
	@auth_dir="$$(sed -n 's/^SUPPLY_CHAIN_DOCKER_CONFIG_HOST_DIR=//p' "$(SUPPLY_CHAIN_CONFIG)")"; \
	auth_file="$$auth_dir/config.json"; \
	test -f "$$auth_file" || { echo "缺少 $$auth_file;请先完成 Harbor robot 凭据校验"; exit 1; }; \
	for namespace in cosign-system chaimir-system chaimir-data chaimir-prepull; do \
		kubectl -n $$namespace create secret generic chaimir-harbor-pull \
			--from-file=.dockerconfigjson="$$auth_file" \
			--type=kubernetes.io/dockerconfigjson \
			--dry-run=client -o yaml | kubectl apply -f -; \
	done
	kubectl -n cosign-system rollout restart deployment/policy-controller-webhook
	kubectl -n cosign-system rollout status deployment/policy-controller-webhook --timeout=180s

# ---- ingress-nginx ----
.PHONY: ingress-up
ingress-up: ## 安装 ingress-nginx 并打 namespace 标签(供 NetworkPolicy 识别)
	@test -n "$(INGRESS_NGINX_DIGEST)" || { echo "$(IMAGE_DIGEST_LOCK) 缺少 ingress/ingress-nginx digest"; exit 1; }
	@auth_dir="$$(sed -n 's/^SUPPLY_CHAIN_DOCKER_CONFIG_HOST_DIR=//p' "$(SUPPLY_CHAIN_CONFIG)")"; \
	auth_file="$$auth_dir/config.json"; \
	test -f "$$auth_file" || { echo "缺少 $$auth_file;请先完成 Harbor robot 凭据校验"; exit 1; }; \
	kubectl create namespace ingress-nginx --dry-run=client -o yaml | kubectl apply -f -; \
	kubectl label namespace ingress-nginx app.kubernetes.io/name=ingress-nginx \
		app.kubernetes.io/part-of=chaimir-trusted-control-plane \
		pod-security.kubernetes.io/enforce=restricted \
		pod-security.kubernetes.io/audit=restricted \
		pod-security.kubernetes.io/warn=restricted --overwrite; \
	kubectl -n ingress-nginx create secret generic chaimir-harbor-pull \
		--from-file=.dockerconfigjson="$$auth_file" \
		--type=kubernetes.io/dockerconfigjson \
		--dry-run=client -o yaml | kubectl apply -f -
	docker compose --project-name $(SUPPLY_CHAIN_PROJECT) --env-file $(SUPPLY_CHAIN_CONFIG) -f $(SUPPLY_CHAIN_COMPOSE) run --rm \
		-e INGRESS_NGINX_DIGEST=$(INGRESS_NGINX_DIGEST) --entrypoint sh helm -lc 'set -eu; \
		helm repo add ingress-nginx "$$SUPPLY_CHAIN_INGRESS_NGINX_CHART_REPOSITORY" --force-update; \
		helm repo update; \
		helm upgrade --install "$$SUPPLY_CHAIN_INGRESS_NGINX_RELEASE_NAME" ingress-nginx/ingress-nginx \
			--version "$$SUPPLY_CHAIN_INGRESS_NGINX_CHART_VERSION" \
			--namespace "$$SUPPLY_CHAIN_INGRESS_NGINX_NAMESPACE" --create-namespace \
			--server-side=false \
			-f "/workspace/$$SUPPLY_CHAIN_INGRESS_NGINX_VALUES_PATH" \
			--set-string controller.image.digest="$$INGRESS_NGINX_DIGEST" \
			--rollback-on-failure --wait --timeout 5m'
	kubectl -n ingress-nginx rollout status deployment/ingress-nginx-controller --timeout=180s
	kubectl -n ingress-nginx get deployment,service,ingressclass

# ---- metrics-server ----
.PHONY: metrics-apply
metrics-apply: ## 应用项目固定的 metrics-server 清单
	kubectl apply -k components/metrics-server

.PHONY: metrics-up
metrics-up: metrics-apply ## 安装官方 metrics-server(生产/标准集群路径,不放宽 kubelet TLS)
	# 首次按摘要拉取固定上游镜像可能接近三分钟,为镜像拉取和 readiness 保留完整窗口。
	kubectl -n kube-system rollout status deployment/metrics-server --timeout=300s
	kubectl top nodes

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
.PHONY: gvisor-up
gvisor-up: ## 在自管 Kind 节点安装固定 gVisor 二进制并注册 containerd 运行时
	@set -euo pipefail; \
	nodes=$$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}'); \
	needs_install=0; \
	for node in $$nodes; do \
		if ! docker exec $$node bash -c "printf '%s  /usr/local/bin/runsc\n%s  /usr/local/bin/containerd-shim-runsc-v1\n' '$(GVISOR_RUNSC_SHA512)' '$(GVISOR_SHIM_SHA512)' | sha512sum -c - >/dev/null && grep -q 'containerd.runtimes.runsc' /etc/containerd/config.toml"; then \
			needs_install=1; \
			break; \
		fi; \
	done; \
	if [ $$needs_install -eq 0 ]; then \
		echo "全部节点的 gVisor 二进制与 containerd 配置已匹配固定版本。"; \
	else \
		tmp=$$(mktemp -d); \
		trap 'rm -rf "$$tmp"' EXIT; \
		echo "下载并校验固定对象版本的 runsc 与 containerd shim..."; \
		curl -fsSL --retry 3 --retry-delay 2 --retry-all-errors --connect-timeout 20 --max-time 900 -o $$tmp/runsc '$(GVISOR_RUNSC_URL)'; \
		curl -fsSL --retry 3 --retry-delay 2 --retry-all-errors --connect-timeout 20 --max-time 900 -o $$tmp/containerd-shim-runsc-v1 '$(GVISOR_SHIM_URL)'; \
		printf '%s  runsc\n' '$(GVISOR_RUNSC_SHA512)' > $$tmp/runsc.sha512; \
		printf '%s  containerd-shim-runsc-v1\n' '$(GVISOR_SHIM_SHA512)' > $$tmp/containerd-shim-runsc-v1.sha512; \
		( cd $$tmp && sha512sum -c runsc.sha512 && sha512sum -c containerd-shim-runsc-v1.sha512 ) \
			|| { echo "runsc/shim 校验和不匹配,已中止安装。"; exit 1; }; \
		chmod +x $$tmp/runsc $$tmp/containerd-shim-runsc-v1; \
		for node in $$nodes; do \
			echo "==> 节点 $$node 安装 gVisor"; \
			docker cp $$tmp/runsc $$node:/usr/local/bin/runsc; \
			docker cp $$tmp/containerd-shim-runsc-v1 $$node:/usr/local/bin/containerd-shim-runsc-v1; \
			docker exec $$node bash -c 'set -e; chmod +x /usr/local/bin/runsc /usr/local/bin/containerd-shim-runsc-v1; grep -q "containerd.runtimes.runsc" /etc/containerd/config.toml || printf "\n        [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.runsc]\n          runtime_type = \"io.containerd.runsc.v1\"\n" >> /etc/containerd/config.toml; systemctl restart containerd'; \
		done; \
	fi; \
	kubectl wait --for=condition=Ready nodes --all --timeout=180s; \
	kubectl apply -k components/gvisor; \
	echo "gVisor 节点运行时安装完成,运行 make gvisor-check 验证。"

.PHONY: gvisor-check
gvisor-check: ## 校验 RuntimeClass 就绪并起一个 gVisor 测试 Pod 验证内核隔离
	@test -n "$(GVISOR_CHECK_DIGEST)" || { echo "$(IMAGE_DIGEST_LOCK) 缺少 base/chain-tools digest"; exit 1; }
	kubectl get runtimeclass gvisor
	kubectl -n kube-system delete pod gvisor-check --ignore-not-found --wait=true
	kubectl -n kube-system run gvisor-check --rm -i --restart=Never --image=$(GVISOR_CHECK_IMAGE) \
		--overrides='{"spec":{"runtimeClassName":"gvisor","imagePullSecrets":[{"name":"chaimir-harbor-pull"}]}}' -- dmesg | grep -i gvisor \
		&& echo "gVisor 隔离生效" || { echo "未检测到 gVisor,请确认节点已装 runsc"; exit 1; }

# ---- Sigstore policy-controller ----
.PHONY: policy-controller-up
policy-controller-up: supply-chain-tools-pull tls ## 安装 Sigstore policy-controller CRD/controller 并注入 Cosign 公钥
	@test -f config/cosign/cosign.pub || { echo "缺少 config/cosign/cosign.pub。请先用供应链流程生成 Cosign 公私钥,私钥不得提交。"; exit 1; }
	@test -f $(TLS_DIR)/tls.crt || { echo "缺少 $(TLS_DIR)/tls.crt。请先执行 make tls 生成 canonical TLS 证书。"; exit 1; }
	kubectl get namespace cosign-system >/dev/null 2>&1 || kubectl create namespace cosign-system
	kubectl -n cosign-system create secret generic cosign-public-key --from-file=cosign.pub=config/cosign/cosign.pub --dry-run=client -o yaml | kubectl apply -f -
	kubectl -n cosign-system create configmap chaimir-registry-ca --from-file=ca-bundle.crt=$(TLS_DIR)/tls.crt --dry-run=client -o yaml | kubectl apply -f -
	# webhook 控制器会维护 CA 与 namespaceSelector;Helm 4 必须使用客户端三方补丁,避免 SSA 字段所有权冲突。
	docker compose --project-name $(SUPPLY_CHAIN_PROJECT) --env-file $(SUPPLY_CHAIN_CONFIG) -f $(SUPPLY_CHAIN_COMPOSE) run --rm --entrypoint sh helm -lc 'set -u; \
		for attempt in 1 2 3; do \
			if helm repo add sigstore https://sigstore.github.io/helm-charts --force-update \
				&& helm repo update \
				&& helm upgrade --install policy-controller sigstore/policy-controller --version "$(POLICY_CONTROLLER_VERSION)" --namespace cosign-system --create-namespace --server-side=false --rollback-on-failure --timeout 10m --set webhook.registryCaBundle.name=chaimir-registry-ca --set webhook.registryCaBundle.key=ca-bundle.crt --set cosign.webhookTimeoutSeconds.mutating=$(POLICY_CONTROLLER_WEBHOOK_TIMEOUT_SECONDS) --set cosign.webhookTimeoutSeconds.validating=$(POLICY_CONTROLLER_WEBHOOK_TIMEOUT_SECONDS) --set webhook.webhookTimeoutSeconds.defaulting=$(POLICY_CONTROLLER_WEBHOOK_TIMEOUT_SECONDS) --set webhook.webhookTimeoutSeconds.validating=$(POLICY_CONTROLLER_WEBHOOK_TIMEOUT_SECONDS) --set webhook.replicaCount=$(POLICY_CONTROLLER_WEBHOOK_REPLICAS) --set-string webhook.resources.requests.cpu=$(POLICY_CONTROLLER_WEBHOOK_CPU_REQUEST) --set-string webhook.resources.limits.cpu=$(POLICY_CONTROLLER_WEBHOOK_CPU_LIMIT) --set webhook.failurePolicy=Fail --wait; then \
				exit 0; \
			fi; \
			if [ $$attempt -eq 3 ]; then echo "固定 policy-controller Chart 安装失败,已完成 3 次尝试。"; exit 1; fi; \
			echo "固定 policy-controller Chart 拉取或安装失败,5 秒后进行第 $$((attempt + 1)) 次尝试..."; \
			sleep 5; \
		done'
	kubectl wait --for=condition=Established crd/clusterimagepolicies.policy.sigstore.dev --timeout=120s
	kubectl apply -f base/admission/image-signature-policy.yaml
	kubectl wait --for=condition=Ready clusterimagepolicy/chaimir-require-signed-images --timeout=120s

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
.PHONY: apply-acceptance
apply-acceptance: gen-secret tls registry-pull-secret-bootstrap ## 应用 acceptance overlay(命名空间/中间件/应用)
	kubectl -n chaimir-system delete job chaimir-migrate --ignore-not-found
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
middleware-wait: apply-acceptance ## 等待中间件就绪(迁移前置)
	kubectl -n chaimir-data rollout status statefulset/postgres --timeout=180s
	kubectl -n chaimir-data rollout status statefulset/redis --timeout=120s
	kubectl -n chaimir-data rollout status statefulset/nats --timeout=120s
	kubectl -n chaimir-data rollout status statefulset/minio --timeout=120s
	kubectl -n chaimir-data rollout status statefulset/clamav --timeout=300s

# ---- 数据库迁移(中间件就绪后)----
.PHONY: migrate
migrate: ## 确认迁移 + RLS 初始化 +基础 seed Job 已提交
	@kubectl -n chaimir-system get job chaimir-migrate >/dev/null
	@echo "migrate Job 已提交(镜像就绪后执行;目录2/3 产出 service/migrate 镜像后生效)"

.PHONY: acceptance-seed
acceptance-seed: migrate ## 等待基础迁移完成并执行 canonical 验收夹具 seed
	kubectl -n chaimir-system wait --for=condition=complete job/chaimir-migrate --timeout=180s
	powershell -NoProfile -ExecutionPolicy Bypass -File ../scripts/db/run-acceptance-seed-job.ps1 -Namespace chaimir-system -MigrationJob chaimir-migrate -SeedJob chaimir-seed-acceptance

.PHONY: application-wait
application-wait: ## 等待应用控制器完成滚动,再允许回收旧哈希配置
	kubectl -n chaimir-system rollout status deployment/chaimir-backend --timeout=180s
	kubectl -n chaimir-system rollout status deployment/chaimir-frontend --timeout=180s
	kubectl -n chaimir-system rollout status deployment/chaimir-sandbox-rbac-provisioner --timeout=180s

.PHONY: generated-config-gc
generated-config-gc: ## 按当前工作负载引用清理未使用的 Kustomize 哈希 ConfigMap/Secret
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/generated-config-gc.ps1 -Context kind-chaimir-cilium

# ---- 镜像供应链工具与 Harbor(可选,本地/私有化私有仓)----
.PHONY: supply-chain-tools-pull
supply-chain-tools-pull: ## 拉取容器化 Trivy/Cosign/Helm 工具镜像
	@set -e; \
	for attempt in 1 2 3; do \
		docker compose --project-name $(SUPPLY_CHAIN_PROJECT) --env-file $(SUPPLY_CHAIN_CONFIG) -f $(SUPPLY_CHAIN_COMPOSE) pull --policy missing trivy cosign helm && exit 0; \
		if [ $$attempt -eq 3 ]; then echo "固定摘要工具镜像拉取失败,已完成 3 次尝试。"; exit 1; fi; \
		echo "工具镜像拉取失败,5 秒后进行第 $$((attempt + 1)) 次尝试..."; \
		sleep 5; \
	done

.PHONY: supply-chain-tools-check
supply-chain-tools-check: ## 通过 Docker 容器检查 Trivy/Cosign/Helm 可执行
	docker compose --project-name $(SUPPLY_CHAIN_PROJECT) --env-file $(SUPPLY_CHAIN_CONFIG) -f $(SUPPLY_CHAIN_COMPOSE) run --rm trivy --version
	docker compose --project-name $(SUPPLY_CHAIN_PROJECT) --env-file $(SUPPLY_CHAIN_CONFIG) -f $(SUPPLY_CHAIN_COMPOSE) run --rm cosign version
	docker compose --project-name $(SUPPLY_CHAIN_PROJECT) --env-file $(SUPPLY_CHAIN_CONFIG) -f $(SUPPLY_CHAIN_COMPOSE) run --rm helm version

.PHONY: harbor-up
harbor-up: supply-chain-secret-check ## 经容器化 Helm 安装 Harbor 到当前集群(官方 chart,非我方平台清单)
	@test -n "$$(sed -n 's/^SUPPLY_CHAIN_KUBECONFIG_HOST_PATH=//p' $(SUPPLY_CHAIN_CONFIG))" || { echo "请先在 $(SUPPLY_CHAIN_CONFIG) 填写 SUPPLY_CHAIN_KUBECONFIG_HOST_PATH"; exit 1; }
	@test -n "$$(sed -n 's/^SUPPLY_CHAIN_HARBOR_KUBECONFIG_HOST_PATH=//p' $(SUPPLY_CHAIN_CONFIG))" || { echo "请先在 $(SUPPLY_CHAIN_CONFIG) 填写 SUPPLY_CHAIN_HARBOR_KUBECONFIG_HOST_PATH"; exit 1; }
	@test -n "$$(sed -n 's/^HARBOR_ADMIN_PASSWORD=//p' $(SUPPLY_CHAIN_SECRET_FILE))" || { echo "请先在 $(SUPPLY_CHAIN_SECRET_FILE) 填写 HARBOR_ADMIN_PASSWORD"; exit 1; }
	SUPPLY_CHAIN_KUBECONFIG_HOST_PATH="$$(sed -n 's/^SUPPLY_CHAIN_HARBOR_KUBECONFIG_HOST_PATH=//p' $(SUPPLY_CHAIN_CONFIG))" \
	HARBOR_ADMIN_PASSWORD="$$(sed -n 's/^HARBOR_ADMIN_PASSWORD=//p' $(SUPPLY_CHAIN_SECRET_FILE))" \
		docker compose --project-name $(SUPPLY_CHAIN_PROJECT) --env-file $(SUPPLY_CHAIN_CONFIG) -f $(SUPPLY_CHAIN_COMPOSE) run --rm --entrypoint sh helm -lc 'set -eu; helm repo add harbor "$$SUPPLY_CHAIN_HARBOR_CHART_REPOSITORY"; helm repo update; helm upgrade --install "$$SUPPLY_CHAIN_HARBOR_RELEASE_NAME" harbor/harbor --namespace "$$SUPPLY_CHAIN_HARBOR_NAMESPACE" --create-namespace -f "$$SUPPLY_CHAIN_HARBOR_VALUES_PATH" --set-string externalURL="$$SUPPLY_CHAIN_HARBOR_EXTERNAL_URL" --set-string harborAdminPassword="$$HARBOR_ADMIN_PASSWORD"'

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
	@for env in acceptance staging prod-saas prod-school; do \
		echo "=== overlays/$$env ==="; \
		kubectl kustomize --load-restrictor=LoadRestrictionsNone overlays/$$env > /dev/null && echo "OK" || exit 1; \
	done

.PHONY: validate
validate: render ## 用 kubeconform 校验渲染产物(需安装 kubeconform)
	@for env in acceptance staging prod-saas prod-school; do \
		echo "=== validate overlays/$$env ==="; \
		kubectl kustomize --load-restrictor=LoadRestrictionsNone overlays/$$env | kubeconform -strict -ignore-missing-schemas -summary; \
	done
