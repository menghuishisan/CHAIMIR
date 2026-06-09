// snowflake 测试:验证节点边界和并发唯一性。
package snowflake

import (
	"sync"
	"testing"
)

// TestNodeRejectsInvalidNodeID 确认节点编号超出 10 bit 范围时拒绝启动。
func TestNodeRejectsInvalidNodeID(t *testing.T) {
	if _, err := NewNode(1024); err == nil {
		t.Fatalf("node id 1024 must be rejected")
	}
}

// TestGenerateIsConcurrentUnique 确认并发生成不会重复。
func TestGenerateIsConcurrentUnique(t *testing.T) {
	node, err := NewNode(7)
	if err != nil {
		t.Fatalf("new node: %v", err)
	}
	const workers = 8
	const perWorker = 512
	ids := make(chan int64, workers*perWorker)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perWorker; j++ {
				ids <- node.Generate()
			}
		}()
	}
	wg.Wait()
	close(ids)

	seen := make(map[int64]bool, workers*perWorker)
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate id: %d", id)
		}
		seen[id] = true
	}
}
