# 本文件定义 Jupyter Notebook 工具镜像。
FROM quay.io/jupyter/minimal-notebook:python-3.12@sha256:5b868d38f2b454cb22abb077052d341d3d7aeec2315c296c44de3b1278425782

LABEL org.opencontainers.image.title="tool/jupyter"
LABEL org.opencontainers.image.description="Chaimir Jupyter notebook tool image"

USER root
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y upgrade \
    && /opt/conda/bin/python -m pip install --no-cache-dir --upgrade \
        "setuptools" \
        "jupyter_server>=2.17.0" \
        "jupyterlab>=4.5.0" \
        "notebook>=7.5.0" \
        "nbconvert>=7.17.0" \
        "mistune>=3.2.1" \
        "tornado>=6.5.2" \
        "urllib3>=2.6.0" \
        "PyJWT>=2.10.2" \
        "wheel>=0.46.0" \
        "Mako>=1.3.10" \
        "Brotli>=1.2.0" \
        "cryptography>=46.0.0" \
        "soupsieve>=2.8.4" \
    && /opt/conda/bin/python -m pip uninstall -y nbclassic \
    && rm -rf /var/lib/apt/lists/* /root/.cache/pip \
    && groupadd --gid 1000 chaimir \
    && usermod --gid 1000 jovyan \
    && mkdir -p /workspace \
    && chown -R 1000:1000 /workspace /home/jovyan

USER 1000:1000
WORKDIR /workspace
EXPOSE 8888

# 直接启动 Jupyter Server,避免上游 start.sh 在日志中回显 token/password 参数名。
ENTRYPOINT ["jupyter"]
CMD ["lab", "--IdentityProvider.token=", "--PasswordIdentityProvider.hashed_password=", "--ServerApp.allow_origin=*", "--ServerApp.ip=0.0.0.0", "--ServerApp.root_dir=/workspace", "--ServerApp.allow_remote_access=True", "--no-browser"]
