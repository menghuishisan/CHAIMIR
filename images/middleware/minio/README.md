# middleware/minio

MinIO 社区 Docker 镜像已不能作为当前生产来源使用,本目录改为从官方源码 tag 构建平台镜像。构建产物进入 Harbor 后,生产 digest 统一写入 `images/image-digests.lock`,不得回退到上游 tag 拉取。

桶名等非密配置和访问密钥变量名统一来自 `deploy/config/chaimir.env`;真实密钥值由 Secret/KMS 注入。生产环境不得向学生或公网直接暴露控制台。
