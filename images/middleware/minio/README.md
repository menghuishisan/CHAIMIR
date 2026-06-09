# middleware/minio

MinIO 使用官方上游固定镜像,不重打包。桶名等非密配置和访问密钥变量名统一来自 `deploy/config/chaimir.env`;真实密钥值由 Secret/KMS 注入。

生产环境不得向学生或公网直接暴露控制台。
