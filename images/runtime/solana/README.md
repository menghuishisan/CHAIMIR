# runtime/solana

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

Solana test-validator 教学运行时镜像。

## command / args 语义

Solana 调用 GNU tar 创建和恢复账本归档。镜像直接使用固定 Debian 工具层的 GNU tar，并提供压缩子进程依赖的固定 BusyBox `sh`、`bzip2` applet，保留 `jcfhS`、sparse 及文件名原义，不经过自写参数转换。工具版本检查只证明入口可执行，账本归档与恢复仍需真实验收。
