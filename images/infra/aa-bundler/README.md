# infra/aa-bundler

本镜像基于 Rundler 提供 ERC-4337 bundler 教学服务,用于 UserOperation、mempool 和批量提交流程讲解。真实链 RPC 通过 `NODE_HTTP` 在运行期受控注入,`SIGNER_PRIVATE_KEYS` 只能来自平台 Secret,镜像内不固化节点地址或签名凭据；缺少 signer 时入口严格失败。
