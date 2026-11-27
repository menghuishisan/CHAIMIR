# infra/postgres-graph

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

The Graph 索引用 PostgreSQL 配套镜像。

## bindings 与 links

- 可配置数据库名和账户，默认分别为 `graph` 和 `postgres`；消费者须使用与本组件一致的参数和数据库 Secret。

## command / args 语义

- 入口写入并校验受控 TLS 材料，再交给官方 PostgreSQL 入口；组件参数 `-c max_connections=200` 原样传递，TLS 配置由入口强制启用。

## Secret / KMS

- Secret 环境键：POSTGRES_PASSWORD、POSTGRES_TLS_CERTIFICATE、POSTGRES_TLS_PRIVATE_KEY，分别引用平台 PG_PASSWORD 与已有 PostgreSQL 证书、私钥来源；只注入 infra 组件。
- 每次启动将证书和私钥以 0600 权限写入 `/runtime-state/postgres/tls`，校验证书未过期且公钥匹配；写入后从交接环境移除证书与私钥变量。无效材料在启动 PostgreSQL 前拒绝。
