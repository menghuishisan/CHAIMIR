// M3 判题器策略测试:覆盖六类判题器中可纯函数验证的核心判定规则。
package judge

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
)

// TestFlagJudgerChecksSha256Hash 确认 Flag 判题优先使用 SHA-256 黑盒比对。
func TestFlagJudgerChecksSha256Hash(t *testing.T) {
	result, err := judgeFlag(map[string]any{
		"flag_hash":      "2bb80d537b1da3e38bd30361aa855686bde0eacd7162fef6a25fe97bf527a25b",
		"flag_input_key": "flag",
	}, map[string]any{"flag": "secret"}, 10)
	if err != nil {
		t.Fatalf("valid flag rejected: %v", err)
	}
	if !result.Passed || result.Score != 10 {
		t.Fatalf("expected full score, got %#v", result)
	}
}

// TestFlagSnapshotHashesPlainFlagValue 确认 M3 输入快照不保存静态 flag 明文。
func TestFlagSnapshotHashesPlainFlagValue(t *testing.T) {
	req := contracts.JudgeSubmitRequest{TenantID: 10, SubmitterID: 20, ItemCode: "prob", ItemVersion: "1.0.0"}
	expectation, err := snapshotExpectationForJudger(req, "prob:1.0.0", JudgerTypeFlag, map[string]any{
		"flag_value":     "flag{secret}",
		"flag_input_key": "flag",
	})
	if err != nil {
		t.Fatalf("flag expectation rejected: %v", err)
	}
	if expectation["flag_value"] != nil {
		t.Fatalf("flag_value must not be persisted in input_snapshot: %#v", expectation)
	}
	result, err := judgeFlag(expectation, map[string]any{"flag": "flag{secret}"}, 10)
	if err != nil {
		t.Fatalf("hashed static flag rejected: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected hashed static flag to pass")
	}
}

// TestFlagSnapshotRejectsInvalidHash 确认 J3 快照中的 flag_hash 必须是 SHA-256 十六进制。
func TestFlagSnapshotRejectsInvalidHash(t *testing.T) {
	req := contracts.JudgeSubmitRequest{TenantID: 10, SubmitterID: 20, ItemCode: "prob", ItemVersion: "1.0.0"}

	_, err := snapshotExpectationForJudger(req, "prob:1.0.0", JudgerTypeFlag, map[string]any{
		"flag_hash":      "not-a-sha256",
		"flag_input_key": "flag",
	})

	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrJudgerInvalid.Code {
		t.Fatalf("invalid flag_hash must return %s, got %v", apperr.ErrJudgerInvalid.Code, err)
	}
}

// TestFlagJudgerDoesNotAcceptPlainFlagValueAtRuntime 确认运行时 J3 只接受已固化 hash。
func TestFlagJudgerDoesNotAcceptPlainFlagValueAtRuntime(t *testing.T) {
	_, err := judgeFlag(map[string]any{
		"flag_value":     "flag{secret}",
		"flag_input_key": "flag",
	}, map[string]any{"flag": "flag{secret}"}, 10)

	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrJudgerInvalid.Code {
		t.Fatalf("runtime flag_value must return %s, got %v", apperr.ErrJudgerInvalid.Code, err)
	}
}

// TestFlagSnapshotSupportsHMACDynamicFlag 确认动态 HMAC flag 可复现且不保存 secret。
func TestFlagSnapshotSupportsHMACDynamicFlag(t *testing.T) {
	req := contracts.JudgeSubmitRequest{TenantID: 10, SubmitterID: 20, ItemCode: "prob", ItemVersion: "1.0.0"}
	problemRef := "prob:1.0.0"
	expectation, err := snapshotExpectationForJudger(req, problemRef, JudgerTypeFlag, map[string]any{
		"flag_hmac_secret": "teacher-secret",
		"flag_hmac_seed":   "round-1",
		"flag_input_key":   "flag",
	})
	if err != nil {
		t.Fatalf("hmac flag expectation rejected: %v", err)
	}
	if expectation["flag_hmac_secret"] != nil {
		t.Fatalf("flag_hmac_secret must not be persisted in input_snapshot: %#v", expectation)
	}
	mac := hmac.New(sha256.New, []byte("teacher-secret"))
	mac.Write([]byte("10:20:" + problemRef + ":round-1"))
	submitted := hex.EncodeToString(mac.Sum(nil))
	result, err := judgeFlag(expectation, map[string]any{"flag": submitted}, 10)
	if err != nil {
		t.Fatalf("hmac flag rejected: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected hmac flag to pass")
	}
}

// TestSnapshotExpectationRejectsSensitiveAnswerMaterial 确认 M3 不保存答案正本或敏感判题材料。
func TestSnapshotExpectationRejectsSensitiveAnswerMaterial(t *testing.T) {
	req := contracts.JudgeSubmitRequest{TenantID: 10, SubmitterID: 20, ItemCode: "prob", ItemVersion: "1.0.0"}

	_, err := snapshotExpectationForJudger(req, "prob:1.0.0", JudgerTypeTestcase, map[string]any{
		"command_result": "json",
		"answer_source":  "contract solution {}",
	})

	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrJudgerInvalid.Code {
		t.Fatalf("sensitive expectation material must return %s, got %v", apperr.ErrJudgerInvalid.Code, err)
	}
}

// TestSimCheckpointRequiresExactStructuredMatch 确认仿真检查点按结构化快照精确比对。
func TestSimCheckpointRequiresExactStructuredMatch(t *testing.T) {
	result, err := judgeSimCheckpoint(map[string]any{
		"checkpoint": map[string]any{"step": "deploy", "ok": true},
	}, map[string]any{"checkpoint": map[string]any{"step": "deploy", "ok": true}}, 5)
	if err != nil {
		t.Fatalf("valid checkpoint rejected: %v", err)
	}
	if !result.Passed || result.Score != 5 {
		t.Fatalf("expected checkpoint pass, got %#v", result)
	}
}

// TestSimCheckpointDetailsRedactExpectedAnswer 确认 J5 失败结果只返回期望摘要,不泄露标准检查点。
func TestSimCheckpointDetailsRedactExpectedAnswer(t *testing.T) {
	result, err := judgeSimCheckpoint(map[string]any{
		"checkpoint_label": "部署流程应完成",
		"checkpoint":       map[string]any{"private_answer": "teacher-only", "ok": true},
	}, map[string]any{"checkpoint": map[string]any{"step": "deploy", "ok": false}}, 5)
	if err != nil {
		t.Fatalf("valid checkpoint rejected: %v", err)
	}
	if result.Passed {
		t.Fatalf("expected checkpoint mismatch to fail")
	}
	details, ok := result.Details.([]map[string]any)
	if !ok || len(details) != 1 {
		t.Fatalf("unexpected details: %#v", result.Details)
	}
	if details[0]["expected"] != nil {
		t.Fatalf("J5 details must not expose full expected checkpoint, got %#v", details[0])
	}
	if details[0]["expected_label"] != "部署流程应完成" || details[0]["actual"] == nil {
		t.Fatalf("J5 details must include expected_label and actual, got %#v", details[0])
	}
}

// TestChainAssertionOperators 确认链上断言支持等于、包含和存在操作符。
func TestChainAssertionOperators(t *testing.T) {
	if err := checkAssertionValue(map[string]any{"balance": "100", "events": []any{"Transfer"}}, "balance", "eq", "100"); err != nil {
		t.Fatalf("eq assertion rejected: %v", err)
	}
	if err := checkAssertionValue(map[string]any{"events": []any{"Transfer"}}, "events", "contains", "Transfer"); err != nil {
		t.Fatalf("contains assertion rejected: %v", err)
	}
	if err := checkAssertionValue(map[string]any{"tx": "0xabc"}, "tx", "exists", nil); err != nil {
		t.Fatalf("exists assertion rejected: %v", err)
	}
}

// TestOnchainAssertionsRequireConfiguredAssertions 确认 J2 不能在无断言时直接判通过。
func TestOnchainAssertionsRequireConfiguredAssertions(t *testing.T) {
	svc := &Service{sandbox: &fakeChainSandbox{}}

	_, err := svc.judgeOnchainAssertions(context.Background(), 9001, map[string]any{}, 10)

	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrJudgerInvalid.Code {
		t.Fatalf("J2 without assertions must return %s, got %v", apperr.ErrJudgerInvalid.Code, err)
	}
}

// TestOnchainAssertionDetailsRedactExpectedAnswer 确认 J2 失败详情只返回期望摘要,不暴露完整断言答案。
func TestOnchainAssertionDetailsRedactExpectedAnswer(t *testing.T) {
	svc := &Service{sandbox: &fakeChainSandbox{queryResult: map[string]any{"balance": "50"}}}

	result, err := svc.judgeOnchainAssertions(context.Background(), 9001, map[string]any{
		"chain_steps": []any{
			map[string]any{"name": "state", "action": "query", "target": "account:alice"},
		},
		"assertions": []any{
			map[string]any{"source": "state", "target": "balance", "op": "eq", "label": "余额应达到目标区间", "expected": map[string]any{"private_answer": "100"}},
		},
	}, 10)
	if err != nil {
		t.Fatalf("valid J2 expectation rejected: %v", err)
	}
	if result.Passed || result.Score != 0 {
		t.Fatalf("expected failed assertion with zero score, got %#v", result)
	}
	details, ok := result.Details.([]map[string]any)
	if !ok || len(details) != 1 {
		t.Fatalf("unexpected details: %#v", result.Details)
	}
	if details[0]["expected"] != nil {
		t.Fatalf("J2 details must not expose full expected assertion, got %#v", details[0])
	}
	if details[0]["expected_label"] != "余额应达到目标区间" || details[0]["actual"] != "50" {
		t.Fatalf("details must include expected_label and actual, got %#v", details[0])
	}
}

// TestJudgerDetailsNeverUseExpectedKey 确认生产结果详情不重新使用会泄露答案的 expected 字段。
func TestJudgerDetailsNeverUseExpectedKey(t *testing.T) {
	data, err := os.ReadFile("service_judgers.go")
	if err != nil {
		t.Fatalf("read service_judgers.go: %v", err)
	}
	if strings.Contains(string(data), `"expected":`) {
		t.Fatalf("judger result details must use expected_label instead of expected")
	}
}

type fakeChainSandbox struct {
	contracts.SandboxService
	queryResult map[string]any
}

func (f *fakeChainSandbox) ChainQuery(context.Context, int64, string) (map[string]any, error) {
	return f.queryResult, nil
}
