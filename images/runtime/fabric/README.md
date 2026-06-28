# runtime/fabric

Hyperledger Fabric 运行时工具镜像,用于 Fabric 实验中的 peer/orderer/CA 管理脚本和链码测试工具。

Fabric 多节点网络不是单个镜像完成,而是由 M2 根据 manifest 和实验拓扑启动多个官方 Fabric 组件容器。本镜像只提供 Fabric 工具链和平台约束入口,不伪造完整联盟链。Fabric Explorer 只有在对应工具镜像通过供应链门禁并由 WorkloadSpec 注入真实 connection profile、证书和数据库后才能作为可选工具启用,不得作为默认可调度工具。
