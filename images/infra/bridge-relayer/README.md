# infra/bridge-relayer

本镜像薄封装 Hyperlane 官方 relayer agent,用于跨链消息中继教学和实验。平台只负责非 root 运行身份、安全更新、配置挂载和 Secret 注入边界,不自研跨链协议逻辑。

真实链连接必须由 `source_chain`、`destination_chain` 两条 `evm-json-rpc` binding 注入到 `HYP_SOURCE_RPC`、`HYP_DESTINATION_RPC`。`HYP_RELAYCHAINS` 必须按源链、目标链顺序提供两个 Hyperlane chain name;入口使用内置 JSON helper 将两条绑定写入只读 base config 的运行期覆盖文件,再通过官方 `CONFIG_FILES` loader 启动 relayer。签名材料和其余 Hyperlane 合约配置由 M2 WorkloadSpec 通过 Secret/受控配置卷注入,不得写入镜像层、仓库或 manifest。缺少配置文件、链名或任一链连接时入口会拒绝启动,避免“二进制能执行”被误判为中继可用。
