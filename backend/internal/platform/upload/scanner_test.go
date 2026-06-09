// upload 测试覆盖统一文件服务的病毒扫描结果语义与策略边界。
package upload

import (
	"errors"
	"testing"
	"time"

	"chaimir/internal/platform/config"
)

type fakeScanner struct {
	result ScanResult
	err    error
}

// Scan 返回预设扫描结果。
func (s fakeScanner) Scan(_ ScanRequest) (ScanResult, error) {
	return s.result, s.err
}

// TestVerifyScanRequiresScannerOnEnabledPolicy 确认启用病毒扫描策略时缺少扫描器会立即失败。
func TestVerifyScanRequiresScannerOnEnabledPolicy(t *testing.T) {
	err := VerifyScan(nil, ScanPolicy{Required: true}, ScanRequest{FileName: "report.pdf", Content: []byte("abc")})
	if err == nil {
		t.Fatalf("expected missing scanner to fail")
	}
}

// TestVerifyScanRejectsInfectedFile 确认统一文件服务会拒绝命中病毒的文件。
func TestVerifyScanRejectsInfectedFile(t *testing.T) {
	err := VerifyScan(fakeScanner{result: ScanResult{Verdict: VerdictInfected, Signature: "EICAR-Test-File"}}, ScanPolicy{Required: true}, ScanRequest{
		FileName: "report.pdf",
		Content:  []byte("abc"),
	})
	if err == nil {
		t.Fatalf("expected infected file to fail")
	}
}

// TestVerifyScanRejectsScanFailureOnRequiredPolicy 确认强制扫描策略下扫描异常不会被静默放过。
func TestVerifyScanRejectsScanFailureOnRequiredPolicy(t *testing.T) {
	err := VerifyScan(fakeScanner{err: errors.New("scan timeout")}, ScanPolicy{Required: true}, ScanRequest{
		FileName: "report.pdf",
		Content:  []byte("abc"),
	})
	if err == nil {
		t.Fatalf("expected required scan failure to fail")
	}
}

// TestVerifyScanAllowsCleanResult 确认清洁文件可通过统一病毒扫描边界。
func TestVerifyScanAllowsCleanResult(t *testing.T) {
	err := VerifyScan(fakeScanner{result: ScanResult{Verdict: VerdictClean}}, ScanPolicy{Required: true}, ScanRequest{
		FileName: "report.pdf",
		Content:  []byte("abc"),
	})
	if err != nil {
		t.Fatalf("expected clean file to pass, got %v", err)
	}
}

// TestNewScannerFromConfigBuildsClamAVScanner 确认平台层会按统一配置装配 ClamAV 扫描器。
func TestNewScannerFromConfigBuildsClamAVScanner(t *testing.T) {
	scanner, err := NewScannerFromConfig(config.UploadConfig{
		VirusScanRequired:       true,
		VirusScanTimeoutSeconds: 12,
		VirusScanNetwork:        "tcp",
		VirusScanAddress:        "clamav:3310",
	})
	if err != nil {
		t.Fatalf("build scanner from config: %v", err)
	}

	clam, ok := scanner.(*ClamAVScanner)
	if !ok {
		t.Fatalf("expected ClamAVScanner, got %T", scanner)
	}
	if clam.network != "tcp" || clam.address != "clamav:3310" || clam.timeout != 12*time.Second {
		t.Fatalf("unexpected scanner config: %+v", clam)
	}
}

// TestNewScannerFromConfigRejectsMissingAddressWhenRequired 确认强制病毒扫描时缺少扫描器地址会在装配期失败。
func TestNewScannerFromConfigRejectsMissingAddressWhenRequired(t *testing.T) {
	_, err := NewScannerFromConfig(config.UploadConfig{
		VirusScanRequired:       true,
		VirusScanTimeoutSeconds: 12,
		VirusScanNetwork:        "tcp",
	})
	if err == nil {
		t.Fatalf("expected missing address to fail")
	}
}

// TestNewScannerFromConfigAllowsDisabledPolicyWithoutScanner 确认未启用病毒扫描时平台层不会强制构造扫描器。
func TestNewScannerFromConfigAllowsDisabledPolicyWithoutScanner(t *testing.T) {
	scanner, err := NewScannerFromConfig(config.UploadConfig{
		VirusScanRequired:       false,
		VirusScanTimeoutSeconds: 12,
	})
	if err != nil {
		t.Fatalf("disabled scan policy should not fail: %v", err)
	}
	if scanner != nil {
		t.Fatalf("disabled scan policy should not build scanner")
	}
}
