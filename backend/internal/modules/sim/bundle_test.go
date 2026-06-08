// M4 bundle 测试:覆盖仿真包静态扫描的危险调用拦截。
package sim

import (
	"archive/zip"
	"bytes"
	"fmt"
	"testing"

	"chaimir/internal/platform/config"
	"chaimir/pkg/apperr"
)

// TestScanBundleRejectsDangerousBrowserCapabilities 确认 bundle 中危险浏览器能力会被拦截。
func TestScanBundleRejectsDangerousBrowserCapabilities(t *testing.T) {
	cases := []string{
		`export const reducer = () => { fetch("https://example.com"); return {}; }`,
		`export const reducer = () => globalThis.fetch("https://example.com");`,
		`export const reducer = () => Function("return 1")();`,
		`export const reducer = () => { new Function("return 1")(); }`,
		`export const reducer = () => importScripts("https://example.com/a.js");`,
		`export const reducer = () => new WebSocket("wss://example.com");`,
		`export const reducer = () => document /* split */ . cookie;`,
		`export const reducer = () => f/**/etch("https://example.com");`,
	}
	for _, source := range cases {
		report, err := scanBundleWithLimits([]byte(source), "bundle.js", testBundleLimits())
		if err == nil {
			t.Fatalf("expected dangerous bundle to be rejected, report=%+v source=%s", report, source)
		}
	}
}

// TestScanBundleAcceptsPureReducerSource 确认纯 reducer 文本可以通过静态扫描。
func TestScanBundleAcceptsPureReducerSource(t *testing.T) {
	data := []byte(`export const reducer = (state, event, tick) => ({...state, tick});`)
	report, err := scanBundleWithLimits(data, "bundle.js", testBundleLimits())
	if err != nil {
		t.Fatalf("expected pure reducer bundle to pass, got %v", err)
	}
	if report["static_scan"] != "passed" {
		t.Fatalf("expected static_scan passed, got %+v", report)
	}
}

// TestScanBundleAcceptsOrdinaryFunctionExpression 确认静态扫描不会把普通 reducer 函数表达式误判为动态执行。
func TestScanBundleAcceptsOrdinaryFunctionExpression(t *testing.T) {
	data := []byte(`export const reducer = function (state, event, tick) { return {...state, tick}; };`)
	report, err := scanBundleWithLimits(data, "bundle.js", testBundleLimits())
	if err != nil {
		t.Fatalf("expected ordinary function expression to pass, got %v", err)
	}
	if report["static_scan"] != "passed" {
		t.Fatalf("expected static_scan passed, got %+v", report)
	}
}

// TestScanBundleRejectsTooManyArchiveEntries 确认上传包展开有文件数边界,防止压缩包耗尽内存。
func TestScanBundleRejectsTooManyArchiveEntries(t *testing.T) {
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for i := 0; i < 3; i++ {
		file, err := writer.Create(fmt.Sprintf("file%d.js", i))
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := file.Write([]byte("export const reducer = () => ({});")); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	_, err := scanBundleWithLimits(buf.Bytes(), "bundle.zip", config.UploadConfig{
		SimBundleMaxFiles:         2,
		SimBundleMaxUnpackedBytes: 1 << 20,
	})
	if err != apperr.ErrSimBundleTooLarge {
		t.Fatalf("expected bundle too large error, got %v", err)
	}
}

// TestScanBundleRejectsDuplicateArchiveEntryNames 确认同名归档条目不能覆盖早期危险内容。
func TestScanBundleRejectsDuplicateArchiveEntryNames(t *testing.T) {
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	dangerous, err := writer.Create("bundle.js")
	if err != nil {
		t.Fatalf("create dangerous entry: %v", err)
	}
	if _, err := dangerous.Write([]byte(`export const reducer = () => eval("1");`)); err != nil {
		t.Fatalf("write dangerous entry: %v", err)
	}
	safe, err := writer.Create("bundle.js")
	if err != nil {
		t.Fatalf("create duplicate entry: %v", err)
	}
	if _, err := safe.Write([]byte(`export const reducer = () => ({});`)); err != nil {
		t.Fatalf("write duplicate entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	_, err = scanBundleWithLimits(buf.Bytes(), "bundle.zip", testBundleLimits())
	if err != apperr.ErrSimPackageInvalid {
		t.Fatalf("expected duplicate entry to be rejected as invalid package, got %v", err)
	}
}

// TestScanBundleRejectsUnsafeArchiveEntryNames 确认归档条目不能使用绝对路径或目录穿越路径。
func TestScanBundleRejectsUnsafeArchiveEntryNames(t *testing.T) {
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	file, err := writer.Create("../bundle.js")
	if err != nil {
		t.Fatalf("create unsafe entry: %v", err)
	}
	if _, err := file.Write([]byte(`export const reducer = () => ({});`)); err != nil {
		t.Fatalf("write unsafe entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	_, err = scanBundleWithLimits(buf.Bytes(), "bundle.zip", testBundleLimits())
	if err != apperr.ErrSimPackageInvalid {
		t.Fatalf("expected unsafe entry to be rejected as invalid package, got %v", err)
	}
}

// TestSimBundleObjectKeyRejectsUnsafeSegments 确认 M4 bundle 对象 key 复用平台存储安全规则。
func TestSimBundleObjectKeyRejectsUnsafeSegments(t *testing.T) {
	if _, err := simBundleObjectKey(1, "pkg", "v1", "../bundle.zip"); err == nil {
		t.Fatalf("unsafe filename must be rejected")
	}
	if _, err := simBundleObjectKey(1, "pkg/escape", "v1", "bundle.zip"); err == nil {
		t.Fatalf("unsafe package code must be rejected")
	}
}

// TestSimBundleUploadSizeUsesPlatformResult 确认 M4 上传大小边界复用 platform/upload 的统一结果。
func TestSimBundleUploadSizeUsesPlatformResult(t *testing.T) {
	cfg := config.UploadConfig{SimBundleMaxBytes: 10}
	cases := []struct {
		size int64
		want error
	}{
		{size: 0, want: apperr.ErrSimPackageInvalid},
		{size: 11, want: apperr.ErrSimBundleTooLarge},
	}
	for _, tc := range cases {
		if err := validateSimBundleUploadSize(tc.size, cfg); err != tc.want {
			t.Fatalf("size %d expected %v, got %v", tc.size, tc.want, err)
		}
	}
}

func testBundleLimits() config.UploadConfig {
	return config.UploadConfig{
		SimBundleMaxFiles:         10,
		SimBundleMaxUnpackedBytes: 1 << 20,
	}
}
