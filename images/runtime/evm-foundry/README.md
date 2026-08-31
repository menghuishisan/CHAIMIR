# runtime/evm-foundry

Foundry/Anvil EVM 运行时镜像,用于 EVM 实验、forked 漏洞题和链上断言类场景。

本镜像优先复用 Foundry 官方镜像,只增加平台启动脚本、非 root 用户和 manifest 元数据。学生可进入时只能访问工作区、公开素材和运行时状态,不得挂载判题私有数据。

Foundry 运行时负责提供 Anvil、Forge、Cast、Solidity 编译器和 `/usr/local/bin/chaimir-foundry-adapter` 原生能力 helper。helper 通过 stdin/stdout JSON 实现 `deploy`、`tx`、`query`，并由 M2 以 `reset_strategy=recreate_runtime` 重建运行时恢复创世态；它不是 `base/chain-tools` 的运行期注入。私钥只由平台控制面按运行期 Secret 或已审核 payload 注入,不写入镜像或学生工作区。

镜像内的 `genesis-state.json` 是固定链 ID、区块参数和平台自检账户的初始状态。入口脚本首次启动时将其复制到沙箱运行态目录，再由 Anvil 通过 `--state` 加载；这样每个新沙箱都从同一份可验证创世状态开始，运行态退出后即可销毁。
