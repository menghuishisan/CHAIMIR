# observability/grafana

Grafana 使用官方上游固定镜像,不重打包。平台只通过 M9 嵌入经授权的只读看板入口,不向学生裸露运维界面。

看板入口配置变量名来自 `deploy/config/chaimir.env`;登录凭据和数据源密钥必须由 Secret/KMS 注入。
