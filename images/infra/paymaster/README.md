# infra/paymaster

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

账户抽象代付 gas 教学服务镜像。
内部请求 HMAC 使用完整 64 位十六进制签名和 16–128 位字母、数字、下划线或连字符 nonce。`CHAIMIR_IMAGE_SERVICE_AUTH_MAX_NONCES` 限制当前进程尚未过期的防重放记录；满额返回 503，不驱逐有效记录。

## 代付响应与链上验收

- `/sponsor` 返回的 `userOperationHash` 已覆盖完整的 v0.6 `paymasterAndData`；其中时间字段按官方合约要求使用两个 ABI 编码的 `uint48`，后接 64 或 65 字节 sponsor signature。该接口成功不等于 UserOperation 已上链，必须继续由合法 bundler 提交并由 EntryPoint 执行。
