# runtime/bitcoin

本镜像提供 Bitcoin regtest 运行时,用于 UTXO、挖矿、交易脚本和区块确认教学。RPC 仅作为容器内端口声明,外部访问必须经平台代理。

原生能力 helper 使用镜像内 `bitcoin-cli` 和当前沙箱的 cookie 钱包：`deploy` 将声明的数据编码为 OP_RETURN 输出并挖矿确认，`tx` 执行真实 regtest UTXO 转账并确认，`query` 读取链状态或交易。它不伪装成 EVM 合约部署器。
