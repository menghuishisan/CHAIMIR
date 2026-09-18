# middleware/postgres

## 镜像职责

PostgreSQL 主数据库官方薄封装镜像。

## 能力边界

- 支持 capability：relational-database, rls-tenant-store
- 明确不支持：manifest 未声明的能力、协议动作和跨镜像联动；入口和编排必须严格失败，不使用 generic fallback。
- conflicts：无；cardinality：未声明；placement：未声明。

## 合法 runtime / provider

- 生态或适配范围：manifest 未固定生态，由 CompositionSpec 按声明能力选择。
- 必需 provider 能力：无。
- provider 只能通过 manifest 能力和显式 CompositionSpec link 选择；多个候选必须显式指定，不能按名称猜测。

## bindings 与 links

- 必需 bindings：无。
- 必需 links：无；本镜像不声明跨组件 link。
- binding 的 endpoint、协议、配置键和启动前置以 manifest 为准；缺失、重复、错误能力或错误 provider 均拒绝编排。

## 端口与服务

- postgres：5432/TCP，暴露边界 internal，用途：PostgreSQL wire protocol。
- 组件：无 resource_spec 组件。

## command / args 语义

- manifest 未声明自定义组件 command；入口保持官方 `docker-entrypoint.sh`，默认执行上游 `postgres`。
- 内置中间件编排通过显式 args 启用 PostgreSQL TLS：`postgres -c ssl=on -c ssl_cert_file=/etc/postgresql/tls/tls.crt -c ssl_key_file=/etc/postgresql/tls/tls.key`；这不是镜像内 fallback，其他 CompositionSpec 不得添加未声明参数。
- `POSTGRES_INITDB_ARGS` 由唯一配置源 `deploy/config/chaimir.env` 注入，固定为 Unix socket 与 TCP 均使用 `scram-sha-256`，并设置 `UTF8` 与 `C` locale；初始化后修改该值不会迁移已有数据目录。

## 初始化材料与运行时注入

- 镜像层不内置租户、链、协议或私有材料；声明的 init_assets：无。
- genesis、rollup、checkpoint、Hyperlane、Chainlink、合约或 subgraph 材料必须由平台受控对象存储、初始化资产和 CompositionSpec 注入并校验来源及 SHA-256；缺失时严格失败。
- 数据卷声明：[{"name":"data","mount_path":"/var/lib/postgresql","backup_required":true}]

## Secret / KMS

- Secret 环境键：PG_PRIV_USER, PG_PRIV_PASSWORD。
- 非密配置键：POSTGRES_INITDB_ARGS；由 `chaimir-data-config` 注入，不允许在镜像层或 Secret 中复制定义。
- 必需材料映射：[{"env":"PG_PRIV_USER","source":"deploy/config/chaimir.env","runtime_value":"secret-or-kms"},{"env":"PG_PRIV_PASSWORD","source":"deploy/config/chaimir.env","runtime_value":"secret-or-kms"}]
- Secret 只来自平台 Secret/KMS 运行时注入，不能写入 Dockerfile、manifest、README、镜像层或测试输出。

## 持久化与隔离边界

- 持久化目录：[{"name":"data","mount_path":"/var/lib/postgresql","backup_required":true}]
- 组件挂载与文件系统：无
## 安全边界

- 安全声明：{"run_as_non_root":true,"read_only_root_filesystem":false,"allow_privilege_escalation":false,"privileged":false,"host_network":false,"automount_service_account_token":false,"drop_capabilities":["ALL"],"network_policy":"deny-all"}
- student_access：{"enabled":false}。
- 网络只允许 manifest binding/link 所需目标，默认拒绝；禁止宿主机路径、hostNetwork、ServiceAccount token 和越权写入。

## readiness / health

- 未声明组件 readiness probe；容器运行不等于就绪或业务成功。
- 上游健康检查：{"type":"exec","command":["pg_isready"]}。

## 清理

- 验收后按 CompositionSpec 所属租户/实例和项目 owner 标签删除容器、Pod、Service、NetworkPolicy、临时卷、Secret 投影和临时材料；保留平台长期资源。
- 不使用 broad prune，不删除 kind-chaimir-cilium 节点、正式锁引用或其他项目资源。

## 验收条件

- 先按 digest lock 启动合法 CompositionSpec，核对 command/args、bindings、links、材料和 Secret 投影，再执行 manifest selftest。
- selftest 只证明镜像布局和声明入口正确；协议、链、存储、索引、签名或跨镜像能力必须追加真实 provider 联动、正向业务、错误输入拒绝、重启/持久化（若声明）和清理证据。
- 未执行或只完成探活的项目不得标记为业务通过。
