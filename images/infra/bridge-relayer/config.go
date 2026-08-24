// 本文件把组合注入的两条 RPC 绑定写入 Hyperlane 官方 JSON 配置覆盖文件。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// main 生成完整的运行期配置文件,不修改平台提供的只读 base config。
func main() {
	basePath := strings.TrimSpace(os.Getenv("HYP_BASE_CONFIG"))
	if basePath == "" {
		fail("HYP_BASE_CONFIG is required")
	}
	relayChains := splitCSV(os.Getenv("HYP_RELAYCHAINS"))
	if len(relayChains) != 2 {
		fail("HYP_RELAYCHAINS must contain exactly two chain names for source_chain and destination_chain")
	}
	rpcURLs := []string{
		requiredEnv("HYP_SOURCE_RPC"),
		requiredEnv("HYP_DESTINATION_RPC"),
	}

	contents, err := os.ReadFile(basePath)
	if err != nil {
		fail("read Hyperlane base config: " + err.Error())
	}
	var root map[string]any
	if err := json.Unmarshal(contents, &root); err != nil {
		fail("parse Hyperlane base config: " + err.Error())
	}
	chains, ok := root["chains"].(map[string]any)
	if !ok {
		fail("Hyperlane base config must contain an object named chains")
	}
	for index, name := range relayChains {
		chain, ok := chains[name].(map[string]any)
		if !ok {
			fail("Hyperlane base config has no chain named " + name)
		}
		chain["rpcUrls"] = []any{map[string]any{"http": rpcURLs[index]}}
	}

	outputPath := "/runtime-state/hyperlane/merged-config.json"
	if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		fail("create Hyperlane runtime config directory: " + err.Error())
	}
	updated, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		fail("encode Hyperlane runtime config: " + err.Error())
	}
	if err := os.WriteFile(outputPath, append(updated, '\n'), 0600); err != nil {
		fail("write Hyperlane runtime config: " + err.Error())
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			fail("HYP_RELAYCHAINS contains an empty chain name")
		}
		result = append(result, part)
	}
	return result
}

func requiredEnv(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		fail(name + " is required")
	}
	return value
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}
