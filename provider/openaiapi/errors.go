package openaiapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"llmgateway"
)

// apiError 渠道错误响应体，message 用于报错信息。
type apiError struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ---------- 错误翻译表 ----------
// 铁律：任何错误都必须翻译为三分类之一，不得漏。

// translateStatus 按 HTTP 状态码翻译渠道错误。
func (c *Client) translateStatus(model string, status int, body []byte) *llmgateway.Error {
	msg := http.StatusText(status)
	var ae apiError
	if json.Unmarshal(body, &ae) == nil && ae.Error.Message != "" {
		msg = ae.Error.Message
	}
	switch {
	// 4xx 多为调用方问题：参数错误、key 无效、余额不足、审核拦截——业务错误
	case status == http.StatusBadRequest,
		status == http.StatusUnauthorized,
		status == http.StatusForbidden,
		status == http.StatusNotFound,
		status == http.StatusRequestEntityTooLarge,
		status == http.StatusUnprocessableEntity:
		return llmgateway.BizError(c.name, model, fmt.Sprintf("渠道返回 %d: %s", status, msg), nil)
	// 429 限流、408 超时、5xx 服务端错误——可重试
	case status == http.StatusTooManyRequests,
		status == http.StatusRequestTimeout,
		status >= 500:
		return llmgateway.RetryableError(c.name, model, fmt.Sprintf("渠道返回 %d: %s", status, msg), nil)
	// 其余 4xx 按业务错误处理
	case status >= 400:
		return llmgateway.BizError(c.name, model, fmt.Sprintf("渠道返回 %d: %s", status, msg), nil)
	default:
		return llmgateway.ChannelDownError(c.name, model, fmt.Sprintf("渠道返回异常状态 %d", status), nil)
	}
}

// translateTransport 翻译传输层错误（连接拒绝、DNS、TLS、超时等）——渠道故障。
func (c *Client) translateTransport(model string, err error) *llmgateway.Error {
	return llmgateway.ChannelDownError(c.name, model, "传输层错误", err)
}
