# infra/ethereum-beacon

本镜像提供 Ethereum Beacon REST API，作为 OP Node 的 `ethereum-beacon-api` provider。它只提供共识层 HTTP 查询，不替代执行层，也不向学生暴露 validator 密钥。WorkloadSpec 必须注入真实 beacon chain 配置、受信任的 `BEACON_CHECKPOINT_SYNC_URL`、数据卷和 NetworkPolicy，OP Node 通过 binding 访问端口 5052。缺少 checkpoint 或只提供从 genesis 的不安全启动材料时，入口会 fail-closed；不使用 `--allow-insecure-genesis-sync` 绕过共识层安全约束。
