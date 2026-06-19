# middleware/clamav

ClamAV 使用官方上游固定镜像 `clamav/clamav:1.4.4-debian`,不重打包。后端上传链路通过 `UPLOAD_VIRUS_SCAN_ADDRESS` 连接 clamd TCP 端口 `3310`,生产环境保持 `UPLOAD_VIRUS_SCAN_REQUIRED=true`。

生产环境只允许集群内部访问,禁止宿主机端口和学生入口。病毒库写入 `/var/lib/clamav` 数据卷,运行时目录使用 `emptyDir`。
