# tool/slither

Solidity 静态安全扫描工具镜像。

本镜像通过官方 Python 包安装 `slither-analyzer`,并固定校验 Solidity 官方 `solc` Linux 二进制,不自研扫描逻辑。它作为命令行工具容器使用,不暴露端口。
