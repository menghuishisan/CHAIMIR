# service/migrate

部署期迁移、授权和 seed 命令镜像,构建 `backend/cmd/migrate`。

本镜像只作为 Job 或初始化流程运行,不对外暴露端口。数据库连接和管理员初始化参数必须由运行期环境变量或 Secret 注入,不得进入镜像层。
