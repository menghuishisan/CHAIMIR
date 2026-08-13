# network/cilium

该镜像使用唯一的 `network/cilium-builder` 已修复源码树构建 Cilium agent，运行层和 Envoy 均来自同一 Harbor `network` 分类依赖。Chart 才负责赋予 CNI 所需的系统权限；该镜像不用于学生或业务 Pod。
