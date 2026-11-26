# tool/zk-prover

隔离的 ZK 命令工具，直接使用锁定的官方 `circom2` 与 `snarkjs`。支持电路编译、见证计算/检查、可信设置材料处理、Groth16/PLONK/FFLONK 证明及验真；具体参数与算法范围以对应官方 CLI 为准，不维护第二套证明实现。

教师在组合中明确选择本工具。M2 使用现有 command-tool 执行路径和共享工作区，不要求链连接、网络服务、私有状态或 Secret。容器以 `1000:1000` 运行，根文件系统只读，网络默认拒绝；命令执行由平台统一设置超时与资源上限。

在工作台命令面板填写 `node` 和 `/opt/chaimir/zk-prover/run.mjs` 两项参数；输入内容为 JSON，例如：

```json
{"tool":"circom2","args":["/workspace/circuit.circom","--r1cs","--wasm","-o","/workspace/build"]}
```

或使用 `{"tool":"snarkjs","args":["groth16","verify","/workspace/verification_key.json","/workspace/public.json","/workspace/proof.json"]}`。输出目录由使用者在工作区创建，材料与结果保留在工作区。参数按数组原样交给官方 CLI，不拼接 shell。调度器拒绝未知工具、畸形输入及非字符串参数，官方工具的输出与退出状态由 M2 返回。

依赖在构建时按 `package-lock.json` 安装，运行时不下载依赖。镜像自检只证明布局和入口；正式验收必须从学生工作台编译实际电路、生成证明、验证成功，并验证篡改公开输入或证明后验真失败。再确认实验/竞赛授权、超时、错误输入及回收。新的 tool 分类须重新构建、取得供应链证明并晋升 digest，原 infra 镜像的证明不能沿用。
