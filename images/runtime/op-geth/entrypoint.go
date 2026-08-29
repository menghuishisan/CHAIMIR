// entrypoint.go validates the platform runtime contract, initializes the
// rollup genesis once, and starts the OP Geth execution and Engine APIs.
package main

import (
	"encoding/hex"
	"fmt"
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
	enginePort := port("OP_GETH_ENGINE_PORT", 8551)
	rpcPort := port("OP_GETH_RPC_PORT", 8545)
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
	jwtPath := filepath.Join(dataDir, "jwt.hex")
	if err := os.WriteFile(jwtPath, []byte(jwt+"\n"), 0600); err != nil {
		fail("write OP Geth JWT: " + err.Error())
	}
	marker := filepath.Join(dataDir, "chaimir-genesis-initialized")
	if _, err := os.Stat(marker); os.IsNotExist(err) {
		run("geth", "init", "--datadir", dataDir, genesis)
		if err := os.WriteFile(marker, []byte("initialized\n"), 0600); err != nil {
			fail("write OP Geth initialization marker: " + err.Error())
		}
	} else if err != nil {
		fail("check OP Geth initialization marker: " + err.Error())
	}
	// Engine API is reached through the generated Service DNS name; allow that
	// Host header while keeping JWT authentication as the access control.
	run("geth", "--datadir", dataDir, "--http", "--http.addr", "0.0.0.0", "--http.port", strconv.Itoa(rpcPort), "--http.api", "eth,net,web3,debug,txpool", "--http.vhosts", "*", "--authrpc.addr", "0.0.0.0", "--authrpc.port", strconv.Itoa(enginePort), "--authrpc.vhosts", "*", "--authrpc.jwtsecret", jwtPath)
}

func required(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		fail(name + " is required")
	}
	return value
}

func port(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 || parsed > 65535 {
		fail(name + " must be a valid TCP port")
	}
	return parsed
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
