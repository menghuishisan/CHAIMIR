# Chaimir 部署(deploy/)

Chaimir 区块链「教学·实验·竞赛」平台的部署清单。双形态:SaaS 公网多租户 + 学校私有化。
编排用 **Kustomize**(base + overlays),依据 `docs/总-部署架构设计.md` 与 `docs/总-镜像与容器设计.md`。

> 注:应用镜像(chaimir/backend|frontend|migrate|cron)为逻辑占位,由目录2(backend)/目录3(images)
> 构建产出后经 overlay 的 `images:` 覆盖生效。本目录提供的是完整运行底座。

## 目录结构

```
deploy/
├── Makefile                 本地一键拉起/销毁/校验
├── image-supply-chain.compose.yaml
│                            容器化 Trivy/Cosign/Helm 工具入口
├── config/                  「唯一」环境变量源(单一来源)
│   ├── chaimir.env          全平台非密配置(每项带注释)
│   ├── secret.env.example   密钥模板(复制为 secret.env,被 .gitignore 忽略)
│   ├── supply-chain.secret.env.example
│   │                       供应链工具专用密钥模板(不注入应用 Pod)
│   ├── external-secret/      SaaS ExternalSecret 统一模板(overlay 只覆盖环境差异)
│   └── kustomization.yaml   config component:生成两命名空间 ConfigMap
├── base/                    通用资源(命名空间/RBAC/应用/入口/策略/定时任务)
│   ├── namespaces/          chaimir-system / chaimir-data / monitoring + PodSecurity
│   ├── rbac/                后端 SA + 最小 ClusterRole(动态运行命名空间管理)
│   ├── backend/             后端单体 Deployment + Service
│   ├── frontend/            前端 Nginx Deployment + Service
│   ├── migrate/             迁移 + RLS 初始化 + seed Job
│   ├── ingress/             Ingress(/ 前端、/api 后端与统一实时通道)
│   ├── networkpolicy/       静态命名空间 deny-all + 精确放行;动态沙箱 deny-all 模板
│   └── cronjobs/            每日备份;业务生命周期清理由各模块后台任务负责
├── components/middleware/   PG16/Redis7/NATS2.10/MinIO 单实例(overlay 按需 include)
├── overlays/
│   ├── local-dev/           现有 K8s 集群 + 单实例中间件 + Harbor digest 镜像 + dev 密钥
│   ├── staging/             SaaS 形态,中间件外接,main 自动部署
│   ├── prod-saas/           多副本 + HPA,中间件外接,真实证书/KMS
│   └── prod-school/         k3s,单实例中间件,关平台层(私有化单校)
├── charts/                  第三方 Helm values(当前 Harbor)
└── ci/                      可复用 composite action + Trivy 配置
```

## Kustomize 与 Helm 边界

Chaimir 平台自身的 Kubernetes 资源统一维护在 `base/` + `overlays/` + `components/`。
不维护平台 Helm chart,避免同一批 Deployment/Service/RBAC/NetworkPolicy 同时存在 Kustomize
与 Helm 两套模板。

`deploy/charts/` 不是必需目录。仅当引入第三方 Kubernetes 组件且需要固定官方 Helm chart
的 values 或安装说明时创建,例如 ingress-nginx、Harbor、External Secrets、Sigstore
policy-controller、Prometheus Adapter。该目录不得放 Chaimir 平台自身模板。

数据库初始化脚本不放在 `deploy/charts/`:建库、应用角色、RLS 初始化入口、seed 编排归
`scripts/db/` 与 `backend/db/migrations/`;本目录只负责通过 `chaimir-migrate` Job 调度执行。

## 环境变量(单一来源)

全平台非密配置只在 **`config/chaimir.env`** 一处定义(每个变量带注释)。它经 `config/` 这个
Kustomize component 同时生成两个命名空间的 ConfigMap:

- `chaimir-config`(chaimir-system)— 后端/migrate `envFrom` 注入。
- `chaimir-data-config`(chaimir-data)— 中间件 Pod 引用(同源同值,无重复定义)。

各 overlay 仅用 `behavior: merge` 覆盖差异键(如 `DEPLOY_MODE`、`PLATFORM_LAYER_ENABLED`、
中间件外接端点),不重写整份配置。

**密钥**走 `secret.env`(从 `config/secret.env.example` 复制,被 `.gitignore` 忽略):

- `local-dev` / `prod-school`:overlay 用 `secretGenerator` 从本地 `secret.env` 生成
  `chaimir-secret`(系统层)+ `chaimir-data-secret`(数据层)。
- `staging` / `prod-saas`:不提交任何密钥 —— overlay 引用 `config/external-secret/`
  统一模板,由 External Secrets Operator 从 KMS/Secret Manager 同步出 `chaimir-secret`。
  集群侧 `ClusterSecretStore` 由运维以 Secret/KMS 管理方式创建,仓库不硬编码第三方端点或凭据。

## 本地环境

前置:`docker`、`kubectl`(内置 kustomize);可选 `kubeconform`(校验)。Trivy、Cosign、Helm 不要求安装到宿主机,统一通过 `image-supply-chain.compose.yaml` 的容器化工具入口运行。本仓库不要求额外安装 kind:只要当前 kubeconfig 指向可用 Kubernetes 集群即可,例如 Docker Desktop Kubernetes、k3s 或生产/预发布集群。

当前联调阶段可以只把 PostgreSQL、Redis、NATS、MinIO、ClamAV 等依赖跑在 Kubernetes,后端和前端进程直接在工作区运行。生产/私有化部署仍使用同一套 K8s 清单把平台服务镜像部署进集群,不会为本地直跑在后端代码里写分支。

检查当前集群:

```bash
kubectl config current-context
kubectl get nodes
```

完整 K8s 本地部署仍可渲染/应用 `overlays/local-dev`,它会部署后端、前端、migrate 和单实例中间件,适合验证容器化部署形态。若只需要本地进程联调,不要应用完整 `local-dev` overlay,避免业务 Pod 与本地进程并行。

本项目把本地/私有化密钥统一放在 `deploy/config/secret.env`,overlay 需要读取目录外的同一份文件。不要直接执行 `kubectl apply -k overlays/...`;使用 Makefile 目标,或显式加 Kustomize load restrictor:

```bash
kubectl kustomize --load-restrictor=LoadRestrictionsNone overlays/local-deps | kubectl apply -f -
```

只部署依赖:

```bash
cd deploy
make deps-up
```

业务进程在宿主机直跑时,通过 port-forward 把依赖暴露到 `127.0.0.1`,并在 `backend/.env` 覆盖对应地址:

```bash
kubectl -n chaimir-data port-forward svc/postgres 15432:5432
kubectl -n chaimir-data port-forward svc/redis 16379:6379
kubectl -n chaimir-data port-forward svc/nats 14222:4222
kubectl -n chaimir-data port-forward svc/minio 19000:9000
kubectl -n chaimir-data port-forward svc/clamav 13310:3310
```

对应本地覆盖示例:

```env
PG_HOST=127.0.0.1
PG_PORT=15432
REDIS_HOST=127.0.0.1
REDIS_PORT=16379
NATS_URL=nats://127.0.0.1:14222
MINIO_ENDPOINT=127.0.0.1:19000
UPLOAD_VIRUS_SCAN_ADDRESS=127.0.0.1:13310
KUBECONFIG_PATH=C:\Users\<你的用户名>\.kube\config
```

`make dev-up` 仅是便捷包装,使用当前 Kubernetes context,不负责创建任何集群。生产/标准集群使用 `make metrics-up` 保持 kubelet TLS 校验;
Docker Desktop 等本地集群如 kubelet 证书不完整,使用 `make metrics-up-local`,仅本地追加
`--kubelet-insecure-tls`,不得用于生产。

M2 快照能力分为两层:通用 `VolumeSnapshot` CRD/snapshot-controller 与具体 CSI 存储驱动。
本仓库提供 `make snapshot-up` 安装官方 CSI snapshotter 集群组件,并提供 `make snapshot-check`
检查 CRD、`VolumeSnapshotClass` 与 `StorageClass` 是否存在。真正能创建快照还必须由集群安装
支持快照的 CSI 驱动,并在 `SANDBOX_STORAGE_CLASS_NAME`、`SANDBOX_VOLUME_SNAPSHOT_CLASS_NAME`
中填写真实类名。`rancher.io/local-path`、普通 Docker volume 或演示用 hostpath CSI 不作为生产
快照方案。

将 `chaimir.local` 指向 `127.0.0.1`(hosts)后,前端经 `http://chaimir.local` 访问。

> 应用镜像就绪前(目录2/3 未产出),backend/frontend/migrate Pod 会处于 ImagePull 待命 —— 属预期。
> 镜像必须由统一供应链直接推送 Harbor、按 digest 回拉并通过门禁,再由 `image-metadata-promotion` 晋升到权威锁和 local-dev overlay;不得导入本地 `:dev` tag 绕过该流程。

## Harbor 与供应链工具

Harbor 使用官方 Helm chart,但 Helm 本身不安装到宿主机。先在 `config/chaimir.env` 填写 `SUPPLY_CHAIN_KUBECONFIG_HOST_PATH`,复制 `config/supply-chain.secret.env.example` 为 `config/supply-chain.secret.env` 并填写 Harbor 管理员密码、robot 凭据和 Cosign 私钥口令,再执行:

```bash
cd deploy
make supply-chain-tools-pull
make supply-chain-tools-check
make harbor-up
make harbor-forward
make harbor-projects-ensure
```

Docker Desktop 本地没有固定入口 IP 时,不要把 ingress 的 `172.18.x.x` 写入配置或 hosts;该地址会随
Docker/K8s 重启变化。统一使用 `SUPPLY_CHAIN_HARBOR_EXTERNAL_URL=http://harbor.chaimir:30080`
和 `make harbor-forward` 暴露 Harbor Ingress Controller。`harbor-forward` 需要在单独终端保持运行;
它按 `SUPPLY_CHAIN_HARBOR_FORWARD_ADDRESS` 监听 `SUPPLY_CHAIN_HARBOR_FORWARD_PORT`,并且必须转发
Ingress Controller,不能直连 `harbor-core`,否则 `/v2/` 上传路径与生产不一致。生产/预发布必须通过交付配置把
`SUPPLY_CHAIN_HARBOR_EXTERNAL_URL` 和 `SUPPLY_CHAIN_REGISTRY` 覆盖为 HTTPS 真实域名。

`harbor-projects-ensure` 创建镜像规范里的 Harbor project:`service/runtime/infra/tool/judger/sim/sidecar/init/base/middleware/observability/ingress`,并创建供应链 robot 账号。平台镜像不得推到默认 `library`,否则 digest 锁和准入策略无法按分类审计。

该目标通过 `deploy/scripts/harbor-projects-ensure.ps1` 运行,避免把 PowerShell 逻辑塞进 Makefile 字符串里。首次创建 robot 时,Harbor 只返回一次 token;脚本会把 `HARBOR_ROBOT_USERNAME` 和 `HARBOR_ROBOT_PASSWORD` 回写到被忽略的 `deploy/config/supply-chain.secret.env`,不得提交。

镜像完成构建、推送和 digest 锁生成后,使用统一供应链入口生成沙箱准入证明:

```bash
cd deploy
make image-attestations-generate
```

该目标会通过容器化 Trivy/Cosign 扫描、生成 CycloneDX SBOM、签名镜像、签署 SBOM 证明并分别验证 digest 锁中的镜像,再把
`SANDBOX_IMAGE_ATTESTATIONS_JSON` 同步回写到 `deploy/config/chaimir.env` 与
`backend/.env`。Cosign 私钥目录固定由 `SUPPLY_CHAIN_COSIGN_KEY_HOST_DIR` 指向,默认是
`deploy/config/cosign/`;Docker registry 认证目录由 `SUPPLY_CHAIN_DOCKER_CONFIG_HOST_DIR`
指向,默认是 `deploy/config/docker-auth/`。这两个目录只保存本地/私有化凭据,已被 Git 忽略,
不得提交到仓库。`SUPPLY_CHAIN_REGISTRY_HOST_ALIAS` 只用于容器化供应链工具访问宿主机暴露的
Harbor 入口,必须与 `SUPPLY_CHAIN_REGISTRY` 的主机名一致。生产/预发布环境应由 CI/Harbor/KMS
提供对应密钥和认证配置。

只有进入 Chaimir 镜像供应链或实验真实测试流程时才启动 `make harbor-forward`;日常只打开 Docker
Desktop 做其他项目时不应自动占用该端口。生产不得依赖 port-forward。

`SUPPLY_CHAIN_TRIVY_IMAGE`、`SUPPLY_CHAIN_COSIGN_IMAGE`、`SUPPLY_CHAIN_HELM_IMAGE` 使用 digest 固定。当前 Trivy 固定为 0.72.0,Cosign 固定为 3.1.1;GitHub 工作流也显式安装同一 Cosign 版本。本地、私有化与 CI 使用项目私钥且不上传公共透明日志。需要升级工具时,先拉取目标稳定版本并确认 digest与命令参数,再同步更新 `config/chaimir.env` 和工作流版本,不得提交可变 `latest`。

## 校验(无需集群)

```bash
make render      # 渲染四个 overlay,检查 kustomize 构建
make validate    # 渲染 + kubeconform 校验(需安装 kubeconform)
```

或单独渲染:`kubectl kustomize --load-restrictor=LoadRestrictionsNone overlays/local-dev`。

## CI/CD

GitHub Actions(`.github/workflows/`)+ 可复用配置(`deploy/ci/`):

- `backend.yml` / `frontend.yml` / `images.yml`:路径触发 → lint+测试 → 构建 →
  Trivy 扫描(高危阻断)+ CycloneDX SBOM → 推 Harbor并解析 digest → Cosign 镜像签名、SBOM 证明和双重验证。三者共用 `ci/build-scan-sign-push` composite action;backend 独占 backend/migrate/cron,frontend 独占 frontend,通用 images 不重复构建四个服务镜像。缺少 registry/robot/Cosign Secret 时在构建前显式失败。
- `image-metadata-promotion.yml`:串行消费三条产物流水线的 digest 片段,同步权威 lock、local-dev digest 和受控配置引用,再由机器人 PR 自动合并;业务流水线不直接写 `main`。
- `deploy.yml`:`images/image-digests.lock` 合入 `main` 后,从同一权威锁按 digest 自动部署 staging;打 `v*` tag 后从该发布提交的锁按 digest 渲染,经 GitHub Environment 人工审批部署 prod-saas。

README、docs 或普通应用提交不直接创建 staging Deployment。只有已完成扫描、推送、签名、验签并由机器人 PR 晋升的权威锁变更才触发部署;backend、frontend、migrate、cron 必须同时存在有效 digest,不按 SHA tag、版本 tag 或 `latest` 降级。

所需 GitHub Secrets:`HARBOR_REGISTRY`、`HARBOR_USERNAME`、`HARBOR_PASSWORD`、
`COSIGN_KEY`、`COSIGN_PASSWORD`、`IMAGE_METADATA_BOT_TOKEN`、`KUBECONFIG_STAGING`、`KUBECONFIG_PROD_SAAS`。仓库还必须启用 Auto-merge;`IMAGE_METADATA_BOT_TOKEN` 使用能触发 PR 检查的 GitHub App 或细粒度 PAT,不得用默认 `GITHUB_TOKEN` 替代。

默认 `ubuntu-latest` runner 必须能通过 HTTPS 访问 `HARBOR_REGISTRY` 与目标 staging/prod Kubernetes API。本机 `harbor.chaimir:30080` 依赖端口转发,只属于本地供应链,不能填写为 GitHub 托管 runner 的 registry。若交付环境只提供私网 Harbor/K8s,必须先在同一受控网络注册专用自托管 runner,再将镜像构建和部署 job 的 `runs-on` 收敛到该 runner 标签;不得通过暴露本机临时端口或提交本地凭据绕过网络边界。

## 安全基线

- NetworkPolicy 默认 deny-all,精确放行(系统↔数据严格隔离;动态沙箱完全隔离模板)。
- 后端 ServiceAccount 最小 RBAC,仅能管理动态沙箱/仿真计算命名空间与 Pod,绝不集群管理员。
- ValidatingAdmissionPolicy 约束后端 ServiceAccount 只能管理带对应引擎所有权标签的 `sbx-*`/`judge-*`/`battle-*`/`sim-*` 命名空间。
- 所有工作负载 PodSecurity restricted:non-root、禁特权、只读根、drop ALL capabilities、seccomp RuntimeDefault。
- 密钥经 Secret/KMS 注入,不入镜像/不入仓库。
- 镜像签名校验门禁(`base/admission/image-signature-policy.yaml`,集群侧 Sigstore policy-controller 执行)。

## 生产集群前置能力

`prod-saas` 集群必须安装并配置:

- External Secrets Operator:同步 `config/external-secret/` 定义的 `chaimir-secret`。
- metrics-server:暴露 `metrics.k8s.io`,供 M2 沙箱资源用量读取真实 CPU/内存指标。
- CSI snapshotter:提供 `VolumeSnapshot` CRD 与 snapshot-controller;生产还必须接入支持快照的 CSI
  存储驱动并创建 `VolumeSnapshotClass`。
- Sigstore policy-controller:执行平台镜像 Cosign 签名门禁。应用包含 `ClusterImagePolicy`,因此部署平台 overlay 前必须先运行 `make policy-controller-up`,由容器化 Helm 安装 CRD/controller 并把本地 `config/cosign/cosign.pub` 注入 `cosign-system/cosign-public-key` Secret。
- Ingress Controller 与证书签发器:执行 Ingress HTTPS 与证书引用。
