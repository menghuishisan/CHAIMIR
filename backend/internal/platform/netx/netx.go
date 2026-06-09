// netx 提供出站网络配置的通用安全校验。
package netx

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// ValidatePublicHTTPURL 校验外部 HTTP(S) 端点,拒绝本机、私网、链路本地和格式非法地址。
func ValidatePublicHTTPURL(raw string) (string, error) {
	return validatePublicURL(raw, "HTTP", map[string]struct{}{"http": {}, "https": {}})
}

// ValidatePublicLDAPSURL 校验外部 LDAPS 端点,用于 SSO/LDAP 这类租户可配置的目录服务。
func ValidatePublicLDAPSURL(raw string) (string, error) {
	return validatePrivateCapableURL(raw, "LDAPS", map[string]struct{}{"ldaps": {}})
}

// validatePublicURL 按调用场景允许的 scheme 校验公网端点,统一阻断 SSRF 常见目标。
func validatePublicURL(raw, label string, schemes map[string]struct{}) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("外部%s端点 URL 格式非法", label)
	}
	if _, ok := schemes[parsed.Scheme]; !ok {
		return "", fmt.Errorf("外部%s端点协议不允许", label)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("外部%s端点不允许携带凭据", label)
	}
	host := parsed.Hostname()
	if strings.TrimSpace(host) == "" || isLocalHostname(host) {
		return "", fmt.Errorf("外部%s端点主机非法", label)
	}
	if addr, err := netip.ParseAddr(host); err == nil && !isPublicAddr(addr) {
		return "", fmt.Errorf("外部%s端点地址不允许指向内网或本机", label)
	}
	return parsed.String(), nil
}

// isLocalHostname 判断显式本机主机名,避免配置绕过公网出站边界。
func isLocalHostname(host string) bool {
	switch strings.ToLower(strings.TrimSuffix(host, ".")) {
	case "localhost":
		return true
	default:
		return false
	}
}

// isPublicAddr 使用 netip 标准地址属性拒绝 SSRF 常见目标网段。
func isPublicAddr(addr netip.Addr) bool {
	if addr.Is4In6() {
		addr = addr.Unmap()
	}
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return false
	}
	if isMetadataAddr(addr) {
		return false
	}
	return true
}

// isMetadataAddr 拒绝云平台元数据服务地址,即使它不属于 netip 的私网分类。
func isMetadataAddr(addr netip.Addr) bool {
	metadata := netip.MustParseAddr("169.254.169.254")
	return addr == metadata
}

// NewPublicHTTPClient 创建带公网出站限制的 HTTP client,用于用户/租户可配置的外部端点。
func NewPublicHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: PublicHTTPTransport(nil)}
}

// PublicHTTPTransport 返回带出站地址防护的 HTTP Transport,防止 DNS 解析后落到内网地址。
func PublicHTTPTransport(base *http.Transport) *http.Transport {
	if base == nil {
		base = http.DefaultTransport.(*http.Transport).Clone()
	} else {
		base = base.Clone()
	}
	base.Proxy = nil
	dialer := &net.Dialer{}
	base.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("解析出站地址失败: %w", err)
		}
		dialAddress, err := publicDialAddress(ctx, host, port)
		if err != nil {
			return nil, err
		}
		return dialer.DialContext(ctx, network, dialAddress)
	}
	return base
}

// PublicResolvedURL 把已校验的外部 URL 解析到公网地址,返回拨号 URL 与原始主机名。
// TLS 客户端应继续使用原始主机名做 ServerName,避免解析到 IP 后破坏证书校验。
func PublicResolvedURL(ctx context.Context, raw, defaultPort string) (resolvedURL string, serverName string, err error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", "", fmt.Errorf("外部端点 URL 格式非法")
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		port = strings.TrimSpace(defaultPort)
	}
	if port == "" {
		return "", "", fmt.Errorf("外部端点缺少端口")
	}
	dialAddress, err := publicDialAddress(ctx, host, port)
	if err != nil {
		return "", "", err
	}
	parsed.Host = dialAddress
	return parsed.String(), host, nil
}

// PrivateResolvedURL 把允许私网的受控 URL 解析为拨号地址,用于学校自有 LDAPS 目录服务。
func PrivateResolvedURL(ctx context.Context, raw, defaultPort string) (resolvedURL string, serverName string, err error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", "", fmt.Errorf("外部端点 URL 格式非法")
	}
	if parsed.User != nil {
		return "", "", fmt.Errorf("外部端点不允许携带凭据")
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		port = strings.TrimSpace(defaultPort)
	}
	if port == "" {
		return "", "", fmt.Errorf("外部端点缺少端口")
	}
	dialAddress, err := privateCapableDialAddress(ctx, host, port)
	if err != nil {
		return "", "", err
	}
	parsed.Host = dialAddress
	return parsed.String(), host, nil
}

// publicDialAddress 解析主机并拒绝任何非公网结果,阻断 DNS 解析到内网的 SSRF 路径。
func publicDialAddress(ctx context.Context, host, port string) (string, error) {
	if isLocalHostname(host) {
		return "", fmt.Errorf("外部端点主机非法")
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if !isPublicAddr(addr) {
			return "", fmt.Errorf("外部端点地址不允许指向内网或本机")
		}
		return net.JoinHostPort(addr.String(), port), nil
	}
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return "", fmt.Errorf("解析外部端点地址失败: %w", err)
	}
	if len(addrs) == 0 {
		return "", fmt.Errorf("外部端点没有可用地址")
	}
	for _, addr := range addrs {
		if !isPublicAddr(addr) {
			return "", fmt.Errorf("外部端点解析到内网或本机地址")
		}
	}
	return net.JoinHostPort(addrs[0].String(), port), nil
}

// validatePrivateCapableURL 校验受控私网场景的 URL,仅限制协议、凭据和本机地址,允许学校内网目录服务。
func validatePrivateCapableURL(raw, label string, schemes map[string]struct{}) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("外部%s端点 URL 格式非法", label)
	}
	if _, ok := schemes[parsed.Scheme]; !ok {
		return "", fmt.Errorf("外部%s端点协议不允许", label)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("外部%s端点不允许携带凭据", label)
	}
	host := parsed.Hostname()
	if strings.TrimSpace(host) == "" || isLocalHostname(host) {
		return "", fmt.Errorf("外部%s端点主机非法", label)
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if addr.IsLoopback() || addr.IsUnspecified() {
			return "", fmt.Errorf("外部%s端点地址不允许指向本机", label)
		}
	}
	return parsed.String(), nil
}

// privateCapableDialAddress 解析允许私网的受控目标,仅禁止本机和未解析结果。
func privateCapableDialAddress(ctx context.Context, host, port string) (string, error) {
	if isLocalHostname(host) {
		return "", fmt.Errorf("外部端点主机非法")
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if addr.IsLoopback() || addr.IsUnspecified() {
			return "", fmt.Errorf("外部端点地址不允许指向本机")
		}
		return net.JoinHostPort(addr.String(), port), nil
	}
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return "", fmt.Errorf("解析外部端点地址失败: %w", err)
	}
	if len(addrs) == 0 {
		return "", fmt.Errorf("外部端点没有可用地址")
	}
	for _, addr := range addrs {
		if addr.IsLoopback() || addr.IsUnspecified() {
			return "", fmt.Errorf("外部端点解析到本机地址")
		}
	}
	return net.JoinHostPort(addrs[0].String(), port), nil
}
