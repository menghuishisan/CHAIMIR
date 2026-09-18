# infra/op-stack

## 镜像职责

Optimism 风格 L2 教学编排镜像。

## 能力边界

- 支持 capability：l2-stack；当前入口提供 rollup/verifier，不声明 sequencer 能力。
- 明确不支持：manifest 未声明的能力、协议动作和跨镜像联动；入口和编排必须严格失败，不使用 generic fallback。
- conflicts：无；cardinality：one；placement：sandbox。

## 合法 runtime / provider

- 生态或适配范围：evm
- 必需 provider 能力：evm-json-rpc, evm-engine-api, ethereum-beacon-api。
- provider 只能通过 manifest 能力和显式 CompositionSpec link 选择；多个候选必须显式指定，不能按名称猜测。
- P2P：当前 manifest 不声明 P2P 能力、端口或 peer 材料，因此组件配置固定为 `OP_NODE_P2P_DISABLE=true`；不能使用上游默认公网 bootnode，也不能把未声明的 P2P 行为写成支持能力。
- 受控写入链参数：CompositionSpec 可显式设置 `sequencer_enabled` 和 `sequencer_l1_confs`，分别映射为 `OP_NODE_SEQUENCER_ENABLED`、`OP_NODE_SEQUENCER_L1_CONFS`；只有同时提供受控 L1 execution、L2 Engine、Beacon、rollup 和链配置材料的组合才允许开启。未显式设置时沿用官方默认关闭，不把 verifier 组件误写成 sequencer 能力。

## bindings 与 links

- 必需 bindings：[{"name":"l1_execution","capability":"evm-json-rpc","source_component":"infra","endpoint":"rpc","protocol":"HTTP","required_at_start":true,"config_binding":"env:OP_NODE_L1_ETH_RPC"},{"name":"l2_engine","capability":"evm-engine-api","source_component":"infra","endpoint":"engine","protocol":"HTTP","required_at_start":true,"config_binding":"env:OP_NODE_L2_ENGINE_RPC"},{"name":"l1_beacon","capability":"ethereum-beacon-api","source_component":"infra","endpoint":"http","protocol":"HTTP","required_at_start":true,"config_binding":"env:OP_NODE_L1_BEACON"}]；rollup_config、l1_chain_config 和 l1_beacon 也必须由同一 CompositionSpec 显式提供。
- 必需 links：无；本镜像不声明跨组件 link。
- binding 的 endpoint、协议、配置键和启动前置以 manifest 为准；缺失、重复、错误能力或错误 provider 均拒绝编排。

## 端口与服务

- http：8545/TCP，暴露边界 internal，用途：OP Stack RPC。
- CompositionSpec services：[{"name":"infra-op-stack-http","component":"infra","ports":[{"name":"http","port":8545,"target_port":"http","protocol":"TCP"}]}]
- 组件：infra。

## command / args 语义

- 镜像默认 ENTRYPOINT：["/usr/local/bin/chaimir-op-node-entrypoint"]；manifest 未覆盖组件 command/args，使用该入口语义。
- 构建阶段使用固定的官方 mikefarah/yq v4.44.3 解析 superchain-registry TOML；版本和 SHA-256 由 manifest 固定，不能替换为 Debian 的 Python yq 包。

## 初始化材料与运行时注入

- 镜像层不内置租户、链、协议或私有材料；声明的 init_assets：无。
- L1 genesis、L2 genesis、rollup、checkpoint、与 finalized checkpoint 对齐的 `l1-execution-state.tar`、Hyperlane、Chainlink、合约或 subgraph 材料必须由平台受控对象存储、初始化资产和 CompositionSpec 注入并校验来源及 SHA-256；本镜像不内置创世或 rollup 材料，缺失时严格失败。
- OP_NODE_ROLLUP_L1_CHAIN_CONFIG 必须指向与 OP_NODE_ROLLUP_CONFIG 同一受控材料归档中的 L1 execution genesis，且 chainId 必须有效。
- 数据卷声明：无顶层 data_volumes 声明。

## Secret / KMS

- Secret 环境键：OP_GETH_JWT_SECRET。
- 必需材料映射：[{"component":"infra","env":"OP_GETH_JWT_SECRET","secret_key":"OP_GETH_JWT_SECRET","source":"deploy/config/chaimir.env","runtime_value":"secret-or-kms"}]
- Secret 只来自平台 Secret/KMS 运行时注入，不能写入 Dockerfile、manifest、README、镜像层或测试输出。

## 持久化与隔离边界

- 持久化目录：由组件 mount_domains 决定；未声明持久化数据时不得自行落盘。
- 组件挂载与文件系统：[{"name":"infra","mount_domains":["runtime-state"],"writable_mount_domains":["runtime-state"],"read_only_root_filesystem":true,"mount_workspace":false}]
## 安全边界

- 安全声明：{"run_as_non_root":true,"read_only_root_filesystem":true,"allow_privilege_escalation":false,"privileged":false,"host_network":false,"automount_service_account_token":false,"drop_capabilities":["ALL"],"network_policy":"deny-all"}
- student_access：{"enabled":false}。
- 网络只允许 manifest binding/link 所需目标，默认拒绝；禁止宿主机路径、hostNetwork、ServiceAccount token 和越权写入。

## readiness / health

- 组件 infra readiness：{"type":"tcp","port":"http","period_seconds":2,"failure_threshold":30}。
- 镜像自检 op-node-version：["op-node","--version"]。

## 清理

- 验收后按 CompositionSpec 所属租户/实例和项目 owner 标签删除容器、Pod、Service、NetworkPolicy、临时卷、Secret 投影和临时材料；保留平台长期资源。
- 不使用 broad prune，不删除 kind-chaimir-cilium 节点、正式锁引用或其他项目资源。

## 验收条件

- 先按 digest lock 启动合法 CompositionSpec，核对 command/args、bindings、links、材料和 Secret 投影，再执行 manifest selftest。
- selftest 只证明镜像布局和声明入口正确；协议、链、存储、索引、签名或跨镜像能力必须追加真实 provider 联动、正向业务、错误输入拒绝、重启/持久化（若声明）和清理证据。
- 未执行或只完成探活的项目不得标记为业务通过。
