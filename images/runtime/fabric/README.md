# runtime/fabric

镜像启动时在 `/runtime-state/fabric` 内用 `cryptogen`、`configtxgen` 生成单组织教学网络,运行态生成 TLS CA/证书和单节点 etcdraft 创世块,再启动 `orderer` 和 `peer`,创建并加入 `chaimir` 通道。运行期身份、TLS 材料、创世块、通道块和账本均属于当前沙箱的临时卷,不会写入镜像层或学生工作区。检测到旧的明文/solo 状态时会直接拒绝复用,不会迁移或降级。

链码安装、批准、提交、调用和查询只允许通过平台链能力适配器的固定 `peer` 子命令执行;Fabric Explorer 需要由组合连接显式注入 peer 端点,不使用隐式运行时地址。

运行时内部客户端使用 `127.0.0.1` 并通过证书 SAN 和受控 hostname override 校验 TLS;监听仍绑定 `0.0.0.0`,由 M2 生成的 Service 代理对外提供受控访问。Fabric Explorer 只有在对应工具镜像通过供应链门禁,并由 WorkloadSpec 注入真实 peer/orderer 连接和只读 TLS 身份后才能启用,不得作为默认工具隐式注入。该镜像提供 cryptogen 生成的受控网络身份根,不提供独立 Fabric CA enrollment 服务;需要动态注册多组织身份的组合必须另行声明 CA 组件。
