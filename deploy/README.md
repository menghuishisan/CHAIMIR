# Chaimir 部署(deploy/)

Chaimir 区块链「教学·实验·竞赛」平台的部署清单。双形态:SaaS 公网多租户 + 学校私有化。
编排用 **Kustomize**(base + overlays),依据 `docs/总-部署架构设计.md` 与 `docs/总-镜像与容器设计.md`。

> 注:应用镜像(chaimir/backend|frontend|migrate)为逻辑占位,由目录2(backend)/目录3(images)
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
│   ├── worker/              判题/撮合 worker(同镜像不同启动参数)
│   ├── frontend/            前端 Nginx Deployment + Service
│   ├── migrate/             迁移 + RLS 初始化 + seed Job
│   ├── ingress/             Ingress(/ 前端、/api 后端、/ws Hub)
│   ├── networkpolicy/       静态命名空间 deny-all + 精确放行;动态沙箱 deny-all 模板
│   └── cronjobs/            僵尸沙箱回收/PV 清理/每日备份/镜像 GC
├── components/middleware/   PG16/Redis7/NATS2.10/MinIO 单实例(overlay 按需 include)
├── overlays/
│   ├── local-dev/           kind 集群 + 单实例中间件 + 本地镜像 + dev 密钥
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

- `chaimir-config`(chaimir-system)— 后端/worker/migrate `envFrom` 注入。
- `chaimir-data-config`(chaimir-data)— 中间件 Pod 引用(同源同值,无重复定义)。

各 overlay 仅用 `behavior: merge` 覆盖差异键(如 `DEPLOY_MODE`、`PLATFORM_LAYER_ENABLED`、
中间件外接端点),不重写整份配置。

**密钥**走 `secret.env`(从 `config/secret.env.example` 复制,被 `.gitignore` 忽略):

- `local-dev` / `prod-school`:overlay 用 `secretGenerator` 从本地 `secret.env` 生成
  `chaimir-secret`(系统层)+ `chaimir-data-secret`(数据层)。
- `staging` / `prod-saas`:不提交任何密钥 —— overlay 引用 `config/external-secret/`
  统一模板,由 External Secrets Operator 从 KMS/Secret Manager 同步出 `chaimir-secret`。
  集群侧 `ClusterSecretStore` 由运维以 Secret/KMS 管理方式创建,仓库不硬编码第三方端点或凭据。

## 一键拉起本地环境

前置:`docker`、`kubectl`(内置 kustomize)、`kind`;可选 `helm`(仅 Harbor)、`kubeconform`(校验)。

```bash
cd deploy
make dev-up      # 建 kind 集群 → 装 ingress-nginx → 起中间件 → apply local-dev → 跑 migrate
make dev-down    # 销毁 kind 集群
```

`make dev-up` 首次会自动生成随机 dev 密钥到 `overlays/local-dev/secret.env`。

将 `chaimir.local` 指向 `127.0.0.1`(hosts)后,前端经 `http://chaimir.local` 访问。

> 应用镜像就绪前(目录2/3 未产出),backend/frontend/migrate Pod 会处于 ImagePull 待命 —— 属预期。
> 镜像构建后:`make load-images` 将本地 `:dev` 镜像 load 进 kind 即转 Running。

## 校验(无需集群)

```bash
make render      # 渲染四个 overlay,检查 kustomize 构建
make validate    # 渲染 + kubeconform 校验(需安装 kubeconform)
```

或单独渲染:`kubectl kustomize overlays/local-dev`。

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
- Sigstore policy-controller:执行平台镜像 Cosign 签名门禁。
- Prometheus Adapter:暴露 `chaimir_nats_queue_backlog{queue="judge|matchmaking"}` external metric,
  供 worker HPA 按队列积压伸缩。
- Ingress Controller 与证书签发器:执行 Ingress HTTPS 与证书引用。
