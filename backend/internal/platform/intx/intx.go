// intx 提供经过边界检查的整数收窄转换，供配置、外部输入和持久化边界复用。
package intx

import (
	"encoding/binary"
	"math"
)

// Int32 将 int 安全收窄为 int32，超出范围时返回 false。
func Int32(value int) (int32, bool) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, false
	}
	return int32(value), true
}

// Int64ToInt32 将 int64 安全收窄为 int32，超出范围时返回 false。
func Int64ToInt32(value int64) (int32, bool) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, false
	}
	return int32(value), true
}

// Int64ToInt16 将 int64 安全收窄为 int16，超出范围时返回 false。
func Int64ToInt16(value int64) (int16, bool) {
	if value < math.MinInt16 || value > math.MaxInt16 {
		return 0, false
	}
	return int16(value), true
}

// Int16 将 int 安全收窄为 int16，超出范围时返回 false。
func Int16(value int) (int16, bool) {
	if value < math.MinInt16 || value > math.MaxInt16 {
		return 0, false
	}
	return int16(value), true
}

// Uint32 将非负 int 安全收窄为 uint32，超出范围时返回 false。
func Uint32(value int) (uint32, bool) {
	if value < 0 || uint64(value) > math.MaxUint32 {
		return 0, false
	}
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(value))
	return binary.BigEndian.Uint32(encoded[4:]), true
}
