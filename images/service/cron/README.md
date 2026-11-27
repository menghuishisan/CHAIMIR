# service/cron

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

受控定时任务镜像,用于真实执行平台备份并写入 M9 backup_record。
