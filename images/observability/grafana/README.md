# observability/grafana

## 镜像职责

Grafana 监控看板官方 distroless 镜像；核心服务从固定源码重建，Prometheus 数据源使用 Grafana 13.1.6 核心能力。运行时使用与源码重建二进制 ABI 匹配的、按 digest 锁定的 glibc distroless 基座。

## 能力边界

- 支持 capability：dashboards, monitoring-embed
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

- http：3000/TCP，暴露边界 gateway，用途：Grafana UI through authenticated ingress。
- 组件：无 resource_spec 组件。

## command / args 语义

- 镜像默认 ENTRYPOINT：["/usr/share/grafana/bin/grafana", "server", "--homepath=/usr/share/grafana", "--config=/etc/grafana/grafana.ini", "--packaging=docker"]；manifest 未覆盖组件 command/args，使用该入口语义。
- 镜像默认 CMD：["cfg:default.log.mode=console"]；manifest 未覆盖组件 command/args，使用该入口语义。

## 初始化材料与运行时注入

- 镜像层不内置租户、链、协议或私有材料；声明的 init_assets：无。
- 服务源码使用项目登记的 `base/go-builder` 候选 digest 构建基座；不是运行时依赖，也不作为镜像内能力发布。
- Prometheus 数据源来自固定的 Grafana 13.1.6 官方源码核心实现；上游镜像只提供固定版本的静态资源和配置，外置 bundled 插件目录在组装阶段清空，不下载、重打包、重签或允许未签名的外置插件。
- 核心源码构建阶段统一锁定 `github.com/apache/thrift v0.24.0`、`github.com/grafana/tempo v2.10.3` 和 `google.golang.org/grpc v1.83.2`。Tempo 使用固定官方源码归档和 SHA-256；由于该上游 v2 标签的 go.mod 未声明 `/v2` 模块路径，builder 内对固定源码和 Grafana 引用做确定性的模块路径规范化后再编译，不使用旧版本兼容分支或未锁定依赖。
- genesis、rollup、checkpoint、Hyperlane、Chainlink、合约或 subgraph 材料必须由平台受控对象存储、初始化资产和 CompositionSpec 注入并校验来源及 SHA-256；缺失时严格失败。
- 数据卷声明：[{"name":"data","mount_path":"/var/lib/grafana","backup_required":true}]

## Secret / KMS

- Secret 环境键：无。
- 必需材料映射：无；该镜像不得自行生成或写入密钥。
- Secret 只来自平台 Secret/KMS 运行时注入，不能写入 Dockerfile、manifest、README、镜像层或测试输出。

## 持久化与隔离边界

- 持久化目录：[{"name":"data","mount_path":"/var/lib/grafana","backup_required":true}]
- 组件挂载与文件系统：无
## 安全边界

- 安全声明：{"run_as_non_root":true,"read_only_root_filesystem":false,"allow_privilege_escalation":false,"privileged":false,"host_network":false,"automount_service_account_token":false,"drop_capabilities":["ALL"],"network_policy":"deny-all"}
- student_access：{"enabled":false}。
- 网络只允许 manifest binding/link 所需目标，默认拒绝；禁止宿主机路径、hostNetwork、ServiceAccount token 和越权写入。

## readiness / health

- 未声明组件 readiness probe；容器运行不等于就绪或业务成功。
- 上游健康检查：{"type":"http","path":"/api/health","port":3000}。

## 清理

- 验收后按 CompositionSpec 所属租户/实例和项目 owner 标签删除容器、Pod、Service、NetworkPolicy、临时卷、Secret 投影和临时材料；保留平台长期资源。
- 不使用 broad prune，不删除 kind-chaimir-cilium 节点、正式锁引用或其他项目资源。

## 验收条件

- 先按 digest lock 启动合法 CompositionSpec，核对 command/args、bindings、links、材料和 Secret 投影，再执行 manifest selftest。
- 安全门禁必须确认 Grafana 核心二进制及其随核心发布的数据源能力无 HIGH/CRITICAL；不能只检查 Grafana HTTP 探活。
- selftest 只证明镜像布局和声明入口正确；协议、链、存储、索引、签名或跨镜像能力必须追加真实 provider 联动、正向业务、错误输入拒绝、重启/持久化（若声明）和清理证据。
- 未执行或只完成探活的项目不得标记为业务通过。
