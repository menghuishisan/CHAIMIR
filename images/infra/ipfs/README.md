# infra/ipfs

## 镜像职责

IPFS/Kubo 内容寻址存储节点镜像。

## 能力边界

- 支持 capability：decentralized-storage, ipfs-node, ipfs-http-api
- 明确不支持：manifest 未声明的能力、协议动作和跨镜像联动；入口和编排必须严格失败，不使用 generic fallback。
- conflicts：无；cardinality：one；placement：sandbox。

## 合法 runtime / provider

- 生态或适配范围：evm, fabric, fisco, solana, aptos, sui, cosmos, substrate, near, bitcoin, edu
- 必需 provider 能力：无。
- provider 只能通过 manifest 能力和显式 CompositionSpec link 选择；多个候选必须显式指定，不能按名称猜测。

## bindings 与 links

- 必需 bindings：无。
- 必需 links：无；本镜像不声明跨组件 link。
- binding 的 endpoint、协议、配置键和启动前置以 manifest 为准；缺失、重复、错误能力或错误 provider 均拒绝编排。

## 端口与服务

- api：5001/TCP，暴露边界 internal，用途：IPFS API。
- gateway：8080/TCP，暴露边界 proxy，用途：IPFS gateway。
- CompositionSpec services：[{"name":"infra-ipfs-api","component":"infra","ports":[{"name":"api","port":5001,"target_port":"api","protocol":"TCP"}]},{"name":"infra-ipfs-gateway","component":"infra","ports":[{"name":"gateway","port":8080,"target_port":"gateway","protocol":"TCP"}]}]
- 组件：infra。

## command / args 语义

- 镜像默认 ENTRYPOINT：["/usr/local/bin/chaimir-ipfs-entrypoint"]；manifest 未覆盖组件 command/args，使用该入口语义。
- 镜像默认 CMD：["daemon", "--migrate=true"]；manifest 未覆盖组件 command/args，使用该入口语义。

## 初始化材料与运行时注入

- 镜像层不内置租户、链、协议或私有材料；声明的 init_assets：无。首次启动由镜像内固定 Kubo 源码编译出的初始化器生成实例本地 node identity，写入 runtime-state 后交给官方 `ipfs init`，初始化临时文件随即删除；该身份不是平台 Secret/KMS 材料。
- genesis、rollup、checkpoint、Hyperlane、Chainlink、合约或 subgraph 材料必须由平台受控对象存储、初始化资产和 CompositionSpec 注入并校验来源及 SHA-256；缺失时严格失败。
- 数据卷声明：无顶层 data_volumes 声明。

## Secret / KMS

- Secret 环境键：无。
- 必需材料映射：无；平台 Secret/KMS 不向本镜像注入业务密钥，实例 node identity 仅按上述受控初始化流程生成并持久化在 runtime-state。
- Secret 只来自平台 Secret/KMS 运行时注入，不能写入 Dockerfile、manifest、README、镜像层或测试输出。

## 持久化与隔离边界

- 持久化目录：由组件 mount_domains 决定；未声明持久化数据时不得自行落盘。
- 组件挂载与文件系统：[{"name":"infra","mount_domains":["runtime-state"],"writable_mount_domains":["runtime-state"],"read_only_root_filesystem":false,"mount_workspace":false}]
## 安全边界

- 安全声明：{"run_as_non_root":true,"read_only_root_filesystem":false,"allow_privilege_escalation":false,"privileged":false,"host_network":false,"automount_service_account_token":false,"drop_capabilities":["ALL"],"network_policy":"deny-all"}
- student_access：{"enabled":false}。
- 网络只允许 manifest binding/link 所需目标，默认拒绝；禁止宿主机路径、hostNetwork、ServiceAccount token 和越权写入。

## readiness / health

- 组件 infra readiness：{"type":"tcp","port":"api","period_seconds":2,"failure_threshold":30}。
- 镜像自检 ipfs-version：["ipfs","version"]。

## 清理

- 验收后按 CompositionSpec 所属租户/实例和项目 owner 标签删除容器、Pod、Service、NetworkPolicy、临时卷、Secret 投影和临时材料；保留平台长期资源。
- 不使用 broad prune，不删除 kind-chaimir-cilium 节点、正式锁引用或其他项目资源。

## 验收条件

- 先按 digest lock 启动合法 CompositionSpec，核对 command/args、bindings、links、材料和 Secret 投影，再执行 manifest selftest。
- selftest 只证明镜像布局和声明入口正确；协议、链、存储、索引、签名或跨镜像能力必须追加真实 provider 联动、正向业务、错误输入拒绝、重启/持久化（若声明）和清理证据。
- 未执行或只完成探活的项目不得标记为业务通过。
