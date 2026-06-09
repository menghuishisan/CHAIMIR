// upload 提供基于 ClamAV clamd INSTREAM 协议的统一病毒扫描适配器。
package upload

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// ClamAVScanner 通过 clamd 的 INSTREAM 协议执行病毒扫描,供统一文件服务复用。
type ClamAVScanner struct {
	network string
	address string
	timeout time.Duration
}

// NewClamAVScanner 构造 ClamAV 扫描器;地址为空时直接报错,避免上传链路带着空配置运行。
func NewClamAVScanner(network, address string, timeout time.Duration) (*ClamAVScanner, error) {
	if strings.TrimSpace(network) == "" {
		network = "tcp"
	}
	if strings.TrimSpace(address) == "" {
		return nil, fmt.Errorf("ClamAV 扫描器地址不能为空")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("ClamAV 扫描超时必须大于 0")
	}
	return &ClamAVScanner{
		network: strings.TrimSpace(network),
		address: strings.TrimSpace(address),
		timeout: timeout,
	}, nil
}

// Scan 按 clamd INSTREAM 协议发送文件内容,并把扫描结果归一为平台统一 Verdict。
func (s *ClamAVScanner) Scan(req ScanRequest) (ScanResult, error) {
	if s == nil {
		return ScanResult{}, fmt.Errorf("ClamAV 扫描器未初始化")
	}
	ctx, cancel := context.WithTimeout(context.Background(), effectiveTimeout(req.Timeout, s.timeout))
	defer cancel()

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, s.network, s.address)
	if err != nil {
		return ScanResult{}, fmt.Errorf("连接 ClamAV 失败: %w", err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(effectiveTimeout(req.Timeout, s.timeout))); err != nil {
		return ScanResult{}, fmt.Errorf("设置 ClamAV 超时失败: %w", err)
	}

	// 第一步:按 clamd 协议发送 INSTREAM 命令和长度分块,避免一次性拼巨大内存。
	if _, err := io.WriteString(conn, "zINSTREAM\x00"); err != nil {
		return ScanResult{}, fmt.Errorf("发送 ClamAV 指令失败: %w", err)
	}
	reader := bytes.NewReader(req.Content)
	buf := make([]byte, 32*1024)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			if err := binary.Write(conn, binary.BigEndian, uint32(n)); err != nil {
				return ScanResult{}, fmt.Errorf("发送 ClamAV 数据块长度失败: %w", err)
			}
			if _, err := conn.Write(buf[:n]); err != nil {
				return ScanResult{}, fmt.Errorf("发送 ClamAV 数据块失败: %w", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return ScanResult{}, fmt.Errorf("读取待扫描内容失败: %w", readErr)
		}
	}
	if err := binary.Write(conn, binary.BigEndian, uint32(0)); err != nil {
		return ScanResult{}, fmt.Errorf("结束 ClamAV 数据流失败: %w", err)
	}

	// 第二步:解析扫描结果,把 FOUND/OK 收敛到统一 Verdict。
	line, err := bufio.NewReader(conn).ReadString('\x00')
	if err != nil {
		return ScanResult{}, fmt.Errorf("读取 ClamAV 结果失败: %w", err)
	}
	return parseClamAVResponse(line)
}

// parseClamAVResponse 把 clamd 原始响应解析成平台统一扫描结果。
func parseClamAVResponse(raw string) (ScanResult, error) {
	line := strings.TrimSpace(strings.TrimSuffix(raw, "\x00"))
	switch {
	case strings.HasSuffix(line, "OK"):
		return ScanResult{Verdict: VerdictClean}, nil
	case strings.Contains(line, "FOUND"):
		signature := strings.TrimSpace(strings.TrimSuffix(strings.SplitN(line, "FOUND", 2)[0], ":"))
		return ScanResult{Verdict: VerdictInfected, Signature: signature}, nil
	case strings.Contains(line, "ERROR"):
		return ScanResult{}, fmt.Errorf("ClamAV 扫描失败: %s", line)
	default:
		return ScanResult{}, fmt.Errorf("ClamAV 返回无法识别的结果: %s", line)
	}
}

// effectiveTimeout 优先使用请求级超时,否则回退到扫描器默认超时。
func effectiveTimeout(requestTimeout, defaultTimeout time.Duration) time.Duration {
	if requestTimeout > 0 {
		return requestTimeout
	}
	return defaultTimeout
}
