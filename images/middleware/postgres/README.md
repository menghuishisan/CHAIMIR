# middleware/postgres

PostgreSQL 主数据库镜像使用官方镜像薄封装,只做系统包安全升级、移除旧 `gosu` 二进制并固定非 root 运行。运行期变量名统一来自 `deploy/config/chaimir.env`;真实密码由 Secret/KMS 注入同名环境变量,不得进入仓库、manifest 或镜像层。

生产部署必须按 digest 拉取,完成漏洞扫描、签名或准入策略校验,并对数据卷执行逻辑备份和卷备份。
