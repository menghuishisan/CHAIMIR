# syntax=docker/dockerfile:1.7
# 本文件定义 Cosmos SDK/Tendermint 单链运行时镜像,从 Gaia 官方源码构建。
FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40 AS source

ARG GAIA_SOURCE_COMMIT=d089f568a9ef2f4d5e6d5660cb128782f4d1b2c0
ARG GAIA_SOURCE_SHA256=9d53edfac968af4a2444b0f0748bd95adb071e920ee320dae3c1078555a3a0fa

RUN for attempt in 1 2 3 4 5 6 7 8; do \
      wget -O /tmp/gaia-source.tar.gz "https://github.com/cosmos/gaia/archive/${GAIA_SOURCE_COMMIT}.tar.gz" && break; \
      if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
      sleep $((attempt * 3)); \
    done \
    && echo "${GAIA_SOURCE_SHA256}  /tmp/gaia-source.tar.gz" | sha256sum -c -

FROM golang:1.26.6-bookworm@sha256:433f9dc4f8ea3a1ce4e28f9f15d0f7c056b10475307f886d6f1ac1ccc4abd976 AS build

ARG GAIA_VERSION=v27.5.0
ARG GAIA_SOURCE_COMMIT=d089f568a9ef2f4d5e6d5660cb128782f4d1b2c0
ARG GAIA_SOURCE_SHA256=9d53edfac968af4a2444b0f0748bd95adb071e920ee320dae3c1078555a3a0fa
ARG GO_MODULE_PROXY=
ENV GOPATH=/go
ENV GOCACHE=/tmp/go-cache

COPY --from=source /tmp/gaia-source.tar.gz /tmp/gaia-source.tar.gz
RUN echo "${GAIA_SOURCE_SHA256}  /tmp/gaia-source.tar.gz" | sha256sum -c - \
    && groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /src --create-home --shell /usr/sbin/nologin chaimir \
    && mkdir -p /go /tmp/go-cache /out \
    && tar -C /src --strip-components=1 -xzf /tmp/gaia-source.tar.gz \
    && chown -R 1000:1000 /src /go /tmp/go-cache /out \
    && rm -f /tmp/gaia-source.tar.gz
USER 1000:1000
# 固定官方 tag,并升级 Trivy 已给出 fixed version 的 Go 依赖。
WORKDIR /src
RUN if [ -n "${GO_MODULE_PROXY}" ]; then go env -w GOPROXY="${GO_MODULE_PROXY}"; fi \
    && go get \
        github.com/ethereum/go-ethereum@v1.16.9 \
        github.com/hashicorp/go-getter@v1.8.6 \
        go.opentelemetry.io/otel@v1.43.0 \
        go.opentelemetry.io/otel/sdk@v1.43.0 \
        github.com/shamaton/msgpack/v2@v2.4.1 \
        github.com/bytedance/sonic@v1.15.0 \
        golang.org/x/crypto@v0.53.0 \
        golang.org/x/net@v0.56.0 \
        golang.org/x/text@v0.39.0 \
        google.golang.org/grpc@v1.82.1 \
    && go mod tidy \
    && CGO_CFLAGS="-D__BLST_PORTABLE__" go build -trimpath -tags "netgo" -ldflags "-s -w" -o /out/gaiad ./cmd/gaiad \
    && find /go/pkg/mod -name "libwasmvm.x86_64.so" -type f -print -exec cp {} /out/libwasmvm.x86_64.so \;
RUN cat > /tmp/chaimir-cosmos-entrypoint.go <<'EOF'
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func ensureEnv(key, value string) {
	if os.Getenv(key) != "" {
		return
	}
	os.Setenv(key, value)
}

func main() {
	runtimeHome := os.Getenv("CHAIMIR_COSMOS_HOME")
	ensureEnv("HOME", "/workspace")
	if os.Getenv("TMPDIR") == "" {
		if runtimeHome == "" {
			ensureEnv("TMPDIR", "/tmp")
		} else {
			ensureEnv("TMPDIR", runtimeHome+"/tmp")
		}
	}
	for _, dir := range []string{os.Getenv("HOME"), os.Getenv("TMPDIR")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			fmt.Fprintf(os.Stderr, "创建 Cosmos 运行目录失败: %v\n", err)
			os.Exit(1)
		}
	}
	if os.Getenv("CHAIMIR_SELFTEST") == "1" {
		execGaiad([]string{"gaiad", "version"})
	}
	if runtimeHome == "" {
		fmt.Fprintln(os.Stderr, "必须通过初始化卷提供 CHAIMIR_COSMOS_HOME")
		os.Exit(1)
	}
	if err := ensureCosmosHome(runtimeHome); err != nil {
		fmt.Fprintf(os.Stderr, "初始化 Cosmos 运行目录失败: %v\n", err)
		os.Exit(1)
	}
	if err := ensureCosmosBindings(runtimeHome); err != nil {
		fmt.Fprintf(os.Stderr, "配置 Cosmos 网络监听失败: %v\n", err)
		os.Exit(1)
	}
	minimumGasPrices := os.Getenv("CHAIMIR_COSMOS_MIN_GAS_PRICES")
	if minimumGasPrices == "" {
		minimumGasPrices = "0.0stake"
	}
	execGaiad([]string{"gaiad", "start", "--home", runtimeHome, "--minimum-gas-prices", minimumGasPrices})
}

// ensureCosmosBindings 让容器内 RPC、REST 和 gRPC 监听在可由 Service 访问的地址。
func ensureCosmosBindings(home string) error {
	rpcLaddr := os.Getenv("CHAIMIR_COSMOS_RPC_LADDR")
	if rpcLaddr == "" {
		rpcLaddr = "tcp://0.0.0.0:26657"
	}
	restAddress := os.Getenv("CHAIMIR_COSMOS_REST_ADDRESS")
	if restAddress == "" {
		restAddress = "tcp://0.0.0.0:1317"
	}
	grpcAddress := os.Getenv("CHAIMIR_COSMOS_GRPC_ADDRESS")
	if grpcAddress == "" {
		grpcAddress = "0.0.0.0:9090"
	}
	for name, value := range map[string]string{
		"CHAIMIR_COSMOS_RPC_LADDR":     rpcLaddr,
		"CHAIMIR_COSMOS_REST_ADDRESS":  restAddress,
		"CHAIMIR_COSMOS_GRPC_ADDRESS":  grpcAddress,
	} {
		if strings.ContainsAny(value, "\r\n\"") {
			return fmt.Errorf("%s 包含非法配置字符", name)
		}
	}
	configPath := filepath.Join(home, "config", "config.toml")
	if err := setTomlValue(configPath, "rpc", "laddr", strconv.Quote(rpcLaddr)); err != nil {
		return err
	}
	appPath := filepath.Join(home, "config", "app.toml")
	if err := setTomlValue(appPath, "api", "enable", "true"); err != nil {
		return err
	}
	if err := setTomlValue(appPath, "api", "address", strconv.Quote(restAddress)); err != nil {
		return err
	}
	return setTomlValue(appPath, "grpc", "address", strconv.Quote(grpcAddress))
}

// setTomlValue 修改指定 TOML section 中的单行键,保留其余用户配置和注释。
func setTomlValue(path, section, key, value string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取 %s 失败: %w", path, err)
	}
	lines := strings.Split(string(data), "\n")
	current := ""
	found := false
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			current = strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]")
			continue
		}
		if current != section || !strings.HasPrefix(trimmed, key+" =") {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		lines[index] = indent + key + " = " + value
		found = true
	}
	if !found {
		return fmt.Errorf("%s [%s].%s 不存在", path, section, key)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", path, err)
	}
	return nil
}

// ensureCosmosHome 只在空运行态卷中生成单节点创世和验证者,不覆盖已有链状态。
func ensureCosmosHome(home string) error {
	marker := filepath.Join(home, ".chaimir-initialized")
	if _, err := os.Stat(marker); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("读取初始化标记失败: %w", err)
	}
	genesis := filepath.Join(home, "config", "genesis.json")
	if _, err := os.Stat(genesis); err == nil {
		return fmt.Errorf("运行目录已有创世文件但缺少平台初始化标记")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("读取创世文件状态失败: %w", err)
	}
	if err := runGaiad(home, "init", "chaimir", "--chain-id", "chaimir-local"); err != nil {
		return fmt.Errorf("生成创世配置失败: %w", err)
	}
	if err := runGaiad(home, "keys", "add", "validator", "--keyring-backend", "test"); err != nil {
		return fmt.Errorf("生成验证者密钥失败: %w", err)
	}
	address, err := runGaiadOutput(home, "keys", "show", "validator", "-a", "--keyring-backend", "test")
	if err != nil || strings.TrimSpace(address) == "" {
		return fmt.Errorf("读取验证者地址失败: %w", err)
	}
	if err := runGaiad(home, "genesis", "add-genesis-account", strings.TrimSpace(address), "1000000000stake", "--keyring-backend", "test"); err != nil {
		return fmt.Errorf("写入创世账户失败: %w", err)
	}
	if err := runGaiad(home, "genesis", "gentx", "validator", "100000000stake", "--chain-id", "chaimir-local", "--keyring-backend", "test"); err != nil {
		return fmt.Errorf("生成创世交易失败: %w", err)
	}
	if err := runGaiad(home, "genesis", "collect-gentxs"); err != nil {
		return fmt.Errorf("收集创世交易失败: %w", err)
	}
	if err := os.WriteFile(marker, []byte("chaimir cosmos runtime initialized\n"), 0600); err != nil {
		return fmt.Errorf("写入初始化标记失败: %w", err)
	}
	return nil
}

// runGaiad 执行初始化子命令并丢弃输出,防止助记词和密钥材料进入容器日志。
func runGaiad(home string, args ...string) error {
	commandArgs := append([]string(nil), args...)
	commandArgs = append(commandArgs, "--home", home)
	command := exec.Command("/usr/local/bin/gaiad", commandArgs...)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	return command.Run()
}

// runGaiadOutput 只返回明确的公开命令输出,用于把密钥名称解析为创世账户地址。
func runGaiadOutput(home string, args ...string) (string, error) {
	commandArgs := append([]string(nil), args...)
	commandArgs = append(commandArgs, "--home", home)
	command := exec.Command("/usr/local/bin/gaiad", commandArgs...)
	command.Stderr = io.Discard
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func execGaiad(args []string) {
	if err := syscall.Exec("/usr/local/bin/gaiad", args, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "启动 gaiad 失败: %v\n", err)
		os.Exit(1)
	}
}

EOF
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/chaimir-cosmos-entrypoint /tmp/chaimir-cosmos-entrypoint.go \
    && mkdir -p /out/rootfs/workspace /out/rootfs/runtime-state/cosmos /out/rootfs/tmp

FROM debian:13-slim@sha256:abc9cb88a5587630d7f915f47b23b0668fe250fbfc6457aa4d52b534c1bbf73f

LABEL org.opencontainers.image.title="runtime/cosmos"
LABEL org.opencontainers.image.description="Chaimir Cosmos SDK local chain runtime built from official Gaia source"

RUN mkdir -p /usr/local/lib /workspace /tmp \
    && chown -R 1000:1000 /usr/local/lib /workspace /tmp

COPY --from=build --chown=1000:1000 /out/rootfs/ /
COPY --from=build --chown=1000:1000 /out/gaiad /usr/local/bin/gaiad
COPY --from=build --chown=1000:1000 /out/libwasmvm.x86_64.so /usr/local/lib/libwasmvm.x86_64.so
COPY --from=build --chown=1000:1000 /out/chaimir-cosmos-entrypoint /usr/local/bin/chaimir-cosmos-entrypoint

ENV LD_LIBRARY_PATH=/usr/local/lib
ENV HOME=/workspace
ENV TMPDIR=/tmp
USER 1000:1000
WORKDIR /workspace
EXPOSE 26657 1317
ENTRYPOINT ["/usr/local/bin/chaimir-cosmos-entrypoint"]
