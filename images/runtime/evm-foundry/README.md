# runtime/evm-foundry

Foundry/Anvil EVM 运行时镜像,用于 EVM 实验、forked 漏洞题和链上断言类场景。

本镜像优先复用 Foundry 官方镜像,只增加平台启动脚本、非 root 用户和 manifest 元数据。学生可进入时只能访问工作区、公开素材和运行时状态,不得挂载判题私有数据。
