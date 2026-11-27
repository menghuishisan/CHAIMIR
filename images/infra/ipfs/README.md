# infra/ipfs

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

IPFS/Kubo 内容寻址存储节点镜像。

## 初始化材料与运行时注入

- 首次启动由固定 Kubo 源码编译的初始化器生成实例身份，通过 `runtime-state` 内的私有临时目录交给官方 `ipfs init`。无论生成或初始化是否成功都清理临时目录，保留初始化与清理错误；已有 repo 不重新生成身份。实例身份不是平台 Secret/KMS 材料。

## Secret / KMS

- 必需材料映射：无；平台 Secret/KMS 不向本镜像注入业务密钥，实例 node identity 仅按上述受控初始化流程生成并持久化在 runtime-state。
