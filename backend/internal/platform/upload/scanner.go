// upload 提供统一病毒扫描接口与策略判断,让文件上传链路在基础设施层共享同一安全口径。
package upload

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chaimir/internal/platform/config"
)

// Verdict 表示统一病毒扫描结果。
type Verdict string

const (
	// VerdictClean 表示文件通过病毒扫描。
	VerdictClean Verdict = "clean"
	// VerdictInfected 表示文件命中病毒特征,必须拒绝写入。
	VerdictInfected Verdict = "infected"
)

// ScanRequest 描述一次待扫描的文件内容与元数据。
type ScanRequest struct {
	FileName string
	Content  []byte
	Timeout  time.Duration
}

// ScanResult 表示扫描引擎返回的安全结论。
type ScanResult struct {
	Verdict   Verdict
	Signature string
}

// ScanPolicy 描述调用方对扫描结果的最低安全要求。
type ScanPolicy struct {
	Required bool
}

// Scanner 是统一病毒扫描能力契约,由具体适配器实现。
type Scanner interface {
	// Scan 执行文件扫描并返回规范化结果。
	Scan(ctx context.Context, req ScanRequest) (ScanResult, error)
}

// NewScannerFromConfig 根据统一配置构造病毒扫描器,确保全平台上传链路使用同一装配口径。
func NewScannerFromConfig(cfg config.UploadConfig) (Scanner, error) {
	if !cfg.VirusScanRequired && strings.TrimSpace(cfg.VirusScanAddress) == "" {
		return nil, nil
	}
	return NewClamAVScanner(cfg.VirusScanNetwork, cfg.VirusScanAddress, time.Duration(cfg.VirusScanTimeoutSeconds)*time.Second)
}

// VerifyScan 在统一策略下执行病毒扫描,确保强制扫描场景不会静默放过风险文件。
func VerifyScan(ctx context.Context, scanner Scanner, policy ScanPolicy, req ScanRequest) error {
	if !policy.Required {
		return nil
	}
	if ctx == nil {
		return fmt.Errorf("上传扫描上下文不能为空")
	}
	if scanner == nil {
		return fmt.Errorf("上传安全策略要求病毒扫描,但未配置扫描器")
	}
	result, err := scanner.Scan(ctx, req)
	if err != nil {
		return fmt.Errorf("文件病毒扫描失败: %w", err)
	}
	switch result.Verdict {
	case VerdictClean:
		return nil
	case VerdictInfected:
		signature := strings.TrimSpace(result.Signature)
		if signature == "" {
			signature = "unknown"
		}
		return fmt.Errorf("文件命中病毒特征: %s", signature)
	default:
		return fmt.Errorf("文件病毒扫描结果非法: %s", result.Verdict)
	}
}
