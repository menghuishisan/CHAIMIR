# 本文件定义 Jupyter Notebook 工具镜像。
FROM quay.io/jupyter/minimal-notebook:python-3.12

LABEL org.opencontainers.image.title="tool/jupyter"
LABEL org.opencontainers.image.description="Chaimir Jupyter notebook tool image"

USER root
RUN mkdir -p /workspace \
    && chown -R jovyan:users /workspace

USER jovyan
WORKDIR /workspace
EXPOSE 8888
CMD ["start-notebook.py", "--NotebookApp.token=", "--NotebookApp.password=", "--ServerApp.allow_origin=*", "--ServerApp.ip=0.0.0.0"]
