# runtime/evm-besu

本镜像封装 Hyperledger Besu 开发链,用于企业 EVM、联盟 EVM 和权限链教学实验。镜像只负责容器内 Besu 进程和 `8545` JSON-RPC 端口,外部访问由 M2 控制面代理并鉴权。Besu dev 网络不暴露可用于 `eth_sendTransaction` 的托管账户,因此交易由外部工具镜像或学生代码签名后通过 JSON-RPC 提交;在提供通用签名适配层前,本镜像保持 `adapter_level: 1`,不能被平台原生动作选择器当作可交易 runtime。

构建阶段只替换官方 Besu 上游中需要安全升级的 Netty/Jackson JAR。`patch-artifacts.sha256` 固定所有 55 个下载件的官方 SHA256, Dockerfile 在每次下载后逐件校验；摘要文件自身的 SHA256 也登记在 manifest 中。摘要校验失败时构建必须失败,不得继续生成镜像。
