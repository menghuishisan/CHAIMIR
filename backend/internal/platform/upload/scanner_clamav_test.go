// upload 测试覆盖 ClamAV 扫描结果解析,保证统一病毒扫描适配器能正确归一 verdict。
package upload

import "testing"

// TestParseClamAVResponseOK 确认 clean 结果会归一为 VerdictClean。
func TestParseClamAVResponseOK(t *testing.T) {
	result, err := parseClamAVResponse("stream: OK\x00")
	if err != nil {
		t.Fatalf("parse clamav ok: %v", err)
	}
	if result.Verdict != VerdictClean {
		t.Fatalf("verdict = %s, want clean", result.Verdict)
	}
}

// TestParseClamAVResponseFOUND 确认命中病毒时会提取签名并归一为 VerdictInfected。
func TestParseClamAVResponseFOUND(t *testing.T) {
	result, err := parseClamAVResponse("stream: Eicar-Signature FOUND\x00")
	if err != nil {
		t.Fatalf("parse clamav found: %v", err)
	}
	if result.Verdict != VerdictInfected || result.Signature != "stream: Eicar-Signature" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
