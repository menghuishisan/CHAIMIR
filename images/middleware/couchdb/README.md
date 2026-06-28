# middleware/couchdb

CouchDB 基于 Wolfi `couchdb-3.3` 包构建,仅在 Fabric 富查询实验需要时由 M2 编排进沙箱容器组。Fabric 使用内嵌 LevelDB 时不启动该镜像。

本目录只治理 CouchDB 镜像来源、端口、数据卷、备份和准入策略,不定义固定组合矩阵。运行期可通过 `COUCHDB_USER`/`COUCHDB_PASSWORD` 注入管理员账号;两个变量必须同时提供,真实密钥只能来自 Secret/KMS。
