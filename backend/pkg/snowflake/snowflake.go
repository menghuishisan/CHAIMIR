// snowflake 实现全平台雪花算法 ID 生成器,用于 BIGINT 主键生成。
package snowflake

import (
	"errors"
	"sync"
	"time"
)

const (
	epochMillis     int64 = 1735689600000
	nodeBits              = 10
	sequenceBits          = 12
	maxNodeID             = -1 ^ (-1 << nodeBits)
	maxSequence           = -1 ^ (-1 << sequenceBits)
	timestampShift        = nodeBits + sequenceBits
	nodeShift             = sequenceBits
	maxLogicalDrift       = int64(5)
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

// Generate 生成下一个全局唯一 ID;同毫秒序列耗尽时进入下一毫秒,时钟回拨时使用单调逻辑时间避免重复。
func (n *Node) Generate() int64 {
	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Now().UnixMilli()
	if now < epochMillis {
		now = epochMillis
	}
	if now < n.lastTime {
		now = n.lastTime
	}
	if now == n.lastTime {
		n.sequence = (n.sequence + 1) & maxSequence
		if n.sequence == 0 {
			now = nextMillis(n.lastTime)
		}
	} else {
		n.sequence = 0
	}
	n.lastTime = now

	return ((now - epochMillis) << timestampShift) | (n.nodeID << nodeShift) | n.sequence
}

// nextMillis 等待物理时钟进入下一毫秒;若检测到明显回拨,使用逻辑毫秒推进避免长时间忙等。
func nextMillis(lastTime int64) int64 {
	for {
		now := time.Now().UnixMilli()
		if now > lastTime {
			return now
		}
		if lastTime-now > maxLogicalDrift {
			return lastTime + 1
		}
		time.Sleep(time.Millisecond)
	}
}
