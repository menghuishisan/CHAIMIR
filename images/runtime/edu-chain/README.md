# runtime/edu-chain

本镜像是 Chaimir 自研教学链运行时,用于共识轮转、区块哈希、交易哈希和可视化原理实验。它不替代真实公链节点,只提供可解释、确定性的教学链接口。

节点和原生能力 helper 使用同一个二进制：主进程提供 HTTP 节点，`-adapter deploy|tx|query` 通过 `CHAIMIR_CHAIN_RPC_URL` 调用本节点并输出平台统一的 JSON。平台只根据 manifest 装配动作，不按 `eco` 猜测行为；运行时重建即恢复创世态。
