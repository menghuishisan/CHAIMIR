# middleware/nats

NATS 使用官方上游固定镜像,不重打包。`NATS_URL`、重连参数和 token 变量名统一来自 `deploy/config/chaimir.env`;真实 token 值由 Secret/KMS 注入。

生产环境默认只允许平台服务访问事件总线。
