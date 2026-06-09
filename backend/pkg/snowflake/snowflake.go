// snowflake 实现全平台雪花算法 ID 生成器,用于 BIGINT 主键生成。
package snowflake

import (
	"errors"
	"sync"
	"time"
)

const (
	epochMillis    int64 = 1735689600000
	nodeBits             = 10
	sequenceBits         = 12
	maxNodeID            = -1 ^ (-1 << nodeBits)
	maxSequence          = -1 ^ (-1 << sequenceBits)
	timestampShift       = nodeBits + sequenceBits
	nodeShift            = sequenceBits
)

// Node 是并发安全的雪花 ID 生成器。
type Node struct {
	mu       sync.Mutex
	nodeID   int64
	lastTime int64
	sequence int64
}

// Generator 是平台统一 ID 生成契约;业务模块只依赖该契约。
type Generator interface {
	Generate() int64
}

// NewNode 创建指定节点编号的生成器;越界返回错误。
func NewNode(nodeID int64) (*Node, error) {
	if nodeID < 0 || nodeID > maxNodeID {
		return nil, errors.New("snowflake: node id 超出范围 [0,1023]")
	}
	return &Node{nodeID: nodeID, lastTime: -1}, nil
}

// Generate 生成下一个全局唯一 ID;同毫秒序列耗尽自旋到下一毫秒,时钟回拨时停在 lastTime。
func (n *Node) Generate() int64 {
	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Now().UnixMilli()
	if now < n.lastTime {
		now = n.lastTime
	}
	if now == n.lastTime {
		n.sequence = (n.sequence + 1) & maxSequence
		if n.sequence == 0 {
			for now <= n.lastTime {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		n.sequence = 0
	}
	n.lastTime = now

	return ((now - epochMillis) << timestampShift) | (n.nodeID << nodeShift) | n.sequence
}
