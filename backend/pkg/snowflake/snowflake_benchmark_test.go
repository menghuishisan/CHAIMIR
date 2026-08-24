// snowflake_benchmark_test 测量主键 ID 生成的串行和并行吞吐。
package snowflake

import "testing"

// BenchmarkGenerate 测量单 goroutine 生成 ID 的成本。
func BenchmarkGenerate(b *testing.B) {
	node, err := NewNode(1)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = node.Generate()
	}
}

// BenchmarkGenerateParallel 测量并发请求下互斥锁和时钟逻辑的吞吐。
func BenchmarkGenerateParallel(b *testing.B) {
	node, err := NewNode(1)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = node.Generate()
		}
	})
}
