# runtime/fabric

镜像启动时在 `/runtime-state/fabric` 内用 `cryptogen`、`configtxgen` 生成单组织教学网络,再启动 `orderer` 和 `peer`,创建并加入 `chaimir` 通道。运行期身份、创世块、通道块和账本均属于当前沙箱的临时卷,不会写入镜像层或学生工作区。

链码安装、批准、提交、调用和查询只允许通过平台链能力适配器的固定 `peer` 子命令执行;Fabric Explorer 需要由组合连接显式注入 peer 端点,不使用隐式运行时地址。

客户端只使用容器内回环地址 `127.0.0.1:7050` 和 `127.0.0.1:7051`;监听仍绑定 `0.0.0.0`,由 M2 生成的 Service 代理对外提供受控访问。Fabric Explorer 只有在对应工具镜像通过供应链门禁,并由 WorkloadSpec 注入真实连接配置、证书和数据库后才能启用,不得作为默认工具隐式注入。
