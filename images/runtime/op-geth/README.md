# runtime/op-geth

## 镜像职责

Optimism OP Labs 官方 OP Geth 源码重建执行层镜像，并内置平台受控的教学链创世材料。

## 能力边界

- 支持 capability：chain-runtime, evm-json-rpc, evm-engine-api, op-geth-execution
- 明确不支持：manifest 未声明的能力、协议动作和跨镜像联动；入口和编排必须严格失败，不使用 generic fallback。
- conflicts：无；cardinality：one；placement：sandbox。

## 合法 runtime / provider

- 生态或适配范围：evm
- 必需 provider 能力：无。
- provider 只能通过 manifest 能力和显式 CompositionSpec link 选择；多个候选必须显式指定，不能按名称猜测。

## bindings 与 links

- 必需 bindings：无。
- 必需 links：无；本镜像不声明跨组件 link。
- binding 的 endpoint、协议、配置键和启动前置以 manifest 为准；缺失、重复、错误能力或错误 provider 均拒绝编排。

## 端口与服务

- rpc：8545/TCP，暴露边界 internal，用途：L2 JSON-RPC。
- engine：8551/TCP，暴露边界 internal，用途：authenticated Engine API for op-node。
- 组件：无 resource_spec 组件。

## command / args 语义

- 镜像默认 ENTRYPOINT：["/usr/local/bin/chaimir-op-geth-entrypoint"]；manifest 未覆盖组件 command/args，使用该入口语义。入口先用 `geth init --state.scheme hash` 初始化内置创世，再以统一的 `--syncmode full --gcmode archive --state.scheme hash` 启动执行层，最后启动 JSON-RPC 和 Engine API；完整执行模式接收 Beacon Engine payload，哈希状态方案与归档模式共同满足索引器对事件区块历史 `eth_call` 的生产要求，所有合法 CompositionSpec 均使用该语义。

## 初始化材料与运行时注入

- 镜像内置默认创世文件：`/opt/chaimir/genesis/op-geth.json`，由平台候选构建流程固定生成并在入口校验 SHA-256；它是本项目受控教学链的执行层创世，不是公网链或租户私有材料。
- OP-Stack 合法 CompositionSpec 使用同一平台受控材料归档，将 `genesis.json` 注入 `runtime-state/op-geth/genesis.json`，并显式设置 `OP_GETH_GENESIS_PATH`；这不是环境分支，也不是宿主机路径覆盖，材料必须来自 init-assets/Secret-KMS 编排链路并经过来源及 SHA-256 校验。
- 运行时工作目录：`/runtime-state/op-geth`，用于链状态、JWT 运行期材料和节点数据；由 CompositionSpec 的 `runtime-state` 卷持久化。
- 受控 OP-Stack 材料生成会将平台统一部署账户 `0xfdaf0ea969c98d0a8410d76bac4ac8838d13c9d4` 写入 L2 allocs，供合法的真实合约流程使用；地址不是密钥，私钥始终只由平台 Secret/KMS 提供，归档不包含私钥。

## Secret / KMS

- Secret 环境键：OP_GETH_JWT_SECRET。
- 必需材料映射：`OP_GETH_JWT_SECRET` 来自平台 Secret/KMS；默认创世使用镜像内 `/opt/chaimir/genesis/op-geth.json`，OP-Stack CompositionSpec 则使用同一受控归档注入的 `runtime-state/op-geth/genesis.json`，不得使用宿主机路径或公网材料。
- Secret 只来自平台 Secret/KMS 运行时注入，不能写入 Dockerfile、manifest、README、镜像层或测试输出。

## 持久化与隔离边界

- 持久化目录：`/runtime-state/op-geth`。当 Beacon finalized checkpoint 引用已生成的 L1 execution head 时，组合初始化还必须从同一受控归档恢复 `l1-execution-state.tar`；该归档只包含 execution 数据库和 genesis 一致性标记，不包含 JWT 或账户私钥。
- 组件挂载与文件系统：运行时组件挂载 `runtime-state`，入口只在该目录写入节点状态；根文件系统因 OP Geth 运行时写入限制保持可写，但安全例外只允许 `runtime-state`。
## 安全边界

- 安全声明：{"run_as_non_root":true,"read_only_root_filesystem":false,"allow_privilege_escalation":false,"privileged":false,"host_network":false,"automount_service_account_token":false,"drop_capabilities":["ALL"],"network_policy":"deny-all"}
- student_access：{"enabled":false}。
- 网络只允许 manifest binding/link 所需目标，默认拒绝；禁止宿主机路径、hostNetwork、ServiceAccount token 和越权写入。

## readiness / health

- 未声明组件 readiness probe；容器运行不等于就绪或业务成功。真实验收必须另行验证 JSON-RPC、Engine API、JWT 和下游 OP Stack/Beacon 联动。
- 镜像自检 op-geth-version：["geth","version"]。

## 清理

- 验收后按 CompositionSpec 所属租户/实例和项目 owner 标签删除容器、Pod、Service、NetworkPolicy、临时卷、Secret 投影和临时材料；保留平台长期资源。
- 不使用 broad prune，不删除 kind-chaimir-cilium 节点、正式锁引用或其他项目资源。

## 验收条件

- 先按 digest lock 启动合法 CompositionSpec，核对 command/args、bindings、links、材料和 Secret 投影，再执行 manifest selftest。
- selftest 只证明镜像布局和声明入口正确；协议、链、存储、索引、签名或跨镜像能力必须追加真实 provider 联动、正向业务、错误输入拒绝、重启/持久化（若声明）和清理证据。
- 未执行或只完成探活的项目不得标记为业务通过。
