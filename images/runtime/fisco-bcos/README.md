# runtime/fisco-bcos

本镜像封装 FISCO BCOS 节点运行时。`CHAIMIR_FISCO_CONFIG` 是组合的必需配置键,必须指向 M2 生成并校验的运行期 `config.ini`;配置和节点拓扑不写入镜像。入口在配置缺失、路径不可读或不由组合注入时显式失败,避免以不受控默认网络启动。账本和日志写入 runtime-state,外部 RPC/P2P 访问由组合生成的 Service 和 NetworkPolicy 控制。
