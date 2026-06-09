# base/node-builder

Node 前端构建基座镜像,用于后续 React 前端构建和需要 Node 工具链的镜像多阶段构建。

本镜像复用官方 Node LTS slim 镜像,只统一工作目录和元数据。它不是学生运行环境,不声明对外端口。
