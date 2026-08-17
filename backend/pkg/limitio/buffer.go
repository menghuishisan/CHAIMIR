// Package limitio 提供写入阶段即生效的有界内存输出缓冲,防止不可信流耗尽进程内存。
package limitio

import (
	"bytes"
	"errors"
)

// ErrLimitExceeded 表示输出流已达到调用方配置的字节上限。
var ErrLimitExceeded = errors.New("stream output limit exceeded")

// Buffer 是达到字节上限后立即拒绝后续写入的内存缓冲区。
type Buffer struct {
	buffer bytes.Buffer
	limit  int64
}

// NewBuffer 创建具有显式字节上限的输出缓冲区;非正上限会拒绝所有写入。
func NewBuffer(limit int64) *Buffer {
	return &Buffer{limit: limit}
}

// Write 保留当前剩余容量内的内容,首次超限即返回 ErrLimitExceeded 以中止上游流。
func (b *Buffer) Write(data []byte) (int, error) {
	remaining := b.limit - int64(b.buffer.Len())
	if remaining <= 0 {
		return 0, ErrLimitExceeded
	}
	if int64(len(data)) > remaining {
		written, err := b.buffer.Write(data[:int(remaining)])
		if err != nil {
			return written, err
		}
		return written, ErrLimitExceeded
	}
	return b.buffer.Write(data)
}

// Bytes 返回已在上限内收集的输出。
func (b *Buffer) Bytes() []byte {
	return b.buffer.Bytes()
}

// Len 返回已收集的输出字节数。
func (b *Buffer) Len() int {
	return b.buffer.Len()
}

// String 返回已在上限内收集的文本输出。
func (b *Buffer) String() string {
	return b.buffer.String()
}
