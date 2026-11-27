# infra/cilium-builder

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

Cilium v1.20.1 agent 与 generic operator 的统一 Alpine 安全构建基座。
构建输入由固定的 Cilium 上游源码和 `scripts/prepare-cilium-source.sh` 准备；脚本只更新明确列出的安全依赖并同步 vendor。
