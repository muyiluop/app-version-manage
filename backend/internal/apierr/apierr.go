// Package apierr 定义统一错误码与 HTTP 响应封装（v2 契约）。
//
// 响应体固定为 {"code":"...","message":"...","data":{...},"requestId":"..."}。
// v1 兼容接口不使用该封装，保持历史结构不变。
package apierr

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Code 业务错误码。
type Code string

const (
	CodeOK               Code = "OK"
	CodeInvalidParam     Code = "INVALID_PARAM"
	CodeUnauthorized     Code = "UNAUTHORIZED"
	CodeForbidden        Code = "FORBIDDEN"
	CodeNotFound         Code = "NOT_FOUND"
	CodeConflict         Code = "CONFLICT"
	CodeTooManyRequests  Code = "TOO_MANY_REQUESTS"
	CodePayloadTooLarge  Code = "PAYLOAD_TOO_LARGE"
	CodeUnsupportedMedia Code = "UNSUPPORTED_MEDIA_TYPE"
	CodePasswordRequired Code = "PASSWORD_REQUIRED"
	CodePasswordInvalid  Code = "PASSWORD_INVALID"
	CodeInternal         Code = "INTERNAL"
)

// Error 带错误码与 HTTP 状态码的业务错误。
type Error struct {
	Code    Code
	Message string
	Status  int
	cause   error
}

// Error 实现 error 接口。
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap 返回底层错误。
func (e *Error) Unwrap() error { return e.cause }

// WithCause 附加底层错误。
func (e *Error) WithCause(err error) *Error {
	clone := *e
	clone.cause = err
	return &clone
}

// New 构造业务错误。
func New(code Code, status int, message string) *Error {
	return &Error{Code: code, Message: message, Status: status}
}

// 常用构造器。
func BadRequest(message string) *Error {
	return New(CodeInvalidParam, http.StatusBadRequest, message)
}

func Unauthorized(message string) *Error {
	return New(CodeUnauthorized, http.StatusUnauthorized, message)
}

func Forbidden(message string) *Error {
	return New(CodeForbidden, http.StatusForbidden, message)
}

func NotFound(message string) *Error {
	return New(CodeNotFound, http.StatusNotFound, message)
}

func Conflict(message string) *Error {
	return New(CodeConflict, http.StatusConflict, message)
}

func TooManyRequests(message string) *Error {
	return New(CodeTooManyRequests, http.StatusTooManyRequests, message)
}

func PayloadTooLarge(message string) *Error {
	return New(CodePayloadTooLarge, http.StatusRequestEntityTooLarge, message)
}

func PasswordRequired(message string) *Error {
	return New(CodePasswordRequired, http.StatusUnauthorized, message)
}

func Internal(message string) *Error {
	return New(CodeInternal, http.StatusInternalServerError, message)
}

// AsError 将任意 error 归一化为 *Error，未知错误统一按 500 处理。
func AsError(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Internal("服务器内部错误").WithCause(err)
}

// Envelope v2 统一响应体。
type Envelope struct {
	Code      Code   `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

// RequestIDKey gin 上下文中请求 ID 的键。
const RequestIDKey = "requestId"

// requestID 从上下文取请求 ID。
func requestID(c *gin.Context) string {
	if v, ok := c.Get(RequestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// OK 输出成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{
		Code:      CodeOK,
		Message:   "success",
		Data:      data,
		RequestID: requestID(c),
	})
}

// Created 输出 201 响应。
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{
		Code:      CodeOK,
		Message:   "success",
		Data:      data,
		RequestID: requestID(c),
	})
}

// Fail 输出错误响应并中止后续处理。
func Fail(c *gin.Context, err error) {
	e := AsError(err)
	c.AbortWithStatusJSON(e.Status, Envelope{
		Code:      e.Code,
		Message:   e.Message,
		RequestID: requestID(c),
	})
}

// FailRaw 输出指定错误码/状态码的错误响应（用于特殊场景）。
func FailRaw(c *gin.Context, code Code, status int, message string) {
	c.AbortWithStatusJSON(status, Envelope{
		Code:      code,
		Message:   message,
		RequestID: requestID(c),
	})
}
