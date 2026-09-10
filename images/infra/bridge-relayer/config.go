// 本文件把组合注入的两条 RPC 绑定写入 Hyperlane 官方 JSON 配置覆盖文件。
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var evmAddressPattern = regexp.MustCompile(`^(0x|0X)?[0-9a-fA-F]{40}$`)

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
	if relayChains[0] == relayChains[1] {
		fail("HYP_RELAYCHAINS must contain two different chain names")
	}
	rpcURLs := []string{
		requiredRPC("HYP_SOURCE_RPC"),
		requiredRPC("HYP_DESTINATION_RPC"),
	}

	contents, err := os.ReadFile(basePath)
	if err != nil {
		fail("read Hyperlane base config: " + err.Error())
	}
	var root map[string]any
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		fail("parse Hyperlane base config: " + err.Error())
	}
	defaultSignerKey := requiredSignerKey()
	chains, ok := root["chains"].(map[string]any)
	if !ok {
		fail("Hyperlane base config must contain an object named chains")
	}
	for index, name := range relayChains {
		chain, ok := chains[name].(map[string]any)
		if !ok {
			fail("Hyperlane base config has no chain named " + name)
		}
		validateEVMChain(name, chain)
		chain["rpcUrls"] = []any{map[string]any{"http": rpcURLs[index]}}
	}
	root["defaultSigner"] = map[string]any{
		"type": "hexKey",
		"key":  defaultSignerKey,
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

// validateEVMChain rejects an incomplete Hyperlane deployment before its
// runtime override becomes visible to the official relayer. The bridge image
// consumes the pinned agent's EVM metadata format, whose protocol material is
// supplied by the controlled base config rather than generated here.
func validateEVMChain(name string, chain map[string]any) {
	requireExactStringField(name, chain, "name")
	protocol, ok := chain["protocol"].(string)
	if !ok || protocol != "ethereum" {
		fail("Hyperlane chain " + name + " must declare protocol=ethereum")
	}
	requirePositiveDomain(name, chain["domainId"])
	for _, addressName := range []string{"mailbox", "interchainGasPaymaster", "validatorAnnounce", "merkleTreeHook"} {
		requireEVMAddress(name, addressName, chain[addressName])
	}
}

func requireExactStringField(chainName string, chain map[string]any, fieldName string) {
	value, ok := chain[fieldName].(string)
	if !ok || value == "" || value != strings.TrimSpace(value) {
		fail("Hyperlane chain " + chainName + " must contain a string field " + fieldName)
	}
}

func requirePositiveDomain(chainName string, value any) {
	var domain int64
	switch parsed := value.(type) {
	case json.Number:
		parsedDomain, err := parsed.Int64()
		if err != nil {
			fail("Hyperlane chain " + chainName + " domainId must be a positive integer")
		}
		domain = parsedDomain
	case string:
		parsedDomain, err := strconv.ParseInt(strings.TrimSpace(parsed), 10, 64)
		if err != nil {
			fail("Hyperlane chain " + chainName + " domainId must be a positive integer")
		}
		domain = parsedDomain
	default:
		fail("Hyperlane chain " + chainName + " must contain domainId")
	}
	if domain < 1 {
		fail("Hyperlane chain " + chainName + " domainId must be a positive integer")
	}
}

func requireEVMAddress(chainName, fieldName string, value any) {
	address, ok := value.(string)
	if !ok || address != strings.TrimSpace(address) || !evmAddressPattern.MatchString(address) {
		fail("Hyperlane chain " + chainName + " must contain a valid " + fieldName + " EVM address")
	}
	if strings.TrimLeft(strings.TrimPrefix(strings.TrimPrefix(address, "0x"), "0X"), "0") == "" {
		fail("Hyperlane chain " + chainName + " " + fieldName + " must not be the zero address")
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

// requiredSignerKey validates the Secret before it is copied into the
// runtime-only Hyperlane configuration consumed by the official agent.
func requiredSignerKey() string {
	value := requiredEnv("HYP_DEFAULTSIGNER_KEY")
	hexValue := strings.TrimPrefix(strings.TrimPrefix(value, "0x"), "0X")
	if len(hexValue) != 64 {
		fail("HYP_DEFAULTSIGNER_KEY must contain exactly 32 bytes of hexadecimal data")
	}
	if _, err := hex.DecodeString(hexValue); err != nil {
		fail("HYP_DEFAULTSIGNER_KEY must contain exactly 32 bytes of hexadecimal data")
	}
	return value
}

// requiredRPC validates a binding endpoint before it is written into the
// official Hyperlane configuration. The port is part of the binding contract.
func requiredRPC(name string) string {
	value := requiredEnv(name)
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.Port() == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		fail(name + " must be an HTTP(S) URL with an explicit port")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		fail(name + " must use a TCP port in 1-65535")
	}
	return value
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}
