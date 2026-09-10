# observability/grafana

Grafana 核心从固定上游源码重建,Prometheus 数据源保留同一固定上游镜像中带官方签名的完整插件目录,运行在官方 `13.2.0-distroless` 兼容的 distroless 运行层。镜像保留 Web 看板、HTTP API 和 Prometheus 数据源能力；平台只通过 M9 嵌入经授权的只读看板入口,不向学生裸露运维界面。不得用本地重编译插件替换带官方签名的插件,也不得关闭插件签名校验。

看板入口配置变量名来自 `deploy/config/chaimir.env`;登录凭据和数据源密钥必须由 Secret/KMS 注入。镜像禁用运行期自动插件预装和更新检查,插件与看板必须通过受控离线包或平台配置注入。
