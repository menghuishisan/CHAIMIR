// Package terminal 定义与执行平台无关的终端尺寸契约。
package terminal

// Size 是终端当前的字符列数与行数。
type Size struct {
	Width  uint16
	Height uint16
}

// SizeQueue 等待下一次尺寸变化；连接结束时返回 nil，唤醒等待中的执行方。
type SizeQueue interface {
	Next() *Size
}
