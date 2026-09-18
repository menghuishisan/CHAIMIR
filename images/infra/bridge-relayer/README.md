# infra/bridge-relayer

## 镜像职责

基于 Hyperlane 官方 agent 的跨链桥中继教学服务镜像。

## 能力边界

- 支持 capability：cross-chain-relayer
- 明确不支持：manifest 未声明的能力、协议动作和跨镜像联动；入口和编排必须严格失败，不使用 generic fallback。
- conflicts：无；cardinality：one；placement：sandbox。

## 合法 runtime / provider

- 生态或适配范围：evm
- 必需 provider 能力：evm-json-rpc。
- provider 只能通过 manifest 能力和显式 CompositionSpec link 选择；多个候选必须显式指定，不能按名称猜测。

## bindings 与 links

- 必需 bindings：[{"name":"source_chain","capability":"evm-json-rpc","source_component":"infra","source_endpoint":"component","endpoint":"rpc","protocol":"HTTP","required_at_start":true,"config_binding":"env:HYP_SOURCE_RPC"},{"name":"destination_chain","capability":"evm-json-rpc","source_component":"infra","source_endpoint":"component","endpoint":"rpc","protocol":"HTTP","required_at_start":true,"config_binding":"env:HYP_DESTINATION_RPC"}]。
- 必需 links：无；本镜像不声明跨组件 link。
- binding 的 endpoint、协议、配置键和启动前置以 manifest 为准；缺失、重复、错误能力或错误 provider 均拒绝编排。

## 端口与服务

- 无对外监听端口；不得把命令工具伪装成 HTTP 服务。
- 组件：infra。

## command / args 语义

- 镜像默认 ENTRYPOINT：["/usr/local/bin/chaimir-bridge-relayer-entrypoint"]；manifest 未覆盖组件 command/args，使用该入口语义。

## 初始化材料与运行时注入

- 镜像层不内置租户、链、协议或私有材料；声明的 init_assets：无。
- genesis、rollup、checkpoint、Hyperlane、Chainlink、合约或 subgraph 材料必须由平台受控对象存储、初始化资产和 CompositionSpec 注入并校验来源及 SHA-256；缺失时严格失败。
- 数据卷声明：无顶层 data_volumes 声明。

## Secret / KMS

- Secret 环境键：HYP_DEFAULTSIGNER_KEY。
- 必需材料映射：[{"component":"infra","env":"HYP_DEFAULTSIGNER_KEY","secret_key":"HYP_DEFAULTSIGNER_KEY","source":"deploy/config/chaimir.env","runtime_value":"secret-or-kms"}]
- Secret 只来自平台 Secret/KMS 运行时注入，不能写入 Dockerfile、manifest、README、镜像层或测试输出。

## 持久化与隔离边界

- 持久化目录：`/runtime-state/hyperlane/db` 仅保存中继数据库状态；含 signer Secret 的合并配置只写入临时挂载 `/tmp/chaimir-hyperlane/merged-config.json`，权限为 `0600`，Pod 生命周期结束后销毁。
- 组件挂载与文件系统：[{"name":"infra","mount_domains":["runtime-state"],"writable_mount_domains":["runtime-state"],"ephemeral_mounts":[{"name":"relayer-tmp","mount_path":"/tmp"}],"read_only_root_filesystem":true,"mount_workspace":false}]
## 安全边界

- 安全声明：{"run_as_non_root":true,"read_only_root_filesystem":true,"allow_privilege_escalation":false,"privileged":false,"host_network":false,"automount_service_account_token":false,"drop_capabilities":["ALL"],"network_policy":"deny-all"}
- student_access：{"enabled":false}。
- 网络只允许 manifest binding/link 所需目标，默认拒绝；禁止宿主机路径、hostNetwork、ServiceAccount token 和越权写入。
- 持久化例外仅用于 relayer 数据库；签名私钥只存在 Secret 投影和临时配置文件，不写入 `runtime-state`。

## source/destination 配置校验

- `HYP_RELAYCHAINS` 必须恰好包含两个不同且无空白的链名；source 与 destination 的 `chainId`、`domainId` 必须分别为正整数，domain 不得相同。
- base config 必须是无重复 JSON 对象键的完整文档；必需字段缺失、空值、首尾空白或地址格式非法时，配置入口立即失败。
- 每个链的 `chainId` 必须与该链显式 RPC binding 返回的 `eth_chainId` 一致；Mailbox `localDomain()` 必须与 `domainId` 一致。
- mailbox、interchainGasPaymaster、validatorAnnounce 和 merkleTreeHook 必须是非零 EVM 地址，且在对应 runtime 上 `eth_getCode` 返回已部署合约字节码。
- RPC 检查使用显式 source/destination binding；任一查询错误、错链、错域或未部署地址均严格失败，不启动 relayer。

## readiness / health

- 未声明组件 readiness probe；容器运行不等于就绪或业务成功。
- 镜像自检 hyperlane-relayer-binary：["test","-x","/app/relayer"]。
- 镜像自检 config-helper：["test","-x","/usr/local/bin/chaimir-hyperlane-config"]。
- 镜像自检 entrypoint-syntax：["sh","-n","/usr/local/bin/chaimir-bridge-relayer-entrypoint"]。

## 清理

- 验收后按 CompositionSpec 所属租户/实例和项目 owner 标签删除容器、Pod、Service、NetworkPolicy、临时卷、Secret 投影和临时材料；保留平台长期资源。
- 不使用 broad prune，不删除 kind-chaimir-cilium 节点、正式锁引用或其他项目资源。

## 验收条件

- 先按 digest lock 启动合法 CompositionSpec，核对 command/args、bindings、links、材料和 Secret 投影，再执行 manifest selftest。
- selftest 只证明镜像布局和声明入口正确；协议、链、存储、索引、签名或跨镜像能力必须追加真实 provider 联动、正向业务、错误输入拒绝、重启/持久化（若声明）和清理证据。
- 未执行或只完成探活的项目不得标记为业务通过。
