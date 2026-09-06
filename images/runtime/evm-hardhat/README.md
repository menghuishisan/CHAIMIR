# runtime/evm-hardhat

Hardhat EVM 教学运行时镜像,用于 Solidity 入门、合约部署和轻量 EVM 实验。最终镜像内的 `/usr/local/bin/chaimir-hardhat-adapter` 通过固定版本 `cast` 完成平台标准链动作。

本镜像基于 Node LTS 安装 Hardhat 工具链,同时安装 Foundry 的 Anvil 用作本地测试链。镜像离线提供 Solidity `0.8.20`、`0.8.24`、`0.8.26` 三个精确版本,路径分别为 `/usr/local/bin/solc-0.8.20`、`/usr/local/bin/solc-0.8.24`、`/usr/local/bin/solc-0.8.26`；Hardhat 项目必须在 `solidity.compilers` 中为每个版本声明对应 `path`，不使用无版本 `solc` 别名，也不允许运行时联网下载未锁定编译器。平台只暴露容器内 RPC 端口,外部访问由 M2 沙箱代理控制。
