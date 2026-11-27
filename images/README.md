# 镜像入口

> 更新：2026-11-27。

职责、分类、安全门禁、供应链、选择性刷新与清理边界统一维护在[镜像与容器设计](../docs/总-镜像与容器设计.md)。运行环境、配置与集群入口见[部署说明](../deploy/README.md)及[部署架构](../docs/总-部署架构设计.md)。

每个镜像目录的 `manifest.yaml` 是机器字段权威，Dockerfile 表达构建；README 只补充该镜像独有的用途、前置、使用语义与专项验收。不从目录存在推断镜像已构建、已晋升或业务可用。

## 脚本入口

| 脚本 | 职责 |
| --- | --- |
| [build-images.ps1](build-images.ps1) | 按真实构建依赖顺序构建并推送 Harbor，生成候选 digest 锁 |
| [pull-images.ps1](pull-images.ps1) | 按正式或候选锁顺序回拉不可变镜像；支持 `all/upstream-pinned/built` 范围 |
| [validate-image-metadata.ps1](validate-image-metadata.ps1) | 核对目录、机器元数据、正式 digest 及供应链证明 |
| [sync-image-metadata.ps1](sync-image-metadata.ps1) | 消费验证片段，统一晋升权威锁与部署引用 |
| [cleanup-local-images.ps1](cleanup-local-images.ps1) | 按项目归属、锁和实际引用清理本地镜像 |

全量构建与离线拉取示例：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File images/build-images.ps1 -Registry registry.chaimir.io -Tag $env:GIT_COMMIT -DigestLock .tmp/backend-functional-test/evidence/candidate-image-digests.lock -DigestLockOut .tmp/backend-functional-test/evidence/candidate-image-digests.lock -Platform linux/amd64
powershell -NoProfile -ExecutionPolicy Bypass -File images/pull-images.ps1 -Scope all -Registry registry.chaimir.io -DigestLock .tmp/backend-functional-test/evidence/candidate-image-digests.lock
```

候选构建后继续完成同 digest 的扫描、SBOM、签名、验签、真实组合及回收验证，达到所属发布门禁后晋升。源码迭代按真实依赖确定 `-Images` 集合，采用同一流程；具体门禁和证明权威见[构建规范](../docs/总-镜像与容器设计.md#十二构建规范)。本地开发可复用项目 BuildKit builder；生产构建节点与业务运行节点的隔离要求仍由镜像和部署规范约束。
