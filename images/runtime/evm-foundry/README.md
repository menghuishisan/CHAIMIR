# runtime/evm-foundry

Foundry/Anvil EVM 运行时镜像,用于 EVM 实验、forked 漏洞题和链上断言类场景。

本镜像优先复用 Foundry 官方镜像,只增加平台启动脚本、非 root 用户和 manifest 元数据。学生可进入时只能访问工作区、公开素材和运行时状态,不得挂载判题私有数据。

`chaimir-chain deploy/tx` 不内置默认私钥。外部或需保密的账户必须由 M2 运行期 payload 或 `CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY` Secret 注入私钥;平台托管的隔离 Anvil 可在 payload 显式传入 `from` 地址,通过节点已解锁账户发送。`from` 模式不得用于未受平台控制的远程 RPC。
