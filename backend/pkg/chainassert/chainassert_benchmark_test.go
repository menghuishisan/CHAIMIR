// chainassert_benchmark_test 测量链上断言判定与结果摘要的热点路径。
package chainassert

import "testing"

// BenchmarkCheck 测量单条断言包含脱敏 JSON 摘要时的端到端判定成本。
func BenchmarkCheck(b *testing.B) {
	assertion, err := FromMap(map[string]any{
		"target": "balance",
		"field":  "balance",
		"op":     OperationEq,
		"value":  100,
	})
	if err != nil {
		b.Fatal(err)
	}
	actual := map[string]any{
		"balance": 100,
		"owner":   "0x1234567890abcdef",
		"api_key": "benchmark-secret",
		"nonce":   7,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Check(assertion, actual); err != nil {
			b.Fatal(err)
		}
	}
}
