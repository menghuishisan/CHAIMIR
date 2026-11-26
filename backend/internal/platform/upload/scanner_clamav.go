package upload

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"chaimir/internal/platform/intx"
)

const clamAVResponseMaxBytes = 4096

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
func (s *ClamAVScanner) Scan(ctx context.Context, req ScanRequest) (result ScanResult, err error) {
	if s == nil {
		return ScanResult{}, fmt.Errorf("ClamAV 扫描器未初始化")
	}
	if ctx == nil {
		return ScanResult{}, fmt.Errorf("ClamAV 扫描上下文不能为空")
	}
	scanCtx, cancel := context.WithTimeout(ctx, effectiveTimeout(req.Timeout, s.timeout))
	defer cancel()

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(scanCtx, s.network, s.address)
	if err != nil {
		return ScanResult{}, fmt.Errorf("连接 ClamAV 失败: %w", err)
	}
	// DialContext 只负责建连；取消请求还必须中断已建立连接上的读写。
	closed := make(chan error, 1)
	stop := context.AfterFunc(scanCtx, func() { closed <- conn.Close() })
	defer func() {
		var closeErr error
		if stop() {
			closeErr = conn.Close()
		} else {
			closeErr = <-closed
		}
		if cause := scanCtx.Err(); cause != nil {
			err = errors.Join(err, fmt.Errorf("ClamAV 扫描已取消: %w", cause))
		}
		if closeErr != nil {
			err = errors.Join(err, fmt.Errorf("关闭 ClamAV 连接失败: %w", closeErr))
		}
	}()
	// 第一步:按 clamd 协议发送 INSTREAM 命令和长度分块,避免一次性拼巨大内存。
	if _, err := io.WriteString(conn, "zINSTREAM\x00"); err != nil {
		return ScanResult{}, fmt.Errorf("发送 ClamAV 指令失败: %w", err)
	}
	reader := bytes.NewReader(req.Content)
	buf := make([]byte, 32*1024)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			chunkLength, ok := intx.Uint32(n)
			if !ok {
				return ScanResult{}, fmt.Errorf("ClamAV 数据块长度超出协议范围")
			}
			if err := binary.Write(conn, binary.BigEndian, chunkLength); err != nil {
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
	responseReader := bufio.NewReaderSize(conn, clamAVResponseMaxBytes)
	line, err := readClamAVResponse(responseReader)
	if err != nil {
		return ScanResult{}, fmt.Errorf("读取 ClamAV 结果失败: %w", err)
	}
	return parseClamAVResponse(line)
}

func readClamAVResponse(reader *bufio.Reader) (string, error) {
	var response []byte
	for {
		chunk, err := reader.ReadSlice('\x00')
		response = append(response, chunk...)
		if len(response) > clamAVResponseMaxBytes {
			return "", fmt.Errorf("ClamAV 响应超出大小上限")
		}
		if err == nil {
			return string(response), nil
		}
		if err != bufio.ErrBufferFull {
			return "", err
		}
	}
}

func parseClamAVResponse(raw string) (ScanResult, error) {
	line := strings.TrimSpace(strings.TrimSuffix(raw, "\x00"))
	result, ok := strings.CutPrefix(line, "stream: ")
	if !ok {
		return ScanResult{}, fmt.Errorf("ClamAV 返回无法识别的结果: %s", line)
	}
	switch {
	case result == "OK":
		return ScanResult{Verdict: VerdictClean}, nil
	case strings.HasSuffix(result, " FOUND"):
		signature := strings.TrimSpace(strings.TrimSuffix(result, " FOUND"))
		if signature == "" {
			return ScanResult{}, fmt.Errorf("ClamAV 病毒特征为空")
		}
		return ScanResult{Verdict: VerdictInfected, Signature: signature}, nil
	case strings.HasSuffix(result, " ERROR"):
		return ScanResult{}, fmt.Errorf("ClamAV 扫描失败: %s", line)
	default:
		return ScanResult{}, fmt.Errorf("ClamAV 返回无法识别的结果: %s", line)
	}
}

func effectiveTimeout(requestTimeout, defaultTimeout time.Duration) time.Duration {
	if requestTimeout > 0 {
		return requestTimeout
	}
	return defaultTimeout
}
