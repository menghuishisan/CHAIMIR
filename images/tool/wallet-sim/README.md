# tool/wallet-sim

本镜像提供钱包模拟器,用于账户、授权和教学签名演示。模拟签名只服务课堂实验,不保存真实私钥,也不直接连接链节点或平台控制面。

真实交易发送必须由工作台调用 M2 统一链能力 API(`/api/v1/sandbox/sandboxes/{id}/chain/tx`),由后端按沙箱 owner 或内部服务 `source_ref` 完成鉴权和运行时能力调度。
