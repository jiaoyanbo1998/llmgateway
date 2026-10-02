package openrouter

import (
	"net/http"

	"llmgateway/config"
	"llmgateway/provider/openaiapi"
)

// name 渠道标识，用于错误归属与监控标签。
const name = "openrouter"

// New 从配置创建 openrouter 适配器。hc 为 nil 时使用默认 HTTP 客户端。
func New(p config.Provider, hc *http.Client) *openaiapi.Client {
	return openaiapi.New(name, p.BaseURL, p.APIKey, hc)
}
