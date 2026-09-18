# infra/paymaster

## 镜像职责

账户抽象代付 gas 教学服务镜像。

## 能力边界

- 支持 capability：erc4337-paymaster；服务只为正式 ERC-4337 v0.6 EntryPoint 和已部署的 VerifyingPaymaster 生成赞助数据。
- 明确不支持：manifest 未声明的能力、协议动作和跨镜像联动；入口和编排必须严格失败，不使用 generic fallback。
- conflicts：无；cardinality：one；placement：sandbox。

## 合法 runtime / provider

- 生态或适配范围：evm
- 必需 provider 能力：evm-json-rpc。
- provider 只能通过 manifest 能力和显式 CompositionSpec link 选择；多个候选必须显式指定，不能按名称猜测。

## bindings 与 links

- 必需 bindings：[{"name":"execution_chain","capability":"evm-json-rpc","source_component":"infra","endpoint":"rpc","protocol":"HTTP","required_at_start":true,"config_binding":"env:CHAIMIR_EVM_RPC_URL"}]。
- 必需 links：无；本镜像不声明跨组件 link。
- binding 的 endpoint、协议、配置键和启动前置以 manifest 为准；缺失、重复、错误能力或错误 provider 均拒绝编排。

## 端口与服务

- http：8080/TCP，暴露边界 internal，用途：Paymaster service API。
- CompositionSpec services：[{"name":"infra-paymaster-http","component":"infra","ports":[{"name":"http","port":8080,"target_port":"http","protocol":"TCP"}]}]
- 组件：infra。

## command / args 语义

- 镜像默认 CMD：["node", "/app/paymaster-service.mjs"]；manifest 未覆盖组件 command/args，使用该入口语义。

## 初始化材料与运行时注入

- 镜像层不内置租户、链、协议或私有材料；声明的 init_assets：无。
- genesis、rollup、checkpoint、Hyperlane、Chainlink、合约或 subgraph 材料必须由平台受控对象存储、初始化资产和 CompositionSpec 注入并校验来源及 SHA-256；缺失时严格失败。
- 数据卷声明：无顶层 data_volumes 声明。

## Secret / KMS

- Secret 环境键：CHAIMIR_PAYMASTER_PRIVATE_KEY, CHAIMIR_PAYMASTER_SERVICE_AUTH_SECRET。
- 必需材料映射：[{"component":"infra","env":"CHAIMIR_PAYMASTER_PRIVATE_KEY","secret_key":"CHAIMIR_PAYMASTER_PRIVATE_KEY","source":"deploy/config/chaimir.env","runtime_value":"secret-or-kms"},{"component":"infra","env":"CHAIMIR_PAYMASTER_SERVICE_AUTH_SECRET","secret_key":"CHAIMIR_PAYMASTER_SERVICE_AUTH_SECRET","source":"deploy/config/chaimir.env","runtime_value":"secret-or-kms"}]
- Secret 只来自平台 Secret/KMS 运行时注入，不能写入 Dockerfile、manifest、README、镜像层或测试输出。

## 持久化与隔离边界

- 持久化目录：由组件 mount_domains 决定；未声明持久化数据时不得自行落盘。
- 组件挂载与文件系统：[{"name":"infra","ephemeral_mounts":[{"name":"paymaster-tmp","mount_path":"/tmp"}],"read_only_root_filesystem":true,"mount_workspace":false}]
## 安全边界

- 安全声明：{"run_as_non_root":true,"read_only_root_filesystem":true,"allow_privilege_escalation":false,"privileged":false,"host_network":false,"automount_service_account_token":false,"drop_capabilities":["ALL"],"network_policy":"deny-all"}
- student_access：{"enabled":false}。
- 网络只允许 manifest binding/link 所需目标，默认拒绝；禁止宿主机路径、hostNetwork、ServiceAccount token 和越权写入。

## readiness / health

- 组件 infra readiness：{"type":"http","path":"/healthz","port":"http","period_seconds":2,"failure_threshold":30}。
- 镜像自检 node-syntax：["node","--check","/app/paymaster-service.mjs"]。
- 镜像自检 auth-required：["node","-e","const fs=require('fs'); const src=fs.readFileSync('/app/paymaster-service.mjs','utf8'); if(!src.includes('verifyServiceSignature') || !src.includes('validateUserOperation')) process.exit(1)"]。

- `/sponsor` 返回的 `userOperationHash` 已覆盖完整的 v0.6 `paymasterAndData`；其中时间字段按官方合约要求使用两个 ABI 编码的 `uint48`，后接 64 或 65 字节 sponsor signature。该接口成功不等于 UserOperation 已上链，必须继续由合法 bundler 提交并由 EntryPoint 执行。

## 清理

- 验收后按 CompositionSpec 所属租户/实例和项目 owner 标签删除容器、Pod、Service、NetworkPolicy、临时卷、Secret 投影和临时材料；保留平台长期资源。
- 不使用 broad prune，不删除 kind-chaimir-cilium 节点、正式锁引用或其他项目资源。

## 验收条件

- 先按 digest lock 启动合法 CompositionSpec，核对 command/args、bindings、links、材料和 Secret 投影，再执行 manifest selftest。
- selftest 只证明镜像布局和声明入口正确；协议、链、存储、索引、签名或跨镜像能力必须追加真实 provider 联动、正向业务、错误输入拒绝、重启/持久化（若声明）和清理证据。
- 未执行或只完成探活的项目不得标记为业务通过。
