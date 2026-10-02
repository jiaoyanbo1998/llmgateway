package llmgateway

import (
	"context"
	"errors"
	"fmt"
)

// ErrKind 统一错误分类，是重试/熔断/降级所有治理判定的唯一依据。
type ErrKind int

const (
	// ErrKindBiz 业务错误：参数错误、审核拦截、余额不足、key 无效。
	// 不重试、不降级、不计熔断，直接抛给调用方。
	ErrKindBiz ErrKind = iota + 1
	// ErrKindRetryable 可重试：429、5xx、网络抖动。
	// 同渠道内重试，耗尽后升级为 ErrKindChannelDown。
	ErrKindRetryable
	// ErrKindChannelDown 渠道故障：超时、连接拒绝、重试耗尽。
	// 计熔断、触发降级。
	ErrKindChannelDown
)

// String 返回分类的机器可读标识，用于日志和 metrics 标签。
func (k ErrKind) String() string {
	switch k {
	case ErrKindBiz:
		return "biz"
	case ErrKindRetryable:
		return "retryable"
	case ErrKindChannelDown:
		return "channel_down"
	}
	return "unknown"
}

// Error 网关统一错误。适配器必须把渠道原始错误翻译为 *Error。
type Error struct {
	Kind     ErrKind
	Provider string // 出错渠道，如 openrouter
	Model    string // 渠道侧模型名，如 deepseek/deepseek-chat
	Msg      string
	Err      error // 原始错误
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s/%s: %s: %v", e.Kind, e.Provider, e.Model, e.Msg, e.Err)
	}
	return fmt.Sprintf("[%s] %s/%s: %s", e.Kind, e.Provider, e.Model, e.Msg)
}

func (e *Error) Unwrap() error { return e.Err }

func NewError(kind ErrKind, provider, model, msg string, err error) *Error {
	return &Error{Kind: kind, Provider: provider, Model: model, Msg: msg, Err: err}
}

// BizError 构造业务错误：不重试、不降级、不计熔断。
func BizError(provider, model, msg string, err error) *Error {
	return NewError(ErrKindBiz, provider, model, msg, err)
}

// RetryableError 构造可重试错误：由重试中间件在同渠道内重试。
func RetryableError(provider, model, msg string, err error) *Error {
	return NewError(ErrKindRetryable, provider, model, msg, err)
}

// ChannelDownError 构造渠道故障错误：计熔断、触发降级。
func ChannelDownError(provider, model, msg string, err error) *Error {
	return NewError(ErrKindChannelDown, provider, model, msg, err)
}

// KindOf 返回错误的分类。
// 铁律：未被翻译的错误（非 *Error）默认按渠道故障处理——宁可误熔断，不可漏熔断。
func KindOf(err error) ErrKind {
	var ge *Error
	if errors.As(err, &ge) {
		return ge.Kind
	}
	return ErrKindChannelDown
}

// IsBiz 判断是否为业务错误。
func IsBiz(err error) bool { return KindOf(err) == ErrKindBiz }

// IsRetryable 判断是否为可重试错误。
func IsRetryable(err error) bool { return KindOf(err) == ErrKindRetryable }

// IsChannelDown 判断是否为渠道故障。
func IsChannelDown(err error) bool { return KindOf(err) == ErrKindChannelDown }

// TimeoutError 将 context 超时包装为渠道故障。
func TimeoutError(provider, model string, err error) *Error {
	if errors.Is(err, context.DeadlineExceeded) {
		return ChannelDownError(provider, model, "请求超时", err)
	}
	if errors.Is(err, context.Canceled) {
		return ChannelDownError(provider, model, "请求已取消", err)
	}
	return nil
}
