// 本文件校验 source/destination 链材料与 RPC 合约状态,再生成 Hyperlane 运行期配置。
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	mailboxLocalDomainCallData = "0x8d3638f4"
	rpcResponseMaxBytes        = 1 << 20
	maxHyperlaneDomain         = uint64(1<<32 - 1)
)

var (
	evmAddressPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
	rpcClient         = &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{Proxy: nil},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
)

// main 生成完整的运行期配置文件,不修改平台提供的只读 base config。
func main() {
	basePath := requiredEnv("HYP_BASE_CONFIG")
	relayChains := splitCSV(requiredEnv("HYP_RELAYCHAINS"))
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
	if err := rejectDuplicateJSONKeys(contents); err != nil {
		fail("invalid Hyperlane base config: " + err.Error())
	}
	var root map[string]any
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		fail("parse Hyperlane base config: " + err.Error())
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		fail("Hyperlane base config must contain exactly one JSON document")
	}
	defaultSignerKey := requiredSignerKey()
	chains, ok := root["chains"].(map[string]any)
	if !ok {
		fail("Hyperlane base config must contain an object named chains")
	}
	selected := make([]chainConfig, 2)
	domainOwners := make(map[uint32]string, 2)
	for index, name := range relayChains {
		chain, ok := chains[name].(map[string]any)
		if !ok {
			fail("Hyperlane base config has no chain named " + name)
		}
		chainID, domainID, addresses := validateEVMChain(name, chain)
		if owner, exists := domainOwners[domainID]; exists {
			fail("Hyperlane source and destination domains must be distinct: " + owner + " and " + name)
		}
		domainOwners[domainID] = name
		selected[index] = chainConfig{name: name, rpcURL: rpcURLs[index], chainID: chainID, domainID: domainID, addresses: addresses, value: chain}
	}
	for _, binding := range selected {
		validateBoundChain(binding)
		binding.value["rpcUrls"] = []any{map[string]any{"http": binding.rpcURL}}
	}
	root["defaultSigner"] = map[string]any{
		"type": "hexKey",
		"key":  defaultSignerKey,
	}

	outputPath := "/tmp/chaimir-hyperlane/merged-config.json"
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
	if err := os.Chmod(outputPath, 0600); err != nil {
		fail("secure Hyperlane runtime config: " + err.Error())
	}
}

// chainConfig 保存一个已绑定到 RPC endpoint 的受控 Hyperlane 链配置。
type chainConfig struct {
	name      string
	rpcURL    string
	chainID   uint64
	domainID  uint32
	addresses map[string]string
	value     map[string]any
}

// validateEVMChain 在官方中继器读取运行时覆盖前拒绝不完整的 Hyperlane 部署。
// 桥接镜像消费固定 agent 的 EVM 元数据格式,协议材料必须来自受控 base config,
// 不能在此处生成。
func validateEVMChain(name string, chain map[string]any) (uint64, uint32, map[string]string) {
	if requireExactStringField(name, chain, "name") != name {
		fail("Hyperlane chain name must match its source/destination binding")
	}
	protocol, ok := chain["protocol"].(string)
	if !ok || protocol != "ethereum" {
		fail("Hyperlane chain " + name + " must declare protocol=ethereum")
	}
	chainID := requirePositiveInteger(name, "chainId", chain["chainId"])
	domain := requirePositiveInteger(name, "domainId", chain["domainId"])
	if domain > maxHyperlaneDomain {
		fail("Hyperlane chain " + name + " domainId must fit uint32")
	}
	addresses := make(map[string]string, 4)
	for _, addressName := range []string{"mailbox", "interchainGasPaymaster", "validatorAnnounce", "merkleTreeHook"} {
		addresses[addressName] = requireEVMAddress(name, addressName, chain[addressName])
	}
	return chainID, uint32(domain), addresses
}

func requireExactStringField(chainName string, chain map[string]any, fieldName string) string {
	value, ok := chain[fieldName].(string)
	if !ok || value == "" || value != strings.TrimSpace(value) {
		fail("Hyperlane chain " + chainName + " must contain a string field " + fieldName)
	}
	return value
}

func requirePositiveInteger(chainName, fieldName string, value any) uint64 {
	number, ok := value.(json.Number)
	if !ok {
		fail("Hyperlane chain " + chainName + " must contain numeric " + fieldName)
	}
	parsed, err := strconv.ParseUint(number.String(), 10, 64)
	if err != nil || parsed == 0 {
		fail("Hyperlane chain " + chainName + " " + fieldName + " must be a positive integer")
	}
	return parsed
}

func requireEVMAddress(chainName, fieldName string, value any) string {
	address, ok := value.(string)
	if !ok || address != strings.TrimSpace(address) || !evmAddressPattern.MatchString(address) {
		fail("Hyperlane chain " + chainName + " must contain a valid " + fieldName + " EVM address")
	}
	decoded, err := hex.DecodeString(address[2:])
	if err != nil {
		fail("Hyperlane chain " + chainName + " must contain a valid " + fieldName + " EVM address")
	}
	if allZero(decoded) {
		fail("Hyperlane chain " + chainName + " " + fieldName + " must not be the zero address")
	}
	return address
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part != strings.TrimSpace(part) {
			fail("HYP_RELAYCHAINS contains a blank or whitespace-padded chain name")
		}
		result = append(result, part)
	}
	return result
}

func requiredEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		fail(name + " is required")
	}
	if value != strings.TrimSpace(value) {
		fail(name + " must not contain leading or trailing whitespace")
	}
	return value
}

// requiredSignerKey 在把 Secret 写入官方 agent 消费的运行态 Hyperlane 配置前完成校验。
func requiredSignerKey() string {
	value := requiredEnv("HYP_DEFAULTSIGNER_KEY")
	if !strings.HasPrefix(value, "0x") {
		fail("HYP_DEFAULTSIGNER_KEY must use the 0x-prefixed hex format required by Hyperlane")
	}
	hexValue := strings.TrimPrefix(value, "0x")
	if len(hexValue) != 64 || strings.Trim(hexValue, "0") == "" {
		fail("HYP_DEFAULTSIGNER_KEY must contain a non-zero 32-byte hexadecimal private key")
	}
	if _, err := hex.DecodeString(hexValue); err != nil {
		fail("HYP_DEFAULTSIGNER_KEY must contain a non-zero 32-byte hexadecimal private key")
	}
	return value
}

// requiredRPC 在把 binding endpoint 写入官方 Hyperlane 配置前完成校验。
// 端口属于 binding 契约的一部分。
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

// validateBoundChain 将声明的链 ID/domain 和合约地址绑定到对应 RPC 的真实链状态。
func validateBoundChain(binding chainConfig) {
	chainID := rpcQuantity(binding.rpcURL, "eth_chainId", []any{})
	if chainID != binding.chainID {
		fail("Hyperlane chain " + binding.name + " chainId does not match its RPC binding")
	}
	domain := rpcMailboxDomain(binding.rpcURL, binding.addresses["mailbox"])
	if domain != binding.domainID {
		fail("Hyperlane chain " + binding.name + " domainId does not match its Mailbox")
	}
	for _, fieldName := range []string{"mailbox", "interchainGasPaymaster", "validatorAnnounce", "merkleTreeHook"} {
		code := rpcString(binding.rpcURL, "eth_getCode", []any{binding.addresses[fieldName], "latest"})
		if !hasDeployedCode(code) {
			fail("Hyperlane chain " + binding.name + " " + fieldName + " has no deployed contract code")
		}
	}
}

// rpcQuantity 查询 RPC quantity,只接受规范的十六进制 JSON-RPC 返回值。
func rpcQuantity(endpoint, method string, params any) uint64 {
	value := rpcString(endpoint, method, params)
	if !strings.HasPrefix(value, "0x") || len(value) < 3 || len(value) > 18 || (len(value) > 3 && value[2] == '0') {
		fail("Hyperlane RPC returned an invalid " + method + " quantity")
	}
	parsed, err := strconv.ParseUint(value[2:], 16, 64)
	if err != nil {
		fail("Hyperlane RPC returned an invalid " + method + " quantity")
	}
	return parsed
}

// rpcMailboxDomain 读取 Hyperlane Mailbox 的 uint32 localDomain 公共 getter。
func rpcMailboxDomain(endpoint, mailbox string) uint32 {
	value := rpcString(endpoint, "eth_call", []any{map[string]string{"to": mailbox, "data": mailboxLocalDomainCallData}, "latest"})
	if !strings.HasPrefix(value, "0x") || len(value) != 66 {
		fail("Hyperlane Mailbox returned an invalid localDomain value")
	}
	decoded, err := hex.DecodeString(value[2:])
	if err != nil {
		fail("Hyperlane Mailbox returned an invalid localDomain value")
	}
	if !allZero(decoded[:28]) {
		fail("Hyperlane Mailbox localDomain is outside uint32 range")
	}
	return binary.BigEndian.Uint32(decoded[28:])
}

// rpcString 执行有超时和响应大小限制的 JSON-RPC 请求,任何协议错误均 fail closed。
func rpcString(endpoint, method string, params any) string {
	payload, err := json.Marshal(struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		fail("encode Hyperlane RPC request: " + err.Error())
	}
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		fail("create Hyperlane RPC request: " + err.Error())
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := rpcClient.Do(request)
	if err != nil {
		fail("Hyperlane RPC request failed for " + method)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		fail("Hyperlane RPC returned a non-success HTTP status for " + method)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, rpcResponseMaxBytes+1))
	if err != nil || len(body) > rpcResponseMaxBytes {
		fail("Hyperlane RPC response is unreadable or too large for " + method)
	}
	if err := rejectDuplicateJSONKeys(body); err != nil {
		fail("Hyperlane RPC response contains invalid JSON: " + err.Error())
	}
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&envelope); err != nil {
		fail("parse Hyperlane RPC response for " + method)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		fail("Hyperlane RPC response must contain one JSON object for " + method)
	}
	if envelope.JSONRPC != "2.0" || string(envelope.ID) != "1" || envelope.Error != nil || len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		fail("Hyperlane RPC returned an invalid response for " + method)
	}
	var value string
	if err := json.Unmarshal(envelope.Result, &value); err != nil || value == "" {
		fail("Hyperlane RPC returned an invalid result for " + method)
	}
	return value
}

// hasDeployedCode 拒绝缺失、非法和全零的 eth_getCode 返回值。
func hasDeployedCode(value string) bool {
	if !strings.HasPrefix(value, "0x") || len(value) <= 2 || len(value)%2 != 0 {
		return false
	}
	code, err := hex.DecodeString(value[2:])
	return err == nil && !allZero(code)
}

// allZero 判断字节序列是否全为零。
func allZero(value []byte) bool {
	for _, b := range value {
		if b != 0 {
			return false
		}
	}
	return true
}

// rejectDuplicateJSONKeys 在对象键被 map 解码覆盖前递归检查重复键和尾随 JSON。
func rejectDuplicateJSONKeys(contents []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("base config must contain exactly one JSON document")
	}
	return nil
}

// scanJSONValue 按 JSON token 遍历值并验证每个对象的键唯一。
func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		keys := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if _, exists := keys[key]; exists {
				return fmt.Errorf("duplicate JSON object key %q", key)
			}
			keys[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("malformed JSON object")
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return fmt.Errorf("malformed JSON array")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter")
	}
	return nil
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}
