# runtime/evm-foundry

## 镜像职责

Foundry/Anvil EVM 运行时镜像。

Ubuntu Jammy 运行层的 `openssl`、`libssl3` 固定为官方安全仓库提供的 `3.0.2-0ubuntu1.30`,Dockerfile 与 manifest 保持同一版本。更新仍须经过完整扫描与签名门禁,不取消版本固定或绕过漏洞检查。

## 能力边界

- 支持 capability：chain-runtime, evm-json-rpc, foundry-toolchain, fork-capable, chain-capability-helper
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

- rpc：8545/TCP，暴露边界 proxy，用途：EVM JSON-RPC。
- 组件：无 resource_spec 组件。

## command / args 语义

- 默认 command/args 由镜像入口启动 Anvil；CompositionSpec 显式传入的 args 会原样追加到 Anvil 参数。需要改变链标识、区块时间等运行时契约时，必须在合法组合中明确声明，不能由入口猜测或回退。

- 镜像默认 ENTRYPOINT：["/usr/local/bin/chaimir-foundry-entrypoint"]；manifest 未覆盖组件 command/args，使用该入口语义。
- runtime 原生动作：helper=/usr/local/bin/chaimir-foundry-adapter；profile=evm-contract；actions={"deploy":["/usr/local/bin/chaimir-foundry-adapter","deploy"],"tx":["/usr/local/bin/chaimir-foundry-adapter","tx"],"query":["/usr/local/bin/chaimir-foundry-adapter","query"]}。query 只支持 manifest 约定的 latest、blockNumber、chainId、balance:<address>、code:<address>、counter:last；未声明目标严格失败。deploy 的 `contract_ref`、tx 的 `target_ref` 和 deploy 的 `constructor_args:[{"type":"address","target_ref":"..."}]` 用于同一沙箱内的显式步骤绑定；构造参数只允许从已部署的地址引用，不能使用动态文本替换。deploy 只有显式 `compile:true` 时才在 runtime 内按 `project_dir`、`artifact` 离线编译，未显式开启时必须读取已经存在的 artifact；能力自检载荷不固化 `from`，由运行时依据正式注入的 `CHAIMIR_EVM_DEPLOYER_MNEMONIC` 派生首个解锁账户。tx payload 的 `expect_revert` 必须是布尔值；只有显式为 `true` 时才允许 unlocked 账户通过受控 JSON-RPC 发送并返回真实链上回滚回执，同时必须提供正整数 `gas_limit`；未声明或为 `false` 的交易仍按默认 gas 预估执行。

## 初始化材料与运行时注入

- 镜像层不内置租户、私有凭据或业务题目材料；声明的 init_assets：无。
- manifest 的 `genesis_baked: true` 只表示镜像内置项目受控的确定性平台自检 genesis，入口首次启动时将其复制到 `/runtime-state`，供本镜像的自检链使用；`genesis_ref` 只能引用这份已声明的内置自检材料，不能把它解释成外部业务链材料。
- 外部 genesis、rollup、checkpoint、Hyperlane、Chainlink、合约或 subgraph 材料不在镜像内，必须由平台受控对象存储、初始化资产和 CompositionSpec 注入，并校验来源及 SHA-256；缺失时严格失败。
- 数据卷声明：无顶层 data_volumes 声明。

## Secret / KMS

- Secret 环境键：CHAIMIR_EVM_DEPLOYER_MNEMONIC；部署和交易适配器在运行时从同一助记词派生首个解锁账户，不要求业务层复制私钥。
- 必需材料映射：[{"component":"runtime","env":"CHAIMIR_EVM_DEPLOYER_MNEMONIC","secret_key":"CHAIMIR_EVM_DEPLOYER_MNEMONIC","source":"deploy/config/chaimir.env","runtime_value":"secret-or-kms"}]
- Secret 只来自平台 Secret/KMS 运行时注入，不能写入 Dockerfile、manifest、README、镜像层或测试输出。

## 持久化与隔离边界

- 持久化目录：由组件 mount_domains 决定；未声明持久化数据时不得自行落盘。
- 组件挂载与文件系统：runtime 主容器显式挂载 workspace（用于按题面要求在 runtime 内离线编译并生成构建产物）、runtime-state 和 public-assets；学生入口仍由独立的 tool/workspace sidecar 挂载 workspace，runtime 主容器不提供学生 shell。
## 安全边界

- 安全声明：{"run_as_non_root":true,"read_only_root_filesystem":true,"allow_privilege_escalation":false,"privileged":false,"host_network":false,"automount_service_account_token":false,"drop_capabilities":["ALL"],"network_policy":"deny-all"}
- student_access：{"enabled":false,"shell":"none","allowed_mounts":[]}。
- 网络只允许 manifest binding/link 所需目标，默认拒绝；禁止宿主机路径、hostNetwork、ServiceAccount token 和越权写入。

## readiness / health

- 未声明组件 readiness probe；容器运行不等于就绪或业务成功。
- 镜像自检 foundry-tools：["sh","-lc","forge --version && anvil --version && cast --version"]。
- 镜像自检 solidity-compilers：["sh","-lc","set -eu; for version in 0.8.15 0.8.19 0.8.20 0.8.24 0.8.25 0.8.26; do case \"$(/usr/local/bin/solc-${version} --version)\" in *\"Version: ${version}\"*) ;; *) exit 1 ;; esac; done"]。

## 清理

- 验收后按 CompositionSpec 所属租户/实例和项目 owner 标签删除容器、Pod、Service、NetworkPolicy、临时卷、Secret 投影和临时材料；保留平台长期资源。
- 不使用 broad prune，不删除 kind-chaimir-cilium 节点、正式锁引用或其他项目资源。

## 验收条件

- 先按 digest lock 启动合法 CompositionSpec，核对 command/args、bindings、links、材料和 Secret 投影，再执行 manifest selftest。
- selftest 只证明镜像布局和声明入口正确；协议、链、存储、索引、签名或跨镜像能力必须追加真实 provider 联动、正向业务、错误输入拒绝、重启/持久化（若声明）和清理证据。
- 未执行或只完成探活的项目不得标记为业务通过。
