# tool/db-viewer

## 镜像职责

状态数据库可视化工具镜像。

## 能力边界

- 支持 capability：database-viewer, state-inspection
- 明确不支持：manifest 未声明的能力、协议动作和跨镜像联动；入口和编排必须严格失败，不使用 generic fallback。
- conflicts：无；cardinality：one；placement：sandbox。

## 合法 runtime / provider

- 生态或适配范围：fabric, evm
- 必需 provider 能力：postgres-compatible。
- provider 只能通过 manifest 能力和显式 CompositionSpec link 选择；多个候选必须显式指定，不能按名称猜测。

## bindings 与 links

- 必需 bindings：[{"name":"database","capability":"postgres-compatible","source_component":"web","endpoint":"postgres","protocol":"TCP","required_at_start":true,"config_binding":"env:CHAIMIR_DATABASE_ADDRESS"}]。
- 必需 links：无；本镜像不声明跨组件 link。
- binding 的 endpoint、协议、配置键和启动前置以 manifest 为准；缺失、重复、错误能力或错误 provider 均拒绝编排。

## 端口与服务

- http：8081/TCP，暴露边界 proxy，用途：Database viewer web UI。
- CompositionSpec services：[{"name":"tool-db-viewer-web","component":"web","ports":[{"name":"http","port":8081,"target_port":"http","protocol":"TCP"}]}]
- 组件：web。

## command / args 语义

- 组件 web：command=["/usr/local/bin/chaimir-pgweb-entrypoint"]；args=无。编排器把 command 首 token 作为 entrypoint，其余 command token 作为参数；manifest args 映射到 Container.Args。

## 初始化材料与运行时注入

- 镜像层不内置租户、链、协议或私有材料；声明的 init_assets：无。
- genesis、rollup、checkpoint、Hyperlane、Chainlink、合约或 subgraph 材料必须由平台受控对象存储、初始化资产和 CompositionSpec 注入并校验来源及 SHA-256；缺失时严格失败。
- 数据卷声明：无顶层 data_volumes 声明。

## Secret / KMS

- Secret 环境键：POSTGRES_PASSWORD。
- 必需材料映射：[{"component":"web","env":"POSTGRES_PASSWORD","secret_key":"PG_PASSWORD","source":"deploy/config/chaimir.env","runtime_value":"secret-or-kms"}]
- Secret 只来自平台 Secret/KMS 运行时注入，不能写入 Dockerfile、manifest、README、镜像层或测试输出。

## 持久化与隔离边界

- 持久化目录：由组件 mount_domains 决定；未声明持久化数据时不得自行落盘。
- 组件挂载与文件系统：[{"name":"web","read_only_root_filesystem":false,"mount_workspace":false}]
## 安全边界

- 安全声明：{"run_as_non_root":true,"read_only_root_filesystem":false,"allow_privilege_escalation":false,"privileged":false,"host_network":false,"automount_service_account_token":false,"drop_capabilities":["ALL"],"network_policy":"deny-all"}
- student_access：{"enabled":true,"shell":"restricted","allowed_mounts":["public-assets"]}。
- 网络只允许 manifest binding/link 所需目标，默认拒绝；禁止宿主机路径、hostNetwork、ServiceAccount token 和越权写入。

## readiness / health

- 组件 web readiness：{"type":"http","path":"/","port":"http","period_seconds":2,"failure_threshold":30}。
- 镜像自检 pgweb-version：["pgweb","--version"]。

## 清理

- 验收后按 CompositionSpec 所属租户/实例和项目 owner 标签删除容器、Pod、Service、NetworkPolicy、临时卷、Secret 投影和临时材料；保留平台长期资源。
- 不使用 broad prune，不删除 kind-chaimir-cilium 节点、正式锁引用或其他项目资源。

## 验收条件

- 先按 digest lock 启动合法 CompositionSpec，核对 command/args、bindings、links、材料和 Secret 投影，再执行 manifest selftest。
- selftest 只证明镜像布局和声明入口正确；协议、链、存储、索引、签名或跨镜像能力必须追加真实 provider 联动、正向业务、错误输入拒绝、重启/持久化（若声明）和清理证据。
- 未执行或只完成探活的项目不得标记为业务通过。
