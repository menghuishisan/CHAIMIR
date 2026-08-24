// hash_benchmark_test 测量认证和内容完整性相关哈希热点路径。
package crypto

import "testing"

// BenchmarkSHA256Hex 测量常规内容摘要的吞吐和分配。
func BenchmarkSHA256Hex(b *testing.B) {
	data := []byte("benchmark payload for content integrity")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SHA256Hex(data)
	}
}

// BenchmarkHMACHash 测量手机号、令牌等确定性哈希的热点成本。
func BenchmarkHMACHash(b *testing.B) {
	key := []byte("benchmark-hmac-key")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := HMACHash(key, "13900001002"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHashPassword 测量登录密码 Argon2id 成本,调用时应使用短 benchtime 避免占满测试机。
func BenchmarkHashPassword(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := HashPassword("benchmark-password"); err != nil {
			b.Fatal(err)
		}
	}
}
