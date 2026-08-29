# infra/ethereum-beacon

本镜像提供 Ethereum Beacon REST API，作为 OP Node 的 `ethereum-beacon-api` provider。它只提供共识层 HTTP 查询，不替代执行层，也不向学生暴露 validator 密钥。WorkloadSpec 必须注入真实 beacon chain 配置、数据卷和 NetworkPolicy，OP Node 通过 binding 访问端口 5052。
