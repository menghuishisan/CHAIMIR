# tool/wallet-sim

本镜像提供钱包模拟器,用于账户、授权和教学签名教学。容器启动时在内存中生成临时教学账户,不把私钥写入仓库、镜像层、文件系统或 manifest。签名接口按 Ethereum `personal_sign` 语义生成可恢复地址的 secp256k1 签名;该账户仅用于沙箱教学,不得用于真实资产。

真实交易发送必须由工作台调用 M2 统一链能力 API(`/api/v1/sandbox/sandboxes/{id}/chain/tx`),由后端按沙箱 owner 或内部服务 `source_ref` 完成鉴权和运行时能力调度。
