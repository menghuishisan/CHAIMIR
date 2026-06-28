# infra/wallet-backend

本镜像提供钱包后端教学服务,用于 DApp 登录、签名挑战和会话绑定实验。镜像不内置私钥;签名私钥和会话密钥必须由 K8s Secret/KMS 在运行期注入,且该 infra 容器不对学生开放 shell。
