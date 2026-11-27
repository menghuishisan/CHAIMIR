# base/go-builder

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

Go 多阶段构建基座镜像。
构建基座复用 manifest 固定的官方 Go Alpine 上游，并在同一固定 Alpine 仓库内安装项目构建链需要的 make、git、curl；不在 Dockerfile 中从公网重新下载 Go 工具链，源码依赖仍由正式构建链的受控代理或缓存提供。
