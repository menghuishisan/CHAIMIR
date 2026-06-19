# Chaimir 部署(deploy/)

Chaimir 区块链「教学·实验·竞赛」平台的部署清单。双形态:SaaS 公网多租户 + 学校私有化。
编排用 **Kustomize**(base + overlays),依据 `docs/总-部署架构设计.md` 与 `docs/总-镜像与容器设计.md`。

> 注:应用镜像(chaimir/backend|frontend|migrate|cron)为逻辑占位,由目录2(backend)/目录3(images)
> 构建产出后经 overlay 的 `images:` 覆盖生效。本目录提供的是完整运行底座。

## 目录结构

```
deploy/
├── Makefile                 本地一键拉起/销毁/校验
├── config/                  「唯一」环境变量源(单一来源)
│   ├── chaimir.env          全平台非密配置(每项带注释)
│   ├── secret.env.example   密钥模板(复制为 secret.env,被 .gitignore 忽略)
│   ├── external-secret/      SaaS ExternalSecret 统一模板(overlay 只覆盖环境差异)
│   └── kustomization.yaml   config component:生成两命名空间 ConfigMap
├── base/                    通用资源(命名空间/RBAC/应用/入口/策略/定时任务)
│   ├── namespaces/          chaimir-system / chaimir-data / monitoring + PodSecurity
│   ├── rbac/                后端 SA + 最小 ClusterRole(动态沙箱命名空间管理)
│   ├── backend/             后端单体 Deployment + Service
│   ├── frontend/            前端 Nginx Deployment + Service
│   ├── migrate/             迁移 + RLS 初始化 + seed Job
│   ├── ingress/             Ingress(/ 前端、/api 后端、/ws Hub)
│   ├── networkpolicy/       静态命名空间 deny-all + 精确放行;动态沙箱 deny-all 模板
│   └── cronjobs/            每日备份;业务生命周期清理由各模块后台任务负责
├── components/middleware/   PG16/Redis7/NATS2.10/MinIO 单实例(overlay 按需 include)
├── overlays/
│   ├── local-dev/           现有 K8s 集群 + 单实例中间件 + 本地镜像 + dev 密钥
│   ├── staging/             SaaS 形态,中间件外接,main 自动部署
│   ├── prod-saas/           多副本 + HPA,中间件外接,真实证书/KMS
│   └── prod-school/         k3s,单实例中间件,关平台层(私有化单校)
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

前置:`docker`、`kubectl`(内置 kustomize);可选 `helm`(仅 Harbor)、`kubeconform`(校验)。本仓库不要求额外安装 kind:只要当前 kubeconfig 指向可用 Kubernetes 集群即可,例如 Docker Desktop Kubernetes、k3s 或生产/预发布集群。

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
> 镜像构建后,根据当前集群类型将本地 `:dev` 镜像导入集群或推送到 overlay 指向的镜像仓库后即可转 Running。

## 校验(无需集群)

```bash
make render      # 渲染四个 overlay,检查 kustomize 构建
make validate    # 渲染 + kubeconform 校验(需安装 kubeconform)
```

或单独渲染:`kubectl kustomize --load-restrictor=LoadRestrictionsNone overlays/local-dev`。

## CI/CD

GitHub Actions(`.github/workflows/`)+ 可复用配置(`deploy/ci/`):

- `backend.yml` / `frontend.yml` / `images.yml`:路径触发 → lint+测试 → 构建 →
  Trivy 扫描(高危阻断)→ Cosign 签名 → 推 Harbor。三者共用 `ci/build-scan-sign-push` composite action。
- `deploy.yml`:`main` → 自动部署 staging;打 `v*` tag → 人工审批(GitHub Environment 保护)→ prod-saas。

所需 GitHub Secrets:`HARBOR_REGISTRY`、`HARBOR_USERNAME`、`HARBOR_PASSWORD`、
`COSIGN_KEY`、`COSIGN_PASSWORD`、`KUBECONFIG_STAGING`、`KUBECONFIG_PROD_SAAS`。

## 安全基线

- NetworkPolicy 默认 deny-all,精确放行(系统↔数据严格隔离;动态沙箱完全隔离模板)。
- 后端 ServiceAccount 最小 RBAC,仅能管理动态沙箱命名空间/Pod,绝不集群管理员。
- ValidatingAdmissionPolicy 约束后端 ServiceAccount 只能管理带平台所有权标签的 `sbx-*`/`judge-*`/`battle-*` 命名空间。
- 所有工作负载 PodSecurity restricted:non-root、禁特权、只读根、drop ALL capabilities、seccomp RuntimeDefault。
- 密钥经 Secret/KMS 注入,不入镜像/不入仓库。
- 镜像签名校验门禁(`base/admission/image-signature-policy.yaml`,集群侧 Sigstore policy-controller 执行)。

## 生产集群前置能力

`prod-saas` 集群必须安装并配置:

- External Secrets Operator:同步 `config/external-secret/` 定义的 `chaimir-secret`。
- metrics-server:暴露 `metrics.k8s.io`,供 M2 沙箱资源用量读取真实 CPU/内存指标。
- CSI snapshotter:提供 `VolumeSnapshot` CRD 与 snapshot-controller;生产还必须接入支持快照的 CSI
  存储驱动并创建 `VolumeSnapshotClass`。
- Sigstore policy-controller:执行平台镜像 Cosign 签名门禁。
- Ingress Controller 与证书签发器:执行 Ingress HTTPS 与证书引用。
