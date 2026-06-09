# runtime/evm-besu

本镜像封装 Hyperledger Besu 开发链,用于企业 EVM、联盟 EVM 和权限链教学实验。镜像只负责容器内 Besu 进程和 `8545` JSON-RPC 端口,外部访问由 M2 控制面代理并鉴权。

