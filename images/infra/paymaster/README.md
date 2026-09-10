# infra/paymaster

本镜像提供 ERC-4337 v0.6 verifying-paymaster 教学服务,用于真实生成可交给 EntryPoint 的 UserOperation 赞助数据。EVM runtime、已部署的标准 EntryPoint、已部署的 verifying-paymaster 合约地址和签名 Secret 必须由 CompositionSpec 在运行期提供;镜像不内置链上材料或私钥。

服务启动时会检查两个地址存在链上字节码,并以确定性只读 UserOperation 调用 EntryPoint 的 `getUserOpHash` 与 paymaster 的 `getHash`;合约接口调用失败时不会开放监听端口。`POST /sponsor` 只接受完整 v0.6 UserOperation,拒绝未知字段、v0.7 字段和已有 paymaster 数据。服务使用这两个合约方法生成签名,返回包含 paymaster 地址、有效期和签名的标准 `paymasterAndData`;缺少合约代码、合约接口或运行期调用失败时严格返回不可用,不会返回自定义 JSON 摘要作为代替。
