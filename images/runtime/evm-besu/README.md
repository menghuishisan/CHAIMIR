# runtime/evm-besu

本镜像封装 Hyperledger Besu 单节点 QBFT 教学链,用于企业 EVM、联盟 EVM 和权限链教学实验。首次启动时,入口在私有 `runtime-state` 中调用官方 generator 生成唯一 validator key 和 genesis,然后启动可出块的链；生成的节点材料不进入镜像、manifest 或仓库。模板只包含公开的开发测试账户地址和余额,交易仍由外部工具镜像或学生代码签名后通过 JSON-RPC 提交。镜像只负责容器内 Besu 进程和 `8545` JSON-RPC 端口,外部访问由 M2 控制面代理并鉴权。

QBFT genesis 固定启用 London 和 Shanghai fork,与项目离线提供的 Solidity 编译器兼容。`/runtime-state/besu/chaimir-qbft` 是受控初始化目录；该目录已存在但不完整、genesis 缺失、validator 数量不为 1 或节点密钥为空时,入口严格失败,不会回退到无出块的 dev/Ethash 网络。运行数据位于同一 runtime-state 卷内,重启后复用已生成材料。

构建阶段只替换官方 Besu 上游中需要安全升级的 Netty/Jackson JAR。`patch-artifacts.sha256` 固定所有 55 个下载件的官方 SHA256, Dockerfile 在每次下载后逐件校验；摘要文件自身的 SHA256 也登记在 manifest 中。摘要校验失败时构建必须失败,不得继续生成镜像。
