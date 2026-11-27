# infra/postgres

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

PostgreSQL 主数据库官方薄封装镜像。

## command / args 语义

- manifest 未声明自定义组件 command；入口保持官方 `docker-entrypoint.sh`，默认执行上游 `postgres`。
- 内置中间件编排通过显式 args 启用 PostgreSQL TLS：`postgres -c ssl=on -c ssl_cert_file=/etc/postgresql/tls/tls.crt -c ssl_key_file=/etc/postgresql/tls/tls.key`；这不是镜像内 fallback，其他 CompositionSpec 不得添加未声明参数。
- `POSTGRES_INITDB_ARGS` 由唯一配置源 `deploy/config/chaimir.env` 注入，固定为 Unix socket 与 TCP 均使用 `scram-sha-256`，并设置 `UTF8` 与 `C` locale；初始化后修改该值不会迁移已有数据目录。

## 初始化配置

- 非密配置键：POSTGRES_INITDB_ARGS；由 `chaimir-data-config` 注入，不允许在镜像层或 Secret 中复制定义。
