// Package apperr 定义全平台统一应用错误体系。
// 依据 docs/总-API接口总览.md §5 错误码分段 + CLAUDE.md §8 分层暴露:
//
//	终端用户只看友好 message + trace_id;完整错误链只进日志,不进 response body。
//
// 错误码用字符串(M10/M11 用字母前缀 A0/B0,纯数字容纳不下)。
package apperr

import (
	"errors"
	"fmt"
)

// Error 携带业务错误码与分层信息。Code/Message 进 response;cause 仅进日志。
type Error struct {
	Code    string
	Message string
	cause   error
}

// Error 实现 error 接口;含内部原因,仅用于日志,不可直接回前端。
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 暴露原始错误链,支持 errors.Is/As 与 %w 溯源。
func (e *Error) Unwrap() error { return e.cause }

// New 用错误码 + 用户向文案构造(无内部原因)。
func New(code, userMessage string) *Error {
	return &Error{Code: code, Message: userMessage}
}

// WithCause 复制错误码/文案预设并附加内部原因(用于预定义错误追加上下文)。
func (e *Error) WithCause(cause error) *Error {
	return &Error{Code: e.Code, Message: e.Message, cause: cause}
}

// As 从任意 error 提取 *Error;非应用错误返回 false。
func As(err error) (*Error, bool) {
	var ae *Error
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

// ---- 通用错误码(1xxxx 段,docs/总-API §5)----
var (
	ErrUnauthorized            = New("11001", "登录已失效,请重新登录")
	ErrForbidden               = New("11002", "你没有权限执行此操作")
	ErrCrossTenant             = New("11003", "无法访问该资源")
	ErrBadRequest              = New("11004", "请求信息有误,请检查后重试")
	ErrNotFound                = New("11005", "请求的内容不存在或已被移除")
	ErrConflict                = New("11006", "操作冲突,请刷新后重试")
	ErrRateLimited             = New("11007", "操作过于频繁,请稍后再试")
	ErrServiceUnauthorized     = New("11008", "内部服务鉴权未通过")
	ErrAuditActorResolveFailed = New("11009", "操作身份暂时无法确认,请稍后重试")
	ErrPathIDInvalid           = New("11010", "请求路径不正确,请检查后重试")
	ErrRequestBodyInvalid      = New("11011", "请求内容格式不正确,请检查后重试")
	ErrInternal                = New("11500", "服务繁忙,请稍后重试")
	ErrUnhandledFailure        = New("11501", "服务暂时无法处理请求,请稍后重试")
	ErrPanicRecovered          = New("11502", "服务暂时无法处理请求,请稍后重试")
)
