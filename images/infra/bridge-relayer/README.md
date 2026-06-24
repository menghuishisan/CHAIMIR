# infra/bridge-relayer

本镜像薄封装 Hyperlane 官方 relayer agent,用于跨链消息中继教学和实验。平台只负责非 root 运行身份、安全更新、配置挂载和 Secret 注入边界,不自研跨链协议逻辑。

真实链连接、Hyperlane chain 配置、relayer 数据目录和签名材料必须由 M2 WorkloadSpec 通过只读配置卷、运行时状态卷和 Secret/短期凭证注入,不得写入镜像层、仓库或 manifest。
