// 本文件校验 Chainlink 运行时绑定，生成官方配置文件并交接给原生节点。
package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/pelletier/go-toml/v2"
)

const defaultRuntimeRoot = "/runtime-state/chainlink"

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	decimalPattern    = regexp.MustCompile(`^[0-9]+$`)
)

func main() {
	if passthrough() {
		return
	}
	values := map[string]string{}
	for _, name := range []string{
		"ETH_URL", "CHAIMIR_CHAINLINK_DATABASE_ADDRESS", "CHAIMIR_CHAINLINK_DATABASE_NAME",
		"CHAIMIR_CHAINLINK_DATABASE_USER", "CHAINLINK_CHAIN_ID", "CHAINLINK_GAS_PRICE_MAX_WEI",
		"CHAINLINK_VRF_GAS_LIMIT", "POSTGRES_PASSWORD", "CHAINLINK_KEYSTORE_PASSWORD",
		"CHAINLINK_API_EMAIL", "CHAINLINK_API_PASSWORD", "CHAINLINK_TLS_CERT_B64", "CHAINLINK_TLS_KEY_B64",
	} {
		values[name] = required(name)
	}
	validate(values)
	cert, key := decodeTLS(values["CHAINLINK_TLS_CERT_B64"], values["CHAINLINK_TLS_KEY_B64"])

	root := defaultRuntimeRoot
	tlsDir := root + "/tls"
	if err := os.MkdirAll(tlsDir, 0o700); err != nil {
		fail("create Chainlink runtime state: " + err.Error())
	}
	if err := os.Chmod(root, 0o700); err != nil {
		fail("secure Chainlink runtime state: " + err.Error())
	}
	if err := os.Chmod(tlsDir, 0o700); err != nil {
		fail("secure Chainlink TLS state: " + err.Error())
	}
	certPath, keyPath := tlsDir+"/server.crt", tlsDir+"/server.key"
	writePrivate(certPath, cert)
	writePrivate(keyPath, key)

	configPath, secretsPath, apiPath := root+"/config.toml", root+"/secrets.toml", root+"/api-credentials"
	config := makeNodeConfig(values, root, certPath, keyPath)
	secrets := makeSecrets(values)
	writeTOML(configPath, config)
	writeTOML(secretsPath, secrets)
	writePrivate(apiPath, []byte(values["CHAINLINK_API_EMAIL"]+"\n"+values["CHAINLINK_API_PASSWORD"]+"\n"))

	args := []string{"/usr/local/bin/chainlink", "--config", configPath, "--secrets", secretsPath, "node", "start", "--api", apiPath}
	env := filteredEnvironment()
	execChainlink(args, env)
}

func passthrough() bool {
	if len(os.Args) < 2 {
		return false
	}
	args := append([]string{"/usr/local/bin/chainlink"}, os.Args[1:]...)
	if os.Args[1] == "chainlink" {
		args = append([]string{"/usr/local/bin/chainlink"}, os.Args[2:]...)
	}
	if os.Args[1] != "--version" && os.Args[1] != "chainlink" {
		return false
	}
	execChainlink(args, filteredEnvironment())
	return true
}

func execChainlink(args, env []string) {
	if err := syscall.Exec(args[0], args, env); err != nil {
		fail("start Chainlink: " + err.Error())
	}
}

func validate(v map[string]string) {
	requireHTTPURL("ETH_URL", v["ETH_URL"])
	if !identifierPattern.MatchString(v["CHAIMIR_CHAINLINK_DATABASE_NAME"]) || !identifierPattern.MatchString(v["CHAIMIR_CHAINLINK_DATABASE_USER"]) {
		fail("Chainlink database name or user is invalid")
	}
	databaseAddress(v["CHAIMIR_CHAINLINK_DATABASE_ADDRESS"])
	positiveBigInt(v["CHAINLINK_CHAIN_ID"], "CHAINLINK_CHAIN_ID")
	positiveBigInt(v["CHAINLINK_GAS_PRICE_MAX_WEI"], "CHAINLINK_GAS_PRICE_MAX_WEI")
	positiveUint(v["CHAINLINK_VRF_GAS_LIMIT"], 32, "CHAINLINK_VRF_GAS_LIMIT")
	for _, name := range []string{"POSTGRES_PASSWORD", "CHAINLINK_KEYSTORE_PASSWORD", "CHAINLINK_API_PASSWORD"} {
		password := v[name]
		if len(password) < 16 || len(password) > 50 {
			fail(name + " length must be in 16-50")
		}
	}
	if strings.ContainsAny(v["CHAINLINK_API_PASSWORD"], "\r\n") {
		fail("CHAINLINK_API_PASSWORD must not contain line endings")
	}
	address, err := mail.ParseAddress(v["CHAINLINK_API_EMAIL"])
	if err != nil || address.Address != v["CHAINLINK_API_EMAIL"] || strings.ContainsAny(v["CHAINLINK_API_EMAIL"], "\r\n") {
		fail("CHAINLINK_API_EMAIL is invalid")
	}
}

func requireHTTPURL(name, raw string) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Hostname() == "" || u.Path != "" || u.ForceQuery || u.RawQuery != "" || strings.Contains(raw, "#") {
		fail(name + " is invalid")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		fail(name + " port must be in 1-65535")
	}
}

func databaseAddress(raw string) string {
	u, err := url.Parse("postgresql://" + raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Path != "" || u.ForceQuery || u.RawQuery != "" || strings.Contains(raw, "#") {
		fail("CHAIMIR_CHAINLINK_DATABASE_ADDRESS is invalid")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		fail("CHAIMIR_CHAINLINK_DATABASE_ADDRESS port must be in 1-65535")
	}
	return u.Host
}

func positiveUint(raw string, bits int, name string) uint64 {
	n, err := strconv.ParseUint(raw, 10, bits)
	if err != nil || n == 0 || !decimalPattern.MatchString(raw) {
		fail(name + " must be a positive integer")
	}
	return n
}

func positiveBigInt(raw, name string) *big.Int {
	n, ok := new(big.Int).SetString(raw, 10)
	if !ok || n.Sign() <= 0 || !decimalPattern.MatchString(raw) {
		fail(name + " must be a positive integer")
	}
	return n
}

func decodeTLS(certValue, keyValue string) ([]byte, []byte) {
	certPEM, err := base64.StdEncoding.DecodeString(certValue)
	if err != nil {
		fail("Chainlink TLS certificate is not valid Base64")
	}
	keyPEM, err := base64.StdEncoding.DecodeString(keyValue)
	if err != nil {
		fail("Chainlink TLS private key is not valid Base64")
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		fail("Chainlink TLS certificate or private key is invalid or mismatched")
	}
	certificates := make([]*x509.Certificate, 0, len(pair.Certificate))
	for _, der := range pair.Certificate {
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			fail("Chainlink TLS certificate chain is invalid")
		}
		certificates = append(certificates, cert)
	}
	// 受控 Secret 是唯一信任来源，验证不依赖容器系统 CA 或网络。
	roots, intermediates := x509.NewCertPool(), x509.NewCertPool()
	roots.AddCert(certificates[len(certificates)-1])
	for _, cert := range certificates[1:] {
		intermediates.AddCert(cert)
	}
	if _, err := certificates[0].Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates, DNSName: "chainlink-node", KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
		fail("Chainlink TLS certificate validity, host or server purpose is invalid")
	}
	return certPEM, keyPEM
}

func makeNodeConfig(v map[string]string, root, certPath, keyPath string) map[string]any {
	vrf := positiveUint(v["CHAINLINK_VRF_GAS_LIMIT"], 32, "CHAINLINK_VRF_GAS_LIMIT")
	return map[string]any{
		"RootDir": root,
		"Log":     map[string]any{"Level": "info"},
		"Feature": map[string]any{"FeedsManager": true, "LogPoller": true, "UICSAKeys": true},
		"OCR2":    map[string]any{"Enabled": true},
		"P2P": map[string]any{"V2": map[string]any{
			"Enabled": true, "ListenAddresses": []string{"0.0.0.0:6690"}, "AnnounceAddresses": []string{"0.0.0.0:6690"},
		}},
		"WebServer": map[string]any{
			"HTTPPort": 0, "SecureCookies": true,
			"TLS": map[string]any{"CertPath": certPath, "KeyPath": keyPath, "HTTPSPort": 6688, "ForceRedirect": false},
		},
		"EVM": []map[string]any{{
			"ChainID": positiveBigInt(v["CHAINLINK_CHAIN_ID"], "CHAINLINK_CHAIN_ID").String(),
			"Enabled": true, "AutoCreateKey": true, "MinContractPayment": 0, "BlockBackfillDepth": 100,
			// HTTP binding 通过 LogPoller 轮询；旧 LogBroadcaster 需要未提供的 WS 订阅。
			"FinalityDepth": 1, "FinalityTagEnabled": false, "LogBroadcasterEnabled": false,
			"GasEstimator": map[string]any{
				"PriceMax":     positiveBigInt(v["CHAINLINK_GAS_PRICE_MAX_WEI"], "CHAINLINK_GAS_PRICE_MAX_WEI").String() + " wei",
				"LimitJobType": map[string]any{"VRF": uint32(vrf)},
			},
			"NodePool": map[string]any{"NewHeadsPollInterval": "1s"},
			"Nodes":    []map[string]any{{"Name": "execution-chain", "HTTPURL": v["ETH_URL"]}},
		}},
	}
}

func makeSecrets(v map[string]string) map[string]any {
	databaseURL := url.URL{Scheme: "postgresql", Host: databaseAddress(v["CHAIMIR_CHAINLINK_DATABASE_ADDRESS"]), Path: "/" + v["CHAIMIR_CHAINLINK_DATABASE_NAME"], User: url.UserPassword(v["CHAIMIR_CHAINLINK_DATABASE_USER"], v["POSTGRES_PASSWORD"]), RawQuery: "sslmode=require"}
	return map[string]any{
		"Database": map[string]any{"URL": databaseURL.String()},
		"Password": map[string]any{"Keystore": v["CHAINLINK_KEYSTORE_PASSWORD"]},
	}
}

func writeTOML(path string, value any) {
	data, err := toml.Marshal(value)
	if err != nil {
		fail("encode Chainlink configuration: " + err.Error())
	}
	writePrivate(path, data)
}

func writePrivate(path string, data []byte) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		fail("open private runtime file: " + err.Error())
	}
	err = file.Chmod(0o600)
	if err == nil {
		_, err = file.Write(data)
	}
	if err := errors.Join(err, file.Close()); err != nil {
		fail("write private runtime file: " + err.Error())
	}
}

func filteredEnvironment() []string {
	blocked := map[string]bool{"ETH_URL": true, "POSTGRES_PASSWORD": true, "CHAINLINK_KEYSTORE_PASSWORD": true, "CHAINLINK_API_EMAIL": true, "CHAINLINK_API_PASSWORD": true, "CHAINLINK_TLS_CERT_B64": true, "CHAINLINK_TLS_KEY_B64": true}
	result := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !blocked[key] {
			result = append(result, entry)
		}
	}
	return result
}

func required(name string) string {
	value := os.Getenv(name)
	if value == "" {
		fail(name + " is required")
	}
	return value
}

func fail(message string) {
	if _, err := fmt.Fprintln(os.Stderr, message); err != nil {
		os.Exit(2)
	}
	os.Exit(2)
}
