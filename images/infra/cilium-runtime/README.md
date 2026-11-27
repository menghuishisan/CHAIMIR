# infra/cilium-runtime

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

Cilium 构建与 agent 共用的安全运行基座，使用已验证 Go 1.26.6 工具链。
