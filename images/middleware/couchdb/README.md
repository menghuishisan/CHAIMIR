# middleware/couchdb

CouchDB 仅在 Fabric 富查询实验需要时由 M2 编排进沙箱容器组。Fabric 使用内嵌 LevelDB 时不启动该镜像。

本目录只治理官方上游固定镜像来源、端口、数据卷、备份和准入策略,不定义固定组合矩阵。
