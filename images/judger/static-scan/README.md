# judger/static-scan

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 离线编译器与私有材料

判题HOME使用自有工作卷`/judge/home`,每次执行在SVM缓存建立指向固定`/usr/local/bin/solc-<version>`的符号链接并开启Foundry离线模式。编译器从只读镜像执行,不要求工作卷允许执行。私有输入`/judge-private`只读,不得在其中建立HOME或编译缓存;空工作卷和按项目版本选择编译器均须纳入新摘要验收。

## 镜像职责

Solidity/SWC/gas 静态扫描判题镜像。
