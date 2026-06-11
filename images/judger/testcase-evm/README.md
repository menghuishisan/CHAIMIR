# judger/testcase-evm

本镜像执行 EVM 测试用例判题。提交目录中存在 `foundry.toml` 时运行 `forge test`,存在 Hardhat 配置时运行 `npx hardhat test`。

Hardhat 判题项目统一使用 Hardhat 3 和 ESM 工程形态,`package.json` 必须声明 `"type": "module"`。平台处于开发阶段,不保留 Hardhat 2 CommonJS 兼容分支。

判题器运行时强制使用镜像内固定版本 Hardhat,不会执行提交目录里的 Hardhat 依赖。镜像构建期预热 Solidity `0.8.26` 编译器缓存,判题期不允许依赖外网下载编译器。
