# 本文件定义 EVM 测试用例判题镜像,基于已验证 digest 保留离线 Hardhat/solc 工具链。
ARG EVM_HARDHAT_IMAGE=invalid.invalid/runtime/evm-hardhat:required
ARG JUDGE_MIN_IMAGE=invalid.invalid/base/judge-min:required
FROM ${EVM_HARDHAT_IMAGE} AS solidity-source
FROM ${JUDGE_MIN_IMAGE} AS judge-base
FROM ${EVM_HARDHAT_IMAGE} AS base

USER root
COPY run_evm_tests.sh /usr/local/bin/run-evm-tests
COPY hardhat-offline-wrapper.mjs /opt/chaimir/testcase-evm/hardhat-offline-wrapper.mjs
COPY rpc-probe.mjs /opt/chaimir/testcase-evm/rpc-probe.mjs
COPY --from=judge-base /usr/local/bin/normalize-result /usr/local/bin/normalize-result
COPY --from=solidity-source /usr/local/bin/solc-0.8.20 /usr/local/bin/solc-0.8.20
COPY --from=solidity-source /usr/local/bin/solc-0.8.24 /usr/local/bin/solc-0.8.24
COPY --from=solidity-source /usr/local/bin/solc-0.8.26 /usr/local/bin/solc-0.8.26
RUN chmod 0755 /usr/local/bin/run-evm-tests \
    && mkdir -p /opt/chaimir/cache /judge /tmp \
    && mkdir -p /opt/chaimir/cache/hardhat-nodejs/compilers-v3/linux-amd64 \
    && chmod 0755 /usr/local/bin/solc-0.8.20 /usr/local/bin/solc-0.8.24 /usr/local/bin/solc-0.8.26 \
    && for spec in '0.8.20|a1b79de6' '0.8.24|e11b9ed9' '0.8.26|8a97fa7a'; do \
        version="${spec%%|*}"; commit="${spec#*|}"; \
        cp "/usr/local/bin/solc-${version}" "/opt/chaimir/cache/hardhat-nodejs/compilers-v3/linux-amd64/solc-linux-amd64-v${version}+commit.${commit}"; \
        chmod 0755 "/opt/chaimir/cache/hardhat-nodejs/compilers-v3/linux-amd64/solc-linux-amd64-v${version}+commit.${commit}"; \
    done \
    && printf '%s' '{"builds":[{"path":"solc-linux-amd64-v0.8.20+commit.a1b79de6","version":"0.8.20","build":"commit.a1b79de6","longVersion":"0.8.20+commit.a1b79de6","keccak256":"0x","sha256":"0x","urls":[]},{"path":"solc-linux-amd64-v0.8.24+commit.e11b9ed9","version":"0.8.24","build":"commit.e11b9ed9","longVersion":"0.8.24+commit.e11b9ed9","keccak256":"0x","sha256":"0x","urls":[]},{"path":"solc-linux-amd64-v0.8.26+commit.8a97fa7a","version":"0.8.26","build":"commit.8a97fa7a","longVersion":"0.8.26+commit.8a97fa7a","keccak256":"0x","sha256":"0x","urls":[]}],"releases":{"0.8.20":"solc-linux-amd64-v0.8.20+commit.a1b79de6","0.8.24":"solc-linux-amd64-v0.8.24+commit.e11b9ed9","0.8.26":"solc-linux-amd64-v0.8.26+commit.8a97fa7a"},"latestRelease":"0.8.26"}' > /opt/chaimir/cache/hardhat-nodejs/compilers-v3/linux-amd64/list.json \
    && chown -R 1000:1000 /opt/chaimir /judge /tmp

USER 1000:1000
ENV HOME=/judge/home
ENV FOUNDRY_OFFLINE=true
WORKDIR /judge
ENTRYPOINT ["run-evm-tests"]
