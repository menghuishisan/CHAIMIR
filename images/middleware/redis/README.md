# middleware/redis

Redis 使用官方镜像薄封装,只做系统包安全升级和非 root 运行固定。认证密码变量名来自 `deploy/config/chaimir.env`,真实值由 Secret/KMS 运行期注入。

生产环境只允许集群内部访问,禁止宿主机端口和学生入口。
