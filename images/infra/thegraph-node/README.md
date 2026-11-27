# infra/thegraph-node

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

The Graph subgraph 索引节点镜像。

## bindings 与 links

- 三个必需连接分别提供链 RPC、索引数据库与 IPFS，目标由教师显式选择并随组合冻结。RPC 提供者可以是链节点或已声明同一能力的 RPC 网关，均不共享其账本卷。
- 数据库绑定只接收 `主机:端口`，端口范围 1–65535；链 RPC 和 IPFS 绑定须为无内嵌凭据和片段、带显式有效端口的 HTTP(S) URL。
- 网络名称、数据库名和账户参数可替换清单默认值；数据库配置须与所选提供者一致。

## 端口与服务

- 三个端点分别声明 ClusterIP Service；admin 与 index-status 不直接向学生或公网开放，只经已授权的显式连接访问。

## command / args 语义

- 入口校验绑定后替换 `POSTGRES_URL`、`ETHEREUM_RPC` 与 `IPFS`，不追加重复环境键；原生参数按 argv 传递。

## Secret / KMS

- 数据库密码由标准库转义后写入原生连接 URL，完整保留空白与特殊字符，不在错误中回显。
- 原生 URL 生成后清除 `POSTGRES_PASSWORD` 环境键；POI token 仅保留原生键的一份值。
- 数据库连接要求 TLS；证书由提供者经平台受控来源注入。

## 持久化与隔离边界

- `volume_source: self` 将组件的 runtime-state 与链账本隔离。索引数据写入所绑定的 PostgreSQL，内容经 IPFS 获取；不挂载链节点的私有状态或工作区。

## 验收条件

- 使用真实 RPC、PostgreSQL 与 IPFS 部署 subgraph，查询 `index-status` 的进度及错误，触发链上事件并用 GraphQL 验证结果；另验证 RPC 网关提供者、重启恢复与未授权访问拒绝。
