// 本文件校验平台运行时契约,只初始化一次 rollup genesis,并启动 OP Geth
// 执行 API 与 Engine API。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	dataDir := required("OP_GETH_DATA_DIR")
	genesis := required("OP_GETH_GENESIS_PATH")
	jwt := required("OP_GETH_JWT_SECRET")
	enginePort := requiredPort("OP_GETH_ENGINE_PORT")
	rpcPort := requiredPort("OP_GETH_RPC_PORT")
	if len(jwt) != 64 {
		fail("OP_GETH_JWT_SECRET must contain 32 bytes as 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(jwt); err != nil {
		fail("OP_GETH_JWT_SECRET is not valid hexadecimal")
	}
	if info, err := os.Stat(genesis); err != nil || info.IsDir() {
		fail("OP_GETH_GENESIS_PATH must point to a mounted genesis file")
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		fail("create OP Geth data directory: " + err.Error())
	}
	genesisHash := hashGenesis(genesis)
	jwtPath := filepath.Join(dataDir, "jwt.hex")
	if err := os.WriteFile(jwtPath, []byte(jwt+"\n"), 0600); err != nil {
		fail("write OP Geth JWT: " + err.Error())
	}
	marker := filepath.Join(dataDir, "chaimir-genesis-initialized")
	if _, err := os.Stat(marker); os.IsNotExist(err) {
		run("geth", "init", "--state.scheme", "hash", "--datadir", dataDir, genesis)
		if err := os.WriteFile(marker, []byte(genesisHash+"\n"), 0600); err != nil {
			fail("write OP Geth initialization marker: " + err.Error())
		}
	} else if err != nil {
		fail("check OP Geth initialization marker: " + err.Error())
	} else {
		markerContents, err := os.ReadFile(marker)
		if err != nil {
			fail("read OP Geth initialization marker: " + err.Error())
		}
		if strings.TrimSpace(string(markerContents)) != genesisHash {
			fail("OP_GETH_GENESIS_PATH does not match the initialized runtime state")
		}
	}
	// 受控链没有外部同步对等端,必须用完整执行模式接收 Beacon 通过 Engine API 提交的受控 payload。
	// The Graph 等索引器会按事件所在区块执行历史 eth_call,因此执行层必须保留历史状态。
	run("geth", "--syncmode", "full", "--gcmode", "archive", "--state.scheme", "hash", "--datadir", dataDir, "--http", "--http.addr", "0.0.0.0", "--http.port", strconv.Itoa(rpcPort), "--http.api", "eth,net,web3,debug,txpool", "--http.vhosts", "*", "--authrpc.addr", "0.0.0.0", "--authrpc.port", strconv.Itoa(enginePort), "--authrpc.vhosts", "*", "--authrpc.jwtsecret", jwtPath)
}

func required(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		fail(name + " is required")
	}
	return value
}

func requiredPort(name string) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		fail(name + " is required")
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 || parsed > 65535 {
		fail(name + " must be a valid TCP port")
	}
	return parsed
}

// hashGenesis 对有界的挂载 genesis 输入计算指纹,避免持久化运行态被不同链定义静默复用。
func hashGenesis(path string) string {
	file, err := os.Open(path)
	if err != nil {
		fail("open OP Geth genesis: " + err.Error())
	}
	// 受控 OP-Stack 的官方 L2 allocs 会使 genesis 超过普通教学链大小；仍保留有界读取，避免异常挂载造成无界内存消耗。
	const maxGenesisBytes = 64 * 1024 * 1024
	contents, err := io.ReadAll(io.LimitReader(file, maxGenesisBytes+1))
	if closeErr := file.Close(); closeErr != nil {
		fail("close OP Geth genesis: " + closeErr.Error())
	}
	if err != nil {
		fail("read OP Geth genesis: " + err.Error())
	}
	if len(contents) > maxGenesisBytes {
		fail("OP_GETH_GENESIS_PATH exceeds the maximum supported size")
	}
	hash := sha256.Sum256(contents)
	return hex.EncodeToString(hash[:])
}

func run(name string, args ...string) {
	command := exec.Command(name, args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		fail(fmt.Sprintf("%s failed: %v", name, err))
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}
