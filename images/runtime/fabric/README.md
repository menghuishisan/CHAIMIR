# runtime/fabric

Hyperledger Fabric 运行时工具镜像,用于 Fabric 实验中的 peer/orderer/CA 管理脚本和链码测试工具。

Fabric 多节点网络不是单个镜像完成,而是由 M2 根据 manifest 和实验拓扑启动多个官方 Fabric 组件容器。本镜像只提供 Fabric 工具链和平台约束入口,不伪造完整联盟链。
