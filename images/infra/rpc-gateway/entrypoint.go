// RPC 网关入口将组合地址编译为 Envoy 原生配置，再交接进程。
package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"syscall"
	"text/template"
)

//go:embed envoy.yaml
var configTemplate string

type upstream struct {
	Host    string
	Port    uint16
	TLS     bool
	SANType string
	SNI     string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

// run 只生成本次启动配置；成功后由 Envoy 接管信号和退出状态。
func run(args []string) (runErr error) {
	argv := []string{"envoy"}
	if len(args) != 1 || args[0] != "--version" {
		targets, err := parseUpstreams(os.Getenv("CHAIMIR_RPC_UPSTREAM_URLS"))
		if err != nil {
			return err
		}
		config, err := template.New("envoy").Funcs(template.FuncMap{"quote": strconv.Quote}).Parse(configTemplate)
		if err != nil {
			return fmt.Errorf("解析网关配置模板: %w", err)
		}
		file, err := os.CreateTemp("/tmp", "chaimir-envoy-*.yaml")
		if err != nil {
			return fmt.Errorf("创建网关临时配置: %w", err)
		}
		defer func() { runErr = errors.Join(runErr, os.Remove(file.Name())) }()
		writeErr := config.Execute(file, targets)
		if err := errors.Join(writeErr, file.Close()); err != nil {
			return fmt.Errorf("写入网关配置: %w", err)
		}
		argv = append(argv, "-c", file.Name())
	}
	return syscall.Exec("/usr/local/bin/envoy", append(argv, args...), os.Environ())
}

// parseUpstreams 对完整目标列表一次校验，不允许丢弃失败项后启动残缺网关。
func parseUpstreams(raw string) ([]upstream, error) {
	var addresses []string
	if err := json.Unmarshal([]byte(raw), &addresses); err != nil || len(addresses) == 0 {
		return nil, errors.New("CHAIMIR_RPC_UPSTREAM_URLS 必须是非空地址数组")
	}
	targets := make([]upstream, 0, len(addresses))
	seen := map[upstream]struct{}{}
	for _, address := range addresses {
		target, err := parseUpstream(address)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[target]; duplicate {
			return nil, errors.New("RPC 上游地址重复")
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}
	return targets, nil
}

// parseUpstream 拒绝 URL 装饰，避免受控 Service 地址被解释为另一目标。
func parseUpstream(raw string) (upstream, error) {
	invalid := errors.New("RPC 上游必须是带显式端口、无凭据或附加部分的 HTTP(S) 地址")
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Opaque != "" || u.Path != "" || strings.ContainsAny(raw, "?#") {
		return upstream{}, invalid
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil || strings.Contains(host, "%") || port == "" || strings.Trim(port, "0123456789") != "" {
		return upstream{}, invalid
	}
	number, err := strconv.ParseUint(port, 10, 16)
	if err != nil || number == 0 {
		return upstream{}, invalid
	}
	target := upstream{Host: strings.ToLower(host), Port: uint16(number), TLS: u.Scheme == "https", SANType: "IP_ADDRESS"}
	if ip := net.ParseIP(host); ip != nil {
		target.Host = ip.String()
	} else {
		if !validDNSName(host) {
			return upstream{}, invalid
		}
		target.SANType = "DNS"
		target.SNI = target.Host
	}
	return target, nil
}

// validDNSName 只允许可作为 Service 或 TLS 身份的 ASCII 主机名。
func validDNSName(host string) bool {
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-') {
				return false
			}
		}
	}
	return true
}
