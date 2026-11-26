# infra/thegraph-node

## 镜像职责

The Graph subgraph 索引节点镜像。

## 能力边界

- 支持 capability：chain-indexer, subgraph-runner
- 明确不支持：manifest 未声明的能力、协议动作和跨镜像联动；入口和编排必须严格失败，不使用 generic fallback。
- conflicts：无；cardinality：one；placement：sandbox。

## 合法 runtime / provider

- 生态或适配范围：evm
- 必需 provider 能力：evm-json-rpc, graph-datastore, ipfs-http-api。
- provider 只能通过 manifest 能力和显式 CompositionSpec link 选择；多个候选必须显式指定，不能按名称猜测。

## bindings 与 links

- 三个必需连接分别提供链 RPC、索引数据库与 IPFS，目标由教师显式选择并随组合冻结。RPC 提供者可以是链节点或已声明同一能力的 RPC 网关，均不共享其账本卷。
- binding 的 endpoint、协议、配置键和启动前置以 manifest 为准；缺失、重复、错误能力或错误 provider 均拒绝编排。
- 数据库绑定只接收 `主机:端口`，端口范围 1–65535；链 RPC 和 IPFS 绑定须为无内嵌凭据和片段、带显式有效端口的 HTTP(S) URL。
- 网络名称、数据库名和账户参数可替换清单默认值；数据库配置须与所选提供者一致。

## 端口与服务

- graphql：8000/TCP，暴露边界 proxy，用途：GraphQL query API。
- admin：8020/TCP，暴露边界 internal，用途：Subgraph deploy API。
- index-status：8030/TCP，暴露边界 internal，用途：索引进度、健康与错误查询。
- 三个端点分别声明 ClusterIP Service；admin 与 index-status 不直接向学生或公网开放，只经已授权的显式连接访问。
- 组件：infra。

## command / args 语义

- 镜像默认 ENTRYPOINT：["/usr/local/bin/chaimir-thegraph-entrypoint"]；manifest 未覆盖组件 command/args，使用该入口语义。
- 入口校验绑定后替换 `POSTGRES_URL`、`ETHEREUM_RPC` 与 `IPFS`，不追加重复环境键；原生参数按 argv 传递。

## 初始化材料与运行时注入

- 镜像层不内置租户、链、协议或私有材料；声明的 init_assets：无。
- genesis、rollup、checkpoint、Hyperlane、Chainlink、合约或 subgraph 材料必须由平台受控对象存储、初始化资产和 CompositionSpec 注入并校验来源及 SHA-256；缺失时严格失败。
- 数据卷声明：无顶层 data_volumes 声明。

## Secret / KMS

- Secret 环境键：POSTGRES_PASSWORD, GRAPH_POI_ACCESS_TOKEN。
- 必需材料映射：[{"component":"infra","env":"POSTGRES_PASSWORD","secret_key":"PG_PASSWORD","source":"deploy/config/chaimir.env","runtime_value":"secret-or-kms"},{"component":"infra","env":"GRAPH_POI_ACCESS_TOKEN","secret_key":"GRAPH_POI_ACCESS_TOKEN","source":"deploy/config/secret.env","runtime_value":"secret-or-kms"}]
- Secret 只来自平台 Secret/KMS 运行时注入，不能写入 Dockerfile、manifest、README、镜像层或测试输出。
- 数据库密码由标准库转义后写入原生连接 URL，完整保留空白与特殊字符，不在错误中回显。
- 原生 URL 生成后清除 `POSTGRES_PASSWORD` 环境键；POI token 仅保留原生键的一份值。
- 数据库连接要求 TLS；证书由提供者经平台受控来源注入。

## 持久化与隔离边界

- `volume_source: self` 将组件的 runtime-state 与链账本隔离。索引数据写入所绑定的 PostgreSQL，内容经 IPFS 获取；不挂载链节点的私有状态或工作区。

## 安全边界

- 安全声明：{"run_as_non_root":true,"read_only_root_filesystem":false,"allow_privilege_escalation":false,"privileged":false,"host_network":false,"automount_service_account_token":false,"drop_capabilities":["ALL"],"network_policy":"deny-all"}
- student_access：{"enabled":false}。
- 网络只允许 manifest binding/link 所需目标，默认拒绝；禁止宿主机路径、hostNetwork、ServiceAccount token 和越权写入。

## readiness / health

- 组件 infra readiness：{"type":"tcp","port":"graphql","period_seconds":2,"failure_threshold":30}。
- 镜像自检 graph-node-version：["graph-node","--version"]。

## 清理

- 验收后按 CompositionSpec 所属租户/实例和项目 owner 标签删除容器、Pod、Service、NetworkPolicy、临时卷、Secret 投影和临时材料；保留平台长期资源。
- 不使用 broad prune，不删除 kind-chaimir-cilium 节点、正式锁引用或其他项目资源。

## 验收条件

- 先按 digest lock 启动合法 CompositionSpec，核对 command/args、bindings、links、材料和 Secret 投影，再执行 manifest selftest。
- selftest 只证明镜像布局和声明入口正确；协议、链、存储、索引、签名或跨镜像能力必须追加真实 provider 联动、正向业务、错误输入拒绝、重启/持久化（若声明）和清理证据。
- 未执行或只完成探活的项目不得标记为业务通过。
- 使用真实 RPC、PostgreSQL 与 IPFS 部署 subgraph，查询 `index-status` 的进度及错误，触发链上事件并用 GraphQL 验证结果；另验证 RPC 网关提供者、重启恢复与未授权访问拒绝。
