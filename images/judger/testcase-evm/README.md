# judger/testcase-evm

本镜像执行 EVM 测试用例判题。提交目录中存在 `foundry.toml` 时运行 `forge test`,存在 Hardhat 配置时运行 `npx hardhat test`。

Hardhat 判题项目统一使用 Hardhat 3 和 ESM 工程形态,`package.json` 必须声明 `"type": "module"`。平台处于开发阶段,不保留 Hardhat 2 CommonJS 兼容分支。

判题器运行时强制使用镜像内固定版本 Hardhat,不会执行提交目录里的 Hardhat 依赖。镜像离线携带 Solidity `0.8.20`、`0.8.24`、`0.8.26` 三个精确编译器；判题器只接受提交项目显式声明的 Hardhat/Foundry 编译器配置，并把配置版本解析到镜像内对应路径。缺少版本、未声明版本或配置要求外网下载时严格失败，不使用默认版本或无版本 `solc` 别名。
