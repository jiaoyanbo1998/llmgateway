// Package provider 渠道适配层。每个渠道一个适配器，屏蔽 API 差异。
package provider

import (
	"context"

	"llmgateway/capability"
)

// Provider 渠道适配器接口。v1 只有 Chat，image/video 等随版本扩展。
// 实现方必须把渠道错误翻译为 *llmgateway.Error，漏翻译默认按渠道故障处理。
type Provider interface {
	Name() string
	Chat(ctx context.Context, model string, req capability.ChatRequest) (capability.ChatResponse, error)
}
