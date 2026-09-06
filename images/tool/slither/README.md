# tool/slither

Solidity 静态安全扫描工具镜像。

本镜像通过官方 Python 包安装 `slither-analyzer`,并校验 Solidity 官方 `solc` Linux 二进制目录中的 `0.8.20`、`0.8.24`、`0.8.26`,分别位于 `/usr/local/bin/solc-0.8.20`、`/usr/local/bin/solc-0.8.24`、`/usr/local/bin/solc-0.8.26`。组合通过 `solidity_version` 参数注入 `SOLIDITY_VERSION`,受控命令入口固定为无参数 `chaimir-slither`,它会用选定版本扫描 `/workspace` 并输出 Slither JSON；不提供无版本 `solc` 别名,也不允许运行时联网下载编译器。
