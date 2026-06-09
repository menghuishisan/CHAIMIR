// apperr 定义全平台统一应用错误体系,区分用户向响应与内部错误链。
package apperr

import (
	"errors"
	"fmt"
)

// Error 是跨 HTTP/API 边界传递的应用错误。
type Error struct {
	Code    string
	Message string
	cause   error
}

// New 构造不带内部原因的应用错误模板。
func New(code string, userMessage string) *Error {
	return &Error{Code: code, Message: userMessage}
}

// Error 返回用户向错误文案,避免把内部技术细节暴露给前端。
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// UserCode 返回稳定错误码,供前端按码做跳转、重试或提示策略。
func (e *Error) UserCode() string {
	if e == nil {
		return CodeInternal
	}
	return e.Code
}

// UserMessage 返回用户向提示文案。
func (e *Error) UserMessage() string {
	if e == nil {
		return MessageInternal
	}
	return e.Message
}

// Unwrap 暴露内部错误链给日志和 errors.Is/As,但响应层不得输出该原因。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// WithCause 基于错误模板包裹底层原因,保留排障链路。
func (e *Error) WithCause(cause error) *Error {
	if e == nil {
		return ErrInternal.WithCause(cause)
	}
	return &Error{Code: e.Code, Message: e.Message, cause: cause}
}

// WithMessage 基于错误模板替换用户向文案,用于同码下更具体的友好提示。
func (e *Error) WithMessage(message string) *Error {
	if e == nil {
		return New(CodeInternal, message)
	}
	return &Error{Code: e.Code, Message: message, cause: e.cause}
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
