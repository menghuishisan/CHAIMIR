// apperr 定义全平台统一应用错误体系,区分用户向响应与内部错误链。
package apperr

import (
	"errors"
	"fmt"
)

// Error 是跨 HTTP/API 边界传递的应用错误。
type Error struct {
	code    string
	message string
	cause   error
}

// New 构造不带内部原因的应用错误模板。
func New(code string, userMessage string) *Error {
	return &Error{code: code, message: userMessage}
}

// Error 只返回稳定错误码和用户向文案,避免 err.Error 被响应或业务状态误用时泄露内部原因。
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("[%s] %s", e.code, e.message)
}

// LogString 返回包含底层原因链的排障字符串,只能用于结构化日志和运维可见链路。
func (e *Error) LogString() string {
	if e == nil {
		return ""
	}
	if e.cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.code, e.message, e.cause)
	}
	return e.Error()
}

// UserCode 返回稳定错误码,供前端按码做跳转、重试或提示策略。
func (e *Error) UserCode() string {
	if e == nil {
		return CodeInternal
	}
	return e.code
}

// UserMessage 返回用户向提示文案。
func (e *Error) UserMessage() string {
	if e == nil {
		return MessageInternal
	}
	return e.message
}

// Unwrap 暴露内部错误链给日志和 errors.Is/As,但响应层不得输出该原因。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Is 按稳定错误码识别同类应用错误,使 WithCause 后仍可用 errors.Is 判断模板。
func (e *Error) Is(target error) bool {
	other, ok := target.(*Error)
	return ok && e != nil && other != nil && e.code != "" && e.code == other.code
}

// WithCause 基于错误模板包裹底层原因,保留排障链路。
func (e *Error) WithCause(cause error) *Error {
	if e == nil {
		return ErrInternal.WithCause(cause)
	}
	return &Error{code: e.code, message: e.message, cause: cause}
}

// AsAppError 将任意错误归一为应用错误,未知错误统一收敛成内部错误。
func AsAppError(err error) *Error {
	if err == nil {
		return nil
	}
	var ae *Error
	if errors.As(err, &ae) {
		return ae
	}
	return ErrInternal.WithCause(err)
}

// As 从任意 error 提取 *Error;非应用错误返回 false。
func As(err error) (*Error, bool) {
	var ae *Error
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}
