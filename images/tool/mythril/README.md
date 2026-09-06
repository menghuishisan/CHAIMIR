# tool/mythril

本镜像从 PyPI 官方 `mythril==0.24.8` 构建 Mythril 符号执行工具,用于 Solidity 漏洞检测教学。运行层基于 Wolfi Python 3.10,不继承 `mythril/myth` 旧 Debian 镜像。

镜像离线提供 Solidity `0.8.20`、`0.8.24`、`0.8.26` 三个精确版本,直接可执行文件分别为 `/usr/local/bin/solc-0.8.20`、`/usr/local/bin/solc-0.8.24`、`/usr/local/bin/solc-0.8.26`,并由 `py-solc-x` 管理为 `/opt/solcx/solc-v<version>`。平台通过 `solidity_version` 参数注入 `SOLIDITY_VERSION`,受控命令入口固定为无参数 `chaimir-mythril`,它会用选定版本逐个分析 `/workspace` 下的 Solidity 文件并输出 JSON；不提供无版本 `solc` 别名,也不允许运行时联网下载编译器。

分析目标来自学生工作区,工具容器不挂载判题私有数据。`setuptools` 固定为仍提供 `pkg_resources` 的 80.9.0,因为 Mythril 的以太坊依赖仍通过该接口加载元数据。
