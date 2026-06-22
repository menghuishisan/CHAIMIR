# tool/contract-ui

本镜像提供合约 ABI 交互面板服务,用于按 ABI 展示方法和构造调用参数。镜像自身不保存私钥、不直连链节点、不暴露 RPC。

真实部署、查询和交易必须由工作台调用 M2 统一链能力 API(`/api/v1/sandbox/sandboxes/{id}/chain/*`),由后端按沙箱 owner 或内部服务 `source_ref` 完成鉴权、状态校验和运行时能力调度。
