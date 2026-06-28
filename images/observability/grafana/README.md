# observability/grafana

Grafana 基于 Wolfi `grafana-13.0` 包构建,保留 Web 看板、HTTP API 和数据源能力。平台只通过 M9 嵌入经授权的只读看板入口,不向学生裸露运维界面。

看板入口配置变量名来自 `deploy/config/chaimir.env`;登录凭据和数据源密钥必须由 Secret/KMS 注入。镜像禁用运行期自动插件预装和更新检查,插件与看板必须通过受控离线包或平台配置注入。
