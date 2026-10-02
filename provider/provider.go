// Package provider 渠道适配层。每个渠道一个适配器，屏蔽 API 差异。
package provider

import (
	"context"

	"llmgateway/capability"
)

// Provider 渠道适配器接口。v2 覆盖 chat（含流式），image/video 等随版本扩展。
// 实现方必须把渠道错误翻译为 *llmgateway.Error，漏翻译默认按渠道故障处理。
type Provider interface {
	Name() string
	Chat(ctx context.Context, model string, req capability.ChatRequest) (capability.ChatResponse, error)
	// ChatStream 流式对话。第一个 token 之前的失败直接返回 error；
	// 开始吐字后的断流通过 StreamChunk.Err 传递。
	ChatStream(ctx context.Context, model string, req capability.ChatRequest) (<-chan capability.StreamChunk, error)
}
