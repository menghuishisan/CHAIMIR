# runtime/evm-hardhat

Hardhat EVM 教学运行时镜像,用于 Solidity 入门、合约部署和轻量 EVM 实验。

本镜像基于 Node LTS 安装 Hardhat 工具链,同时安装 Foundry 的 Anvil 用作本地测试链。平台只暴露容器内 RPC 端口,外部访问由 M2 沙箱代理控制。
