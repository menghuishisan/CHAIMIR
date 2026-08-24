# tool/fabric-explorer

本镜像是平台自建的 Fabric 原生浏览器服务,通过固定参数调用运行时中的 `peer` CLI,提供通道列表、区块读取、链码查询和链码提交。它不携带固定网络、示例证书或数据库,也不把 Fabric 凭据写入镜像层。

`manifest.yaml` 保留 `runtime_config_required=true`:只有 Fabric 运行时已经提供 peer/orderer 端点、最小权限身份和受控网络规则时,工具才会在组合编译后就绪。`CORE_PEER_ADDRESS` 与 `CHAIMIR_FABRIC_ORDERER_ADDRESS` 必须由组合 links 注入,`runtime-state` 仅以只读安全域挂载组织身份和配置。学生只能通过 M2 代理访问页面和 API;命令参数经过白名单校验,原生 CLI 错误只写入带 trace_id 的工具日志。
