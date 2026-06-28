# middleware/clamav

ClamAV 基于 Wolfi `clamav-1.5` 包构建,保留 clamd TCP `3310` 与 INSTREAM 扫描能力。后端上传链路通过 `UPLOAD_VIRUS_SCAN_ADDRESS` 连接 clamd,生产环境保持 `UPLOAD_VIRUS_SCAN_REQUIRED=true`。

生产环境只允许集群内部访问,禁止宿主机端口和学生入口。病毒库写入 `/var/lib/clamav` 数据卷;若数据卷为空,入口脚本会先通过 `freshclam` 初始化病毒库,因此部署侧必须提供受控更新出口或预热后的病毒库卷。
