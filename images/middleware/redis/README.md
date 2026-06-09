# middleware/redis

Redis 使用官方上游固定镜像,不重打包。认证密码变量名来自 `deploy/config/chaimir.env`,真实值由 Secret/KMS 运行期注入。

生产环境只允许集群内部访问,禁止宿主机端口和学生入口。
