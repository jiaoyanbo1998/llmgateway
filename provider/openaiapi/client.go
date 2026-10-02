package openaiapi

import (
	"context"
	"net/http"
	"strings"

	"llmgateway/capability"
)

// Client OpenAI 兼容渠道客户端，实现 provider.Provider 接口。
type Client struct {
	name    string       // 渠道名，如 openrouter，用于错误归属
	baseURL string       // 如 https://openrouter.ai/api/v1
	apiKey  string       // 仅用于请求头，绝不写入日志
	hc      *http.Client // 可注入，便于测试与连接池定制
}

// 确保 Client 实现 provider.Provider（编译期检查）。
var _ interface {
	Name() string
	Chat(context.Context, string, capability.ChatRequest) (capability.ChatResponse, error)
	ChatStream(context.Context, string, capability.ChatRequest) (<-chan capability.StreamChunk, error)
} = (*Client)(nil)

// New 创建客户端。hc 为 nil 时使用 http.DefaultClient；
// 超时由网关通过 context 控制，Client 自身不设超时。
func New(name, baseURL, apiKey string, hc *http.Client) *Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{name: name, baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, hc: hc}
}

// Name 返回渠道名。
func (c *Client) Name() string { return c.name }
