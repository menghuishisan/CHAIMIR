# Chaimir 镜像层

本目录是 Chaimir 全量镜像治理目录。这里不按后端模块划分,而按镜像职责划分:`service`、`runtime`、`infra`、`tool`、`judger`、`sim`、`sidecar`、`init`、`base`、`middleware`、`observability`、`ingress`。

## 维护原则

- 一个镜像一个目录,目录内至少维护 `manifest.yaml` 和 `README.md`。`source.type=platform-built`、`thin-wrapper`、`build-base` 必须维护 Dockerfile;`source.type=upstream-pinned` 不重打包,不得维护 Dockerfile。
- 成熟官方镜像能满足需求时优先复用官方镜像,只做安全基线、元数据和必要薄封装;平台确有特殊教学、判题、初始化或安全隔离需求时才自研。
- 纯上游固定镜像也必须在 manifest 中声明 registry、image、version、license、digest 锁、离线导入、`deploy/config/chaimir.env` 环境变量键、端口、卷、备份、网络策略、学生权限和供应链门禁。
- 平台自建、薄封装和构建基座镜像的生产 digest 统一来自 CI 推送后的 `image-digests.lock` 锁文件;不得在 manifest 中维护第二套构建产物 digest 来源。
- 镜像拉取必须使用不可变 digest。`images/pull-images.ps1` 会拒绝任何缺少 `upstream.digest`、组件 `digest` 或构建产物锁文件条目的镜像,不得退回 tag 拉取。
- 镜像只负责自身进程、依赖和默认容器端口;一个或多个镜像如何组成容器组,由 M2 沙箱控制面和部署层按 manifest 编排。
- 本目录不得维护固定组合矩阵、固定 bundle 或镜像到镜像白名单。`manifest.yaml` 只声明本镜像能力、生态标签、端口、安全域和资源约束;具体容器组由 `runtime.adapter_spec`、实验/题目配置与 M2 编排器动态校验后生成。
- 教师脚本、题目固化资产或链上依赖合约即可表达的轻量模拟能力,不得进入平台必需镜像清单。真实链下基础设施必须用真实镜像或官方薄封装,不得用 `*-mock` 服务冒充。
- J2 链上断言、J3 Flag、J5 仿真检查点由 M3 后端策略统一承接,不得在 `images/judger` 下重复维护执行器镜像。镜像判题只保留需要独立工具链/沙箱命令的 J1 测试用例和 J4 静态扫描。
- 容器内部端口优先沿用官方默认端口;生产禁止固定宿主机端口、`hostPort`、`hostNetwork`;本地开发宿主机映射必须可配置并默认绑定 `127.0.0.1`。
- 学生可进入容器不得挂载密钥、判题私有数据、宿主机路径、ServiceAccount token、答案、flag 或其他用户数据。
- PostgreSQL、Redis、MinIO、NATS、CouchDB、Harbor、Ingress、监控等基础设施镜像也在本目录治理;是否有 Dockerfile 由 `source.type` 决定,不是由分类决定。

## 实现范围

`docs/总-镜像与容器设计.md` 列出的全部镜像都属于本层交付范围,必须逐个手写目录并达到生产标准。成熟上游镜像能满足职责时采用纯上游固定或薄封装和安全元数据约束;需要平台确定性教学行为、判题、初始化或真实适配服务时才自研实现。不得用空启动进程、临时兼容分支或文字声明替代真实镜像职责。

## 拉取脚本

`images/pull-images.ps1` 是镜像层的生产拉取脚本,按 `manifest.yaml` 排序逐个拉取镜像引用,不会并发拉取。默认 `-Scope all`,会同时处理纯上游固定镜像和已构建镜像;也可用 `-Scope upstream-pinned` 或 `-Scope built` 做分段验证。

上游固定镜像从 manifest 的 `upstream.digest` 或 `upstream.components[].digest` 生成 `registry/image@sha256:...`。平台自建、薄封装和构建基座镜像只从 `image-digests.lock` 读取 digest,格式为:

```text
runtime/evm-hardhat sha256:<64位hex>
service/backend sha256:<64位hex>
```

CI 的 images 流水线在推送 Harbor 后解析真实 digest,上传 `image-digest-*` 锁片段。发布或离线包制作时必须把已构建镜像的锁片段合并为 `images/image-digests.lock`,再执行:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File images/pull-images.ps1 -Scope all -Registry harbor.chaimir.local
```

脚本默认每个镜像最多尝试 3 次。单次 `docker pull` 失败后,会先执行 `docker image rm <ref>` 清理失败引用,再进入下一次重试;全部重试失败后返回非 0。生产不应启用 `-NoCleanupFailedPull`,该开关仅用于诊断 Docker 本地状态。
