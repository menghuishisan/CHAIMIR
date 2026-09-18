# infra/chainlink-node

## 镜像职责

Chainlink 预言机节点镜像。

## 能力边界

- 支持 capability：oracle-service, chainlink-node
- 明确不支持：manifest 未声明的能力、协议动作和跨镜像联动；入口和编排必须严格失败，不使用 generic fallback。
- conflicts：无；cardinality：one；placement：sandbox。

## 合法 runtime / provider

- 生态或适配范围：evm
- 必需 provider 能力：evm-json-rpc, postgres-compatible。
- 必需非敏感配置：`gas_price_max_wei`、`vrf_gas_limit`，分别由统一 `deploy/config/chaimir.env` 的 `CHAINLINK_GAS_PRICE_MAX_WEI`、`CHAINLINK_VRF_GAS_LIMIT` 通过 CompositionSpec 参数注入；前者限制 EVM gas 价格，后者覆盖 VRF fulfill 的 gas 估算上限，均禁止依赖不适用的官方默认值。
- provider 只能通过 manifest 能力和显式 CompositionSpec link 选择；多个候选必须显式指定，不能按名称猜测。

## bindings 与 links

- 必需 bindings：[{"name":"execution_chain","capability":"evm-json-rpc","source_component":"infra","endpoint":"rpc","protocol":"HTTP","required_at_start":true,"config_binding":"env:ETH_URL"},{"name":"database","capability":"postgres-compatible","source_component":"infra","endpoint":"postgres","protocol":"TCP","required_at_start":true,"config_binding":"env:CHAIMIR_CHAINLINK_DATABASE_ADDRESS"}]；数据库 URL 由入口固定生成 `sslmode=require`，与平台 PostgreSQL TLS 契约一致。
- 必需 links：无；本镜像不声明跨组件 link。
- binding 的 endpoint、协议、配置键和启动前置以 manifest 为准；缺失、重复、错误能力或错误 provider 均拒绝编排。

## 端口与服务

- https：6688/TCP，暴露边界 internal，用途：Chainlink node HTTPS API；HTTP 明文端口关闭。
- p2p：6690/TCP，暴露边界 internal，用途：Chainlink OCR2/P2P v2 节点通信；不向学生或公网暴露。
- CompositionSpec services：[{"name":"infra-chainlink-node-https","component":"infra","ports":[{"name":"https","port":6688,"target_port":"https","protocol":"TCP"}]},{"name":"infra-chainlink-node-p2p","component":"infra","ports":[{"name":"p2p","port":6690,"target_port":"p2p","protocol":"TCP"}]}]
- 组件：infra。

## command / args 语义

- 组件 infra：command=["/usr/local/bin/chaimir-chainlink-entrypoint"]；args=无。编排器把 command 首 token 作为 entrypoint，其余 command token 作为参数；manifest args 映射到 Container.Args。入口把 `CHAINLINK_GAS_PRICE_MAX_WEI`、`CHAINLINK_VRF_GAS_LIMIT` 写入官方 Chainlink gas estimator 配置，并统一启用 FeedsManager、OCR2 与 P2P v2；6690 只承担内部节点通信。

## 初始化材料与运行时注入

- 镜像层不内置租户、链、协议或私有材料；声明的 init_assets：无。
- genesis、rollup、checkpoint、Hyperlane、Chainlink、合约或 subgraph 材料必须由平台受控对象存储、初始化资产和 CompositionSpec 注入并校验来源及 SHA-256；缺失时严格失败。
- 数据卷声明：无顶层 data_volumes 声明。

## Secret / KMS

- Secret 环境键：POSTGRES_PASSWORD, CHAINLINK_KEYSTORE_PASSWORD, CHAINLINK_API_EMAIL, CHAINLINK_API_PASSWORD, CHAINLINK_TLS_CERT_B64, CHAINLINK_TLS_KEY_B64。
- 必需材料映射：上述键均由 `deploy/config/chaimir.env` 对应 Secret/KMS 字段注入；TLS 证书和私钥以 Base64 Secret 值注入后仅写入 runtime-state，绝不进入镜像层。Chainlink EVM oracle 钱包地址和余额属于组合生成的运行时材料，由受控 EVM 账户注资，不能写入镜像或仓库。
- Secret 只来自平台 Secret/KMS 运行时注入，不能写入 Dockerfile、manifest、README、镜像层或测试输出；PostgreSQL 服务端证书和私钥由 `middleware/postgres` 的受控 Secret 投影，Chainlink 只通过 TLS 数据库连接消费，不复制数据库私钥。

## 持久化与隔离边界

- 持久化目录：由组件 mount_domains 决定；未声明持久化数据时不得自行落盘。
- 组件挂载与文件系统：[{"name":"infra","mount_domains":["runtime-state"],"writable_mount_domains":["runtime-state"],"read_only_root_filesystem":false,"mount_workspace":false}]
## 安全边界

- 安全声明：{"run_as_non_root":true,"read_only_root_filesystem":false,"allow_privilege_escalation":false,"privileged":false,"host_network":false,"automount_service_account_token":false,"drop_capabilities":["ALL"],"network_policy":"deny-all"}
- student_access：{"enabled":false}。
- 网络只允许 manifest binding/link 所需目标，默认拒绝；禁止宿主机路径、hostNetwork、ServiceAccount token 和越权写入。

## readiness / health

- 组件 infra readiness：{"type":"exec","command":["/usr/local/bin/chaimir-chainlink-healthcheck"],"period_seconds":2,"failure_threshold":30}；探针读取运行期 TLS 证书，以受控 CA、固定主机映射和有界超时访问 `https://chainlink-node:6688/readyz`。K8s 不使用明文 HTTP 探针或只代表端口已监听的 TCP 探针。
- 镜像 healthcheck 与 readiness 使用同一项目脚本；没有 TLS 材料、证书不可读、HTTPS 服务未 ready 或证书主机不匹配时均失败。
- 镜像自检 chainlink-version：["chainlink","--version"]。

## 清理

- 验收后按 CompositionSpec 所属租户/实例和项目 owner 标签删除容器、Pod、Service、NetworkPolicy、临时卷、Secret 投影和临时材料；保留平台长期资源。
- 不使用 broad prune，不删除 kind-chaimir-cilium 节点、正式锁引用或其他项目资源。

## 验收条件

- 先按 digest lock 启动合法 CompositionSpec，核对 command/args、bindings、links、材料和 Secret 投影，再执行 manifest selftest。
- selftest 只证明镜像布局和声明入口正确；协议、链、存储、索引、签名或跨镜像能力必须追加真实 provider 联动、正向业务、错误输入拒绝、重启/持久化（若声明）和清理证据。
- 未执行或只完成探活的项目不得标记为业务通过。
