# middleware/harbor

Harbor 以官方组件集治理,用于 SaaS 公网仓库或私有化校内仓。该目录不重打包 Harbor,只记录上游固定版本、离线导入、准入策略、数据卷和备份要求。

Harbor 管理凭据不得进入 manifest 或镜像层,必须由部署 Secret/KMS 注入。
