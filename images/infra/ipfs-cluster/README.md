# infra/ipfs-cluster

## 镜像职责

本镜像运行官方 IPFS Cluster `v1.1.6`，为同一 CompositionSpec 中的 IPFS provider 提供 pin 协调、状态查询、IPFS proxy 和 pinning API。它不是 IPFS 数据节点，也不替代 `infra/ipfs`；所有内容读写都通过必需的 `ipfs_api` binding 访问 IPFS provider。

## 能力边界

支持：

- IPFS Cluster REST API：`9094`；
- IPFS-compatible proxy：`9095`；
- CRDT pinning API：`9097`；
- peer transport：`9096`；
- 对已绑定 IPFS provider 的内容 add、pin、status、proxy cat 联动；
- 将官方 identity、service 配置、peerstore 和 pin 状态持久化到 `runtime-state`。

不支持：

- 单独承担 IPFS 内容存储、IPFS gateway 或 provider 发现；
- 使用通配 peer 信任、自动接受任意远端 peer 或公网节点；
- 通过宿主机端口、宿主机源码或可变镜像 tag 提供服务；
- 把 Cluster API 地址或长期凭据直接暴露给学生或业务模块。

## 合法 runtime / provider

- 合法 runtime：由 CompositionSpec 的 runtime capability 选择；该镜像本身不限定链 runtime。
- 合法 provider：同一 CompositionSpec 必须包含提供 `ipfs-node` capability 的 `infra/ipfs`。

## bindings 与 links
- 必需 binding：`ipfs_api`，来源为 `ipfs-node` provider 的 `api` endpoint，注入 `CHAIMIR_IPFS_API_URL`。
- 必需 links：无额外跨链 link；Cluster 到 IPFS 的唯一联动就是上述 binding。不得把 Cluster 自身当作 IPFS provider。
- `CHAIMIR_IPFS_CLUSTER_TRUSTED_PEERS` 为非密钥配置，使用逗号分隔的官方 peer ID；为空时入口只写入当前官方 identity。通配值 `*`、控制字符和重复 peer ID 均拒绝启动。

## command / args 语义

镜像默认 `ENTRYPOINT` 为 `/usr/local/bin/chaimir-ipfs-cluster-entrypoint`，manifest 未声明覆盖 command/args。入口先执行官方 `ipfs-cluster-service init`（仅在 `runtime-state` 中没有 `service.json` 时），写入 binding、Secret 和显式 peer 列表，然后以 `exec` 启动 `/usr/local/bin/ipfs-cluster-service daemon`；运行参数只按 CompositionSpec 的 `ComponentSpec.Args` 追加，不通过兼容入口或 generic fallback 改写。

## 端口与服务

| 端口 | 语义 | 暴露范围 |
| --- | --- | --- |
| 9094 | Cluster REST API | 集群内部 |
| 9095 | IPFS-compatible proxy | 集群内部 |
| 9096 | Cluster peer transport | 集群内部 |
| 9097 | CRDT pinning API | 集群内部 |

readiness 使用 `9094` TCP；业务验收还必须请求 `/id`、执行 pin/status 和 proxy cat，端口探活不能替代业务成功。

## 初始化材料与运行时注入

- 官方初始化材料由镜像入口在声明的 `/runtime-state/ipfs-cluster` 中生成，不写入镜像层、工作区或仓库。
- `CHAIMIR_IPFS_CLUSTER_SECRET` 是官方 CRDT `cluster.secret`，必须由平台统一 Secret/KMS 源按 manifest 的 `secrets_required` 注入；缺失、空白或短于 16 字节时严格退出。
- `CHAIMIR_IPFS_API_URL` 和 `CHAIMIR_IPFS_CLUSTER_TRUSTED_PEERS` 由 CompositionSpec 的配置与 binding 注入；入口会校验 scheme、主机、显式端口和地址族。
- 持久化边界只有 `runtime-state`，包括 `service.json`、`identity.json`、`peerstore` 和 pin 状态。每个 sandbox/实例使用独立卷，不共享其他租户或实例状态。

## Secret / KMS

`CHAIMIR_IPFS_CLUSTER_SECRET` 只能由平台 Secret/KMS 注入，不能由组合调用方把明文写入镜像或仓库。

## 持久化与隔离边界

持久化目录和实例隔离以 `runtime-state` 及 CompositionSpec owner 为边界；不得复用其他租户或实例的卷。

## 安全边界

容器以非 root 用户 `1000:1000` 运行，关闭特权提升、特权模式、host network 和 ServiceAccount token，丢弃全部 Linux capabilities。根文件系统保持可写是官方服务运行约束；除 `/runtime-state` 外不授予业务写入目录。网络默认 deny-all，只允许 CompositionSpec 声明的 IPFS provider binding 和必要的 Cluster peer 连接。Secret 不写入镜像、README、源码或日志。

## readiness / health

readiness 使用 `9094` TCP；业务验收还必须请求 `/id`、执行 pin/status 和 proxy cat，端口探活不能替代业务成功。

## 清理

验收结束必须删除本次 CompositionSpec 对应容器、网络和测试卷，确认 `docker ps -a`、`docker network ls`、`docker volume ls` 没有测试残留；不得使用 broad prune，也不得删除 `kind-chaimir-cilium` 节点或其他项目资源。

## 验收条件

候选 digest 验收使用项目的 `run-platform-provider-links.ps1`。存储专项可通过 `-StorageOnly -StorageRuntime runtime/edu-chain -EvidenceSuffix <唯一编号>` 限定一个合法 runtime，结果写入独立 TSV 且拒绝覆盖旧证据；未指定 `-StorageRuntime` 时遍历执行器声明的存储 runtime 组合，并校验所选 runtime 所需的受控材料。验收必须同时证明：IPFS add/cat/gateway 内容一致；Cluster 返回的 `peer_map` 中本地 peer 为 `pinned` 且无错误，并完成 proxy cat；IPFS 与 Cluster 分别重启后数据、pin 状态和配置仍在；缺失 Secret、缺端口 binding、wildcard peer trust 严格失败；记录容器、网络和卷清理结果。地址族入口探针另执行 `run-ipfs-cluster-address-family-functional.ps1 -DigestLockPath <候选锁> -EvidenceSuffix <唯一编号>`，只验证 IPv4、IPv6、DNS binding 的实际物化，不替代真实内容联动测试。
