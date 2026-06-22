# tool/db-viewer

本镜像提供状态数据库可视化工具,用于 Fabric CouchDB 或实验内 PostgreSQL 状态查看。

`manifest.yaml` 声明 `tool.runtime_config_required=true`:只有当运行时/实验 WorkloadSpec 已真实提供目标数据库 Service、最小权限短期凭证、NetworkPolicy 和平台代理路由后,才能把该工具标记为可调度。连接串不得由教师或学生手填,也不得暴露数据库直连入口。
