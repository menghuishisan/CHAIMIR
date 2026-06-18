# service/cron

`service/cron` 是 Chaimir 受控定时任务镜像。当前只实现真实备份任务:

- 使用 PostgreSQL 官方 `pg_dump` 生成可恢复数据库备份。
- 使用平台对象存储能力把代码、附件、报告桶对象服务端复制到备份桶。
- 执行结果写入 M9 `backup_record`,M9 HTTP API 只查询记录。

该镜像不提供 HTTP 服务,不得被学生或业务工作台直接访问。
