// 本文件实现 The Graph 入口,校验组合绑定并转换为 graph-node 原生环境变量。
package main

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	userPattern       = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

func main() {
	rpc := required("CHAIMIR_GRAPH_RPC_URL")
	databaseAddress := required("CHAIMIR_GRAPH_POSTGRES_ADDRESS")
	ipfs := required("CHAIMIR_GRAPH_IPFS_ADDRESS")
	password := required("POSTGRES_PASSWORD")
	poiAccessToken := required("GRAPH_POI_ACCESS_TOKEN")
	network := required("CHAIMIR_GRAPH_NETWORK")
	database := required("CHAIMIR_GRAPH_POSTGRES_DB")
	user := required("CHAIMIR_GRAPH_POSTGRES_USER")
	if !identifierPattern.MatchString(network) || !identifierPattern.MatchString(database) || !userPattern.MatchString(user) || strings.TrimSpace(poiAccessToken) != poiAccessToken || strings.ContainsAny(poiAccessToken, "\r\n") {
		fail("graph binding contains invalid characters")
	}
	requireHTTPURL("CHAIMIR_GRAPH_RPC_URL", rpc)
	databaseHostPort, ok := normalizeDatabaseAddress(databaseAddress)
	if !ok {
		fail("CHAIMIR_GRAPH_POSTGRES_ADDRESS is invalid")
	}
	requireHTTPURL("CHAIMIR_GRAPH_IPFS_ADDRESS", ipfs)
	databaseURL := url.URL{Scheme: "postgresql", Host: databaseHostPort, Path: "/" + database, User: url.UserPassword(user, password), RawQuery: "sslmode=require"}
	for key, value := range map[string]string{
		"POSTGRES_URL": databaseURL.String(),
		"ETHEREUM_RPC": network + ":" + rpc,
		"IPFS":         ipfs,
	} {
		if err := os.Setenv(key, value); err != nil {
			fail("set graph-node " + key + ": " + err.Error())
		}
	}
	if err := os.Unsetenv("POSTGRES_PASSWORD"); err != nil {
		fail("remove raw database password: " + err.Error())
	}
	args := append([]string{"/usr/local/bin/graph-node"}, os.Args[1:]...)
	if err := syscall.Exec(args[0], args, os.Environ()); err != nil {
		fail("exec graph-node: " + err.Error())
	}
}

// normalizeDatabaseAddress 只接受 TCP binding 的主机与显式端口，不接收连接 URL。
func normalizeDatabaseAddress(raw string) (string, bool) {
	parsed, err := url.Parse("postgresql://" + raw)
	if err != nil || parsed.Hostname() == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.ForceQuery || strings.Contains(raw, "#") || parsed.User != nil {
		return "", false
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return "", false
	}
	return parsed.Host, true
}

// requireHTTPURL 在 graph-node 解析前强制执行 HTTP binding 契约。
func requireHTTPURL(name, raw string) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || strings.Contains(raw, "#") {
		fail(name + " is invalid")
	}
	portNumber, err := strconv.Atoi(parsed.Port())
	if err != nil || portNumber < 1 || portNumber > 65535 {
		fail(name + " port must be in 1-65535")
	}
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
