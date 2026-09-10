# infra/wallet-backend

本镜像提供钱包后端教学服务,用于 DApp 登录、签名挑战和会话绑定实验。用户地址由调用方提交并绑定到挑战消息,服务使用 `ethers.verifyMessage` 校验外部钱包签名;服务只接收内部 HMAC 鉴权密钥,不保存或代替用户持有私钥。挑战状态持久化在 `/runtime-state` 并以原子文件替换更新,该 infra 容器不对学生开放 shell。

`POST /challenge` 要求 `domain` 和 EVM `address`,签发有时效的一次性 EIP-191 文本挑战。`POST /verify` 要求原始挑战、签名和同一地址；只有签名验证成功才消费挑战，错误签名不会让合法用户失去重试机会，成功后的重放、过期和错误地址都会失败。
