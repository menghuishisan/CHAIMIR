# middleware/nats

NATS 使用官方源码重建 `nats-server`,以平台统一 Go builder 消除已修复的 Go 标准库漏洞。`NATS_URL`、重连参数和 token 变量名统一来自 `deploy/config/chaimir.env`;真实 token 值由 Secret/KMS 注入。

生产环境默认只允许平台服务访问事件总线。
