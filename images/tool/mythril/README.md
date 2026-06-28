# tool/mythril

本镜像从 PyPI 官方 `mythril==0.24.8` 构建 Mythril 符号执行工具,用于 Solidity 漏洞检测教学。运行层基于 Wolfi Python 3.10,不继承 `mythril/myth` 旧 Debian 镜像。

分析目标来自学生工作区,工具容器不挂载判题私有数据。`setuptools` 固定为仍提供 `pkg_resources` 的 80.9.0,因为 Mythril 的以太坊依赖仍通过该接口加载元数据。
