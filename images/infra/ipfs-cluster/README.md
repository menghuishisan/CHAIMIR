# infra/ipfs-cluster

> 更新：2026-11-27。端口、命令、资源、探针和安全字段见本目录 [manifest](manifest.yaml)；通用构建、供应链与隔离见[镜像规范](../../../docs/总-镜像与容器设计.md)。

为同一组合内提供 `ipfs-node` 能力的 IPFS provider 协调 pin、查询状态及提供原生 proxy/pinning API。Cluster 不存储 IPFS 内容，不替代 IPFS provider；所有内容访问经显式 `ipfs_api` binding，不按名称发现或连接外部节点。API 仅在组合授权范围内访问，不向学生或公网直接公开长期凭据。

## 身份、配置与持久化

入口仅在运行域没有 `service.json` 时执行官方初始化，随后写入连接、Secret 和明确的 peer 列表，以 exec 交接原生 daemon；追加参数保持 argv 语义。

- `CHAIMIR_IPFS_CLUSTER_SECRET` 由平台 Secret/KMS 注入，为 32 字节、64 位十六进制的原生 CRDT cluster secret；缺失、空白、长度或编码不符时拒绝。读取后从子进程环境清除，官方初始化和 daemon 不继承原始环境键。
- `CHAIMIR_IPFS_CLUSTER_TRUSTED_PEERS` 是明确的逗号分隔 peer ID；空值只信任当前 identity，拒绝通配符、控制字符与重复 peer。
- binding URL 校验 scheme、主机、显式端口和地址族；不得注入任意外部目标。
- identity、service 配置、peerstore 与 pin 状态只保存在本实例 `runtime-state`，不共享其他租户或实例卷。配置保留 JSON 数字精度、拒绝尾随文档，在同目录 0600 临时文件写入、同步并关闭后替换；失败保留旧配置并报告写入、关闭和清理错误。
- 根文件系统可写属于原生服务运行约束，业务持久写入只授予声明的运行域；网络仅允许绑定 IPFS 和受控 Cluster peer。

## 专项验收

使用 `run-platform-provider-links.ps1` 的存储流程。`-StorageOnly -StorageRuntime runtime/edu-chain -EvidenceSuffix <唯一编号>` 可限定一个合法 runtime；省略 runtime 时遍历执行器声明的存储组合，校验其受控材料并写入不覆盖的独立证据。

验证 IPFS add/cat/gateway 内容一致；Cluster `peer_map` 中本地 peer 为 pinned 且无错误，完成 pin/status 与 proxy cat；IPFS 和 Cluster 分别重启后内容、配置、身份及 pin 状态仍在。缺 Secret、缺端口 binding 与 wildcard trust 必须失败。TCP readiness 只证明端口监听，不能代替上述操作。

另以 `run-ipfs-cluster-address-family-functional.ps1 -DigestLockPath <候选锁> -EvidenceSuffix <唯一编号>` 验证 IPv4、IPv6、DNS binding 的实际物化；不代替真实内容联动。结束后按本次组合归属回收容器、网络及测试卷，保留平台长期资源和证据。
