# runtime/evm-geth

go-ethereum 私链运行时镜像,用于真实节点形态的 EVM 教学实验。

本镜像从官方 go-ethereum 源码构建,并在最终镜像内提供 `/usr/local/bin/chaimir-geth-adapter`。适配器通过 JSON-RPC 完成合约部署、交易、查询;链状态目录由平台挂载,不得写入镜像层。
