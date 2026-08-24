# infra/ipfs-cluster

本镜像封装 IPFS Cluster,用于多节点 pinning、内容复制和存储容错实验。`ipfs_api` binding 将任意提供 `ipfs-node` 能力的组件 API 地址注入 `CHAIMIR_IPFS_API_URL`;镜像入口把它写入官方 `service.json` 的 IPFS proxy 与 connector 两处配置后再启动 daemon。集群 Secret、peer 配置和拓扑由 M2 初始化卷注入，缺少初始化配置或 API 绑定时拒绝启动。
