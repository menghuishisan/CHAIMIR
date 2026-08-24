# infra/thegraph-node

本镜像封装 The Graph 索引节点,用于 subgraph 部署、链上事件索引和 GraphQL 查询实验。它通过 manifest 的 `requires` 声明 EVM RPC、图数据 Postgres 和 `ipfs-http-api` 三类能力；M2 以显式 bindings/links 注入地址,并把数据库用户、密码作为组合 Secret 注入,入口再转换为官方进程需要的 `ETHEREUM_RPC`、`POSTGRES_URL` 和 `IPFS` 格式。没有三条连接或数据库 Secret 时不会启动。`ipfs-http-api` 可以由 Kubo API 或 IPFS Cluster 的兼容 proxy 提供，不能把 Cluster REST API 当作 IPFS API。
