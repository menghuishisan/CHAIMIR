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
│   ├── secret.env.example   受控密钥源输入模板(不直接注入应用 Pod)
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
├── clusters/cilium-kind/    无默认 CNI Kind 配置、生产同版 Cilium values 与验收入口
├── components/middleware/   PG16/Redis7/NATS2.10/MinIO 单实例(overlay 按需 include)
├── overlays/
│   ├── acceptance/          生产安全基线 + 单实例测试数据组件 + Harbor digest 镜像
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

ingress-nginx 使用 `charts/ingress-nginx/values.yaml` 保存生产安全基线,由容器化 Helm
按 `config/chaimir.env` 中固定的官方 chart 版本安装。控制器镜像 digest 只能从
`images/image-digests.lock` 注入,chart 与控制器保持同一 1.15 版本代;admission 证书工具
使用 `images/ingress/kube-webhook-certgen/manifest.yaml` 固定的上游 digest。部署入口不再
下载和修改远程静态清单,避免版本、RBAC 与安全上下文漂移。

数据库初始化脚本不放在 `deploy/charts/`:建库、应用角色、RLS 初始化入口、seed 编排归
`scripts/db/` 与 `backend/db/migrations/`;本目录只负责通过 `chaimir-migrate` Job 调度执行。

## 环境变量(单一来源)

全平台非密配置只在 **`config/chaimir.env`** 一处定义(每个变量带注释)。它经 `config/` 这个
Kustomize component 同时生成两个命名空间的 ConfigMap:

- `chaimir-config`(chaimir-system)— 后端/migrate `envFrom` 注入。
- `chaimir-data-config`(chaimir-data)— 中间件 Pod 引用(同源同值,无重复定义)。

各 overlay 仅用 `behavior: merge` 覆盖差异键(如 `DEPLOY_MODE`、`PLATFORM_LAYER_ENABLED`、
中间件外接端点),不重写整份配置。

`backend/.env`、`frontend/.env` 与 `config/secret.env` 都是被 Git 忽略的私有文件,不得提交。Kubernetes 运行中的后端不直接读取这些工作区文件:非密配置只由 `config/chaimir.env` 生成 ConfigMap,密钥统一由 External Secrets Operator 从受控 SecretStore 同步。`config/secret.env` 是私有化和验收密钥源的受控运维输入,也供浏览器验收读取账号凭据;它只更新 `chaimir-secrets/chaimir` 源 Secret,不会直接生成或注入应用 Secret。`frontend/.env` 仅作为前端镜像构建期的 `VITE_*` 输入;`VITE_*` 会进入浏览器静态资源,严禁放密码、token 或私钥。前端部署形态和工具 origin 由初始化容器从同一个 `chaimir-config` 生成 `/runtime-config.js`,不再维护 overlay 专用配置文件。

**密钥**统一走 ExternalSecret + SecretStore/KMS。`secret.env.example` 仅列出受控验收账号需要的键名,实际部署值必须先写入环境对应的密钥提供方:

`make tls` 会在 `config/chaimir-tls/` 持久化生成有效期 825 天的 `www.chaimir.io` 自签证书，并创建统一名称 `chaimir-tls`。证书文件被 `.gitignore` 忽略，不会上传到 GitHub；文件存在且仍有效时，重复启动不会重新生成。浏览器入口统一使用 `https://www.chaimir.io`，需将域名解析到 `127.0.0.1` 并信任 `config/chaimir-tls/tls.crt`；Cookie 不提供 HTTP 降级。

staging 和 prod-saas 部署流水线会在应用资源后等待 `chaimir-system/chaimir-tls`，并校验 Secret 类型为 `kubernetes.io/tls` 且同时包含证书和私钥；缺失或不完整会阻断部署。prod-school 交付前必须由学校运维在同一命名空间预创建同名 TLS Secret，再执行清单应用和 `bash deploy/scripts/wait-tls-secret.sh chaimir-system chaimir-tls 300` 验收。

- 所有 overlay 都引用 `config/external-secret/` 统一模板,由 External Secrets Operator 同步出 `chaimir-secret`。
- 包含内置中间件的部署形态同时引用 `config/external-secret-middleware/`,同步 `chaimir-data-secret` 和 `postgres-tls`。
  各目标命名空间的 `SecretStore` 由运维以 Kubernetes Secret/KMS/Vault 管理方式创建,仓库不硬编码第三方端点或凭据。

私有化和验收使用同一 Kubernetes Provider 契约。首次安装固定版本 Operator 后,运维在被忽略的 `config/secret.env` 填写全部真实值,准备 PostgreSQL 受信任证书和私钥,执行:

```bash
make external-secrets-up
make tls
make secret-source-up
```

`secret-source-up` 会校验必填键、拒绝模板占位值,把输入写入独立命名空间的唯一源 Secret,随后等待 `SecretStore` 与三个 `ExternalSecret` 全部 `Ready=True`;命令输出不包含敏感值。后续轮换仍修改同一受控输入并再次执行 `make secret-source-up`,不得手工修改 `chaimir-system/chaimir-secret` 或 `chaimir-data/chaimir-data-secret`。

SaaS 生产的 `chaimir-staging-secret-store`、`chaimir-prod-secret-store` 由基础设施团队连接 KMS/Vault;每个环境的密钥后端保存一个名为 `chaimir` 的结构化对象,对象字段与 `config/secret.env.example` 的键一致。应用、前端构建和部署清单均不保存这些真实值。

## 统一部署与验收环境

前置:`docker`、`kubectl`(内置 kustomize);可选 `kubeconform`(校验)。Trivy、Cosign、Helm 不要求安装到宿主机,统一通过 `image-supply-chain.compose.yaml` 的容器化工具入口运行。项目脚本下载固定 Kind `v0.32.0` 到被忽略的 `.tmp/tools`,校验官方 SHA256 后执行;不使用宿主机上的未知 Kind 版本。

Sigstore policy-controller 的 webhook 控制器会在运行期维护 CA 与 `namespaceSelector` 排除条件。容器化 Helm 4 安装该 chart 时固定使用客户端三方补丁模式,避免 Server-Side Apply 与控制器对同一 webhook 字段争夺所有权;这不改变 chart 声明的 `policy.sigstore.dev/include=true`、`failurePolicy=Fail` 或超时配置。canonical Harbor 使用受控 CA 时,部署入口把同一 CA 写入 `cosign-system/chaimir-registry-ca` ConfigMap,并通过 chart 原生 `webhook.registryCaBundle` 只读挂载;不得使用跳过 TLS 校验参数。

签名准入启用后,部署入口先应用静态 namespace,再把唯一 Harbor robot `dockerconfigjson` 以固定名称 `chaimir-harbor-pull` 同步到 `chaimir-system`、`chaimir-data` 与 `chaimir-prepull`,最后才提交受保护工作负载。policy-controller 的 `signaturePullSecrets` 从工作负载所在 namespace 读取该 Secret;不得把凭据同步放到工作负载创建之后。

所有运行位置执行同一套生产 Cilium 安全基线。`kind-chaimir-cilium` 从无默认 CNI 状态创建并运行完整平台、数据中间件、gVisor 和动态沙箱;不得让 `docker-desktop` 的 Kindnet 集群继续运行 Chaimir 业务。迁移期间该旧集群只提供外部 Harbor,项目脚本会删除其中的旧 Chaimir 测试命名空间。工作区直跑后端或前端不作为平台启动方式。若 Kind 节点继承的宿主代理指向回环地址,集群引导只在节点 containerd 服务层把代理主机改写为 `host.docker.internal`,避免把节点自身误当成宿主;同时由 CoreDNS 将 canonical Harbor 域名解析到同一宿主网关并通过临时 Pod 实测。Kubernetes 业务清单、镜像 digest 和安全策略不因此改变。

短信验证码仍走后端唯一的 HTTP 网关发送链路。`acceptance` 的 `APP_ENV=test` 只用于开启受控验收 seed 与集群内短信夹具这两类测试边界,不改变 Cilium、gVisor、TLS、镜像签名、NetworkPolicy 或应用资源等生产安全基线。短信夹具把 `SMS_HTTP_ENDPOINT_SCOPE` 设为 `cluster`,并限制到 `10.96.0.0/12` Service CIDR;正式交付使用 `APP_ENV=prod` 与 `public` 外部网关。该夹具由 `scripts/e2e/start-sms-gateway.ps1` 临时创建并按标签清理,不是项目运行时镜像,也不改变业务发送流程。

检查运行集群与外部 Harbor:

```bash
kubectl --context docker-desktop -n harbor get pods
kubectl --context kind-chaimir-cilium get nodes
```

完整启动使用唯一入口:

```bash
cd deploy
make dev-up
```

该目标会验证外部 Harbor、创建或校验无默认 CNI 的 `kind-chaimir-cilium`、安装正式 digest 的 Cilium 并通过 NetworkPolicy 数据面测试,再安装 gVisor 并实际运行隔离 Pod 验证用户态内核、安装 ingress、metrics-server、policy-controller,部署完整 overlay 并运行迁移任务。启动完成后将 `www.chaimir.io` 解析到 `127.0.0.1`,信任 `config/chaimir-tls/tls.crt`,通过 `https://www.chaimir.io` 访问。验收 seed 只由明确的 seed Job 生成,不会进入 SaaS/学校交付 overlay;CNI、证书、签名、隔离运行时、日志级别和网络策略全部执行生产标准。

`make dev-up` 是完整环境的唯一首次入口,不会接受 Kindnet 集群作为运行目标。已有环境的日常生命周期分为三个明确入口:`make dev-start` 只恢复现有 Kind 节点、动态地址、Cilium 健康状态与 HTTPS 入口,不执行 Helm upgrade、NetworkPolicy smoke、gVisor smoke、迁移或验收 seed;`make dev-refresh` 只在镜像、配置或数据库源码变化后恢复现有集群并重新应用唯一 overlay、迁移和验收 seed,不重新安装 Cilium;`make dev-stop` 通过 Docker pause 暂停项目 Kind 节点的 CPU 执行并保留内存、volume、数据库进程与节点镜像缓存,避免用强制退出中断数据库。完整 Cilium 数据面与 gVisor 验收保留在首次 `dev-up` 和显式检查目标中,不会降级或形成第二套配置。所有环境只使用 `make metrics-up`;Kind 节点由集群入口为 kubelet 启用服务证书轮换,并只批准节点身份、主体和 SAN 均匹配的请求,不提供 `--kubelet-insecure-tls` 入口。

Docker Desktop 重启不会删除 Kind 节点 `/var` Docker volume 或 Harbor PVC,因此同一 digest 的 Harbor 数据和节点 containerd 缓存会保留。集群入口把项目 Kind 节点的 Docker restart policy 固定为 `unless-stopped`,并会主动启动仍存在但已停止的节点;若节点容器地址在重启后变化,入口只删除节点归属失效的派生 `CiliumEndpoint`,等待当前 Agent 以新地址重新发布,不重建业务 Pod 或清空镜像缓存。恢复验收从当前 `CiliumNode` 读取全部主机地址与健康端点地址,由每个 Kind 节点主动探测完整地址矩阵,不等待仍可能保留重启前结果的 Cilium 后台健康缓存。Docker 恢复后执行 `make dev-start` 即可恢复网站;只有镜像、配置或数据库源码变化时才执行 `make dev-refresh`。应用滚动完成后,刷新入口会按当前控制器和 Pod 引用精确清理未使用的 Kustomize 哈希 ConfigMap/Secret,不保留旧配置副本。不得执行 `make cilium-cluster-down`、删除 Harbor PVC、删除 Kind 节点 volume 或带 volume 的全局 Docker prune,否则相应集群数据或镜像缓存会被明确删除并需要重新拉取。

上述目标都是前台一次性运维命令,完成后会返回 PowerShell 提示符,网站继续由 Docker/Kubernetes 托管,不需要保持终端命令运行。执行中可用 `Ctrl+C` 停止等待;已经提交给 Kubernetes 的幂等变更不会自动回滚,重新执行同一目标即可继续收敛。需要停止网站时使用 `make dev-stop`,不要用全局 Docker 清理命令。

M2 运行时预拉取不拉取 `images/image-digests.lock` 的全部镜像。平台管理员针对一个运行时触发预拉取时,M2 只把该运行时主镜像、其基础设施组件和兼容可用工具组成的默认工作负载集合拉到每个可调度沙箱节点,并执行镜像自检;成功 DaemonSet 常驻以便新节点自动补拉。现有节点重启且 `/var` volume 未删除时,`imagePullPolicy=IfNotPresent` 会复用 digest 缓存。

集群级入口:

```bash
make cilium-cluster-up       # 创建/修复无默认 CNI 集群并安装生产同版 Cilium
make cilium-cluster-start    # 恢复现有集群、动态网络地址和外部 HTTPS 入口
make cilium-cluster-check    # 校验 Cilium 状态与真实 NetworkPolicy 阻断/放行
make cilium-cluster-stop     # 暂停项目 Kind 节点并保留进程、数据与镜像缓存
make cilium-cluster-down     # 删除项目 Kind 集群,不删除外部 Harbor
```

禁止在 `docker-desktop` 内删除 Kindnet 后直接安装 Cilium,也禁止 CNI chaining。Cilium Chart 固定为 `1.20.1@sha256:906ce40d35daad838d12add8a5ba7033e767767f51799a93c7eace2cec9cdc05`,agent/operator/Envoy 必须引用 `images/image-digests.lock` 的正式 Harbor digest,策略模式固定为 `always`。

M2 快照能力分为两层:通用 `VolumeSnapshot` CRD/snapshot-controller 与具体 CSI 存储驱动。
本仓库提供 `make snapshot-up` 安装官方 CSI snapshotter 集群组件,并提供 `make snapshot-check`
检查 CRD、`VolumeSnapshotClass` 与 `StorageClass` 是否存在。真正能创建快照还必须由集群安装
支持快照的 CSI 驱动,并在 `SANDBOX_STORAGE_CLASS_NAME`、`SANDBOX_VOLUME_SNAPSHOT_CLASS_NAME`
中填写真实类名。`rancher.io/local-path`、普通 Docker volume 或演示用 hostpath CSI 不作为生产
快照方案。

将 `www.chaimir.io` 指向 `127.0.0.1`(hosts)后,前端经 `https://www.chaimir.io` 访问,并信任 `config/chaimir-tls/tls.crt`。首次环境使用 `dev-up`;Docker 重启或此前执行过 `dev-stop` 时使用 `dev-start`;只有镜像、配置或数据库源码变化后才使用 `dev-refresh`。三个入口共用同一 Cilium 集群、生产安全基线和 acceptance overlay,不存在本地 CNI 或应用清单分叉。

> 应用镜像就绪前(目录2/3 未产出),backend/frontend/migrate Pod 会处于 ImagePull 待命 —— 属预期。
> 镜像必须由统一供应链直接推送 Harbor、按 digest 回拉并通过门禁,再由 `image-metadata-promotion` 晋升到权威锁及资源所属的 base/component;不得导入 `:dev` tag 绕过该流程。

### 本地代码变更的选择性镜像刷新

测试阶段仍使用 Kubernetes + HTTPS,不在宿主机直跑 Go 或 pnpm。为缩短单次修改后的刷新时间,只对实际受影响的服务执行构建、按 digest 回拉和供应链证明:

- `frontend/**` 或 `images/service/frontend/**` 只更新 `service/frontend`。
- `backend/**` 会同时更新 `service/backend`、`service/migrate`、`service/cron`,因为三个 Dockerfile 都复制后端源码。
- 只改 `images/service/backend/**`、`images/service/migrate/**` 或 `images/service/cron/**` 时,只更新对应服务。
- 共享构建基座 digest 变化时,按 Dockerfile 的真实 `FROM` 依赖扩展受影响集合;未受影响镜像继续使用正式锁中的原 digest。

选定集合必须使用 `images/build-images.ps1 -Images` 生成候选锁,再用 `images/pull-images.ps1 -Images` 按候选锁回拉。候选集合必须通过 `deploy/scripts/image-attestations-generate.ps1 -Images -NoEnvWrite -DigestFragmentsDir <证据目录>\\fragments`;脚本只有在选定集合全部验证成功时才写出 `verified-images.lock`。把该片段交给 `images/sync-image-metadata.ps1` 后,才更新正式 `images/image-digests.lock`、`deploy/base` 与资源所属 component 的镜像引用、`deploy/config/chaimir.env` 和存在时的环境 `backend/.env` 准入证明。禁止把候选锁直接复制成正式锁,也禁止用 tag、手工 digest、`kubectl set image` 或临时本地镜像绕过流程。最后仍执行 `make dev-refresh`;该命令只会因 digest 变化滚动更新对应工作负载,未变更服务继续使用原镜像。

## Harbor 与供应链工具

Harbor 使用官方 Helm chart,但 Helm 本身不安装到宿主机。先在 `config/chaimir.env` 填写 `SUPPLY_CHAIN_KUBECONFIG_HOST_PATH`,复制 `config/supply-chain.secret.env.example` 为 `config/supply-chain.secret.env` 并填写 Harbor 管理员密码、robot 凭据和 Cosign 私钥口令,再执行:

```bash
cd deploy
make supply-chain-tools-pull
make supply-chain-tools-check
make harbor-up
make harbor-projects-ensure
```

容器化 Helm 不读取宿主机隐含的当前 context。`make cilium-cluster-up` 会把平台运行集群写入被忽略的
`config/supply-chain.kubeconfig`,供 ingress-nginx、policy-controller 等运行组件使用;外部 Harbor
依赖集群写入 `config/harbor.kubeconfig`,只由 `make harbor-up` 显式使用。两个文件按职责分离,
不得把旧依赖集群 kubeconfig 继续当作平台运行集群入口。

Docker Desktop 本地没有固定入口 IP 时,不要把 Ingress 的 `172.18.x.x` 写入配置或 hosts;该地址会随
Docker/Kubernetes 重启变化。`SUPPLY_CHAIN_HARBOR_EXTERNAL_URL`、`SUPPLY_CHAIN_REGISTRY` 与
`SUPPLY_CHAIN_REGISTRY_ENDPOINT` 必须都等于同一个 HTTPS 域名 `registry.chaimir.io`。宿主机 hosts 只解析该域名，
镜像构建、签名、验签、digest 锁和集群运行时都走其标准 443 入口；禁止 port-forward、附加端口或直连 `harbor-core`，
否则上传路径、TLS 和生产链路会分叉。

`harbor-projects-ensure` 创建镜像规范里的 Harbor project:`service/runtime/infra/tool/judger/sim/sidecar/init/base/middleware/observability/ingress/network`,并创建供应链 robot 账号。平台镜像不得推到默认 `library`,否则 digest 锁和准入策略无法按分类审计。

该目标通过 `deploy/scripts/harbor-projects-ensure.ps1` 运行,避免把 PowerShell 逻辑塞进 Makefile 字符串里。首次创建 robot 时,Harbor 只返回一次 token;脚本会把 `HARBOR_ROBOT_USERNAME` 和 `HARBOR_ROBOT_PASSWORD` 回写到被忽略的 `deploy/config/supply-chain.secret.env`,不得提交。

镜像完成构建、推送和 digest 锁生成后,使用统一供应链入口生成沙箱准入证明:

```bash
cd deploy
make image-attestations-generate
```

该目标会通过容器化 Trivy/Cosign 扫描、生成 CycloneDX SBOM、签名镜像、签署 SBOM 证明并分别验证 digest 锁中的镜像,再把
`PLATFORM_IMAGE_ATTESTATIONS_JSON` 同步回写到 `deploy/config/chaimir.env` 与
`backend/.env`。Cosign 私钥目录固定由 `SUPPLY_CHAIN_COSIGN_KEY_HOST_DIR` 指向,默认是
`deploy/config/cosign/`;Docker registry 认证目录由 `SUPPLY_CHAIN_DOCKER_CONFIG_HOST_DIR`
指向,默认是 `deploy/config/docker-auth/`。这两个目录只保存本地/私有化凭据,已被 Git 忽略,
不得提交到仓库。`SUPPLY_CHAIN_REGISTRY_HOST_ALIAS` 只用于容器化供应链工具访问宿主机暴露的
Harbor 入口,必须与 `SUPPLY_CHAIN_REGISTRY` 的主机名一致。生产/预发布环境应由 CI/Harbor/KMS
提供对应密钥和认证配置。

本地供应链和实验测试都直接使用 `https://registry.chaimir.io` 的标准 443 入口；Docker Desktop
节点启动时由 `make registry-runtime-dns` 使用节点动态解析到的 `host.docker.internal` 网关维护
split-DNS，节点内不会把宿主机的 `127.0.0.1` hosts 映射当作 registry 地址；日常只打开 Docker
Desktop 做其他项目时不会占用额外 Harbor 端口。生产与本地不使用两套 registry 地址。

`SUPPLY_CHAIN_TRIVY_IMAGE`、`SUPPLY_CHAIN_COSIGN_IMAGE`、`SUPPLY_CHAIN_HELM_IMAGE` 使用 digest 固定。当前 Trivy 固定为 0.72.0,Cosign 固定为 2.4.3（与 policy-controller 0.13.1 的 legacy 签名格式契约一致）;GitHub 工作流也显式安装同一 Cosign 版本。本地、私有化与 CI 使用项目私钥且不上传公共透明日志。需要升级工具时,先拉取目标稳定版本并确认 digest与命令参数,再同步更新 `config/chaimir.env` 和工作流版本,不得提交可变 `latest`。


## 校验(无需集群)

```bash
make render      # 渲染四个 overlay,检查 kustomize 构建
make validate    # 渲染 + kubeconform 校验(需安装 kubeconform)
```

或单独渲染:`kubectl kustomize overlays/acceptance`。所有 overlay 必须在默认加载限制下通过。

## CI/CD

GitHub Actions(`.github/workflows/`)+ 可复用配置(`deploy/ci/`):

- `backend.yml` / `frontend.yml` / `images.yml`:路径触发 → lint+测试 → 构建 →
  Trivy 扫描(高危阻断)+ CycloneDX SBOM → 推 Harbor并解析 digest → Cosign 镜像签名、SBOM 证明和双重验证。三者共用 `ci/build-scan-sign-push` composite action;backend 独占 backend/migrate/cron,frontend 独占 frontend,通用 images 不重复构建四个服务镜像。缺少 registry/robot/Cosign Secret 时在构建前显式失败。
- `image-metadata-promotion.yml`:串行消费三条产物流水线的 digest 片段,同步权威 lock、base/component 镜像引用和受控配置引用,再由机器人 PR 自动合并;业务流水线不直接写 `main`。
- `deploy.yml`:`images/image-digests.lock` 合入 `main` 后,从同一权威锁按 digest 自动部署 staging;打 `v*` tag 后从该发布提交的锁按 digest 渲染,经 GitHub Environment 人工审批部署 prod-saas。

README、docs 或普通应用提交不直接创建 staging Deployment。只有已完成扫描、推送、签名、验签并由机器人 PR 晋升的权威锁变更才触发部署;backend、frontend、migrate、cron 必须同时存在有效 digest,不按 SHA tag、版本 tag 或 `latest` 降级。

所需 GitHub Secrets:`HARBOR_REGISTRY`、`HARBOR_USERNAME`、`HARBOR_PASSWORD`、
`COSIGN_KEY`、`COSIGN_PASSWORD`、`IMAGE_METADATA_BOT_TOKEN`、`KUBECONFIG_STAGING`、`KUBECONFIG_PROD_SAAS`。仓库还必须启用 Auto-merge;`IMAGE_METADATA_BOT_TOKEN` 使用能触发 PR 检查的 GitHub App 或细粒度 PAT,不得用默认 `GITHUB_TOKEN` 替代。

默认 `ubuntu-latest` runner 必须能通过 HTTPS 访问 `HARBOR_REGISTRY` 与目标 staging/prod Kubernetes API。本机 `registry.chaimir.io` 仅用于当前受控 Docker Desktop 环境，不能填写为 GitHub 托管 runner 的 registry。若交付环境只提供私网 Harbor/K8s,必须先在同一受控网络注册专用自托管 runner,再将镜像构建和部署 job 的 `runs-on` 收敛到该 runner 标签;不得通过暴露本机临时端口或提交本地凭据绕过网络边界。

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
- Sigstore policy-controller:执行平台镜像 Cosign 签名门禁。应用包含 `ClusterImagePolicy`,因此部署平台 overlay 前必须先运行 `make policy-controller-up`,由容器化 Helm 安装 CRD/controller 并把本地 `config/cosign/cosign.pub` 注入 `cosign-system/cosign-public-key` Secret。`policy-controller-up` 将准入 webhook 超时固定为 30 秒、保持 `failurePolicy=Fail`,并使用两个副本和 200m/500m CPU 请求/上限,覆盖一次性校验运行时、工具和基础设施镜像的预拉取请求。
- Ingress Controller 与证书签发器:执行 Ingress HTTPS 与证书引用。
