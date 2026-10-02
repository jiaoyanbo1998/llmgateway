package openaiapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"llmgateway"
	"llmgateway/capability"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model         string         `json:"model"`
	Messages      []chatMessage  `json:"messages"`
	Temperature   *float64       `json:"temperature,omitempty"`
	MaxTokens     int            `json:"max_tokens,omitempty"`
	Stream        bool           `json:"stream,omitempty"`
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage usage `json:"usage"`
}

// ---------- 统一接口转换 ----------

func toWire(model string, req capability.ChatRequest) chatRequest {
	msgs := make([]chatMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = chatMessage{Role: m.Role, Content: m.Content}
	}
	return chatRequest{
		Model:       model,
		Messages:    msgs,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}
}

// ---------- 同步调用 ----------

// Chat 同步对话。
func (c *Client) Chat(ctx context.Context, model string, req capability.ChatRequest) (capability.ChatResponse, error) {
	wire := toWire(model, req)
	respBody, err := c.do(ctx, wire)
	if err != nil {
		return capability.ChatResponse{}, err
	}
	var cr chatResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		// 响应体无法解析：渠道返回了不符合协议的内容，视为渠道故障
		return capability.ChatResponse{}, llmgateway.ChannelDownError(c.name, model, "响应体解析失败", err)
	}
	if len(cr.Choices) == 0 {
		return capability.ChatResponse{}, llmgateway.ChannelDownError(c.name, model, "响应缺少 choices", nil)
	}
	return capability.ChatResponse{
		Content: cr.Choices[0].Message.Content,
		Usage: capability.Usage{
			PromptTokens:     cr.Usage.PromptTokens,
			CompletionTokens: cr.Usage.CompletionTokens,
			TotalTokens:      cr.Usage.TotalTokens,
		},
	}, nil
}

// do 发起请求并处理非 2xx 错误，成功时返回响应体。
func (c *Client) do(ctx context.Context, wire chatRequest) ([]byte, error) {
	body, err := json.Marshal(wire)
	if err != nil {
		return nil, llmgateway.BizError(c.name, wire.Model, "请求序列化失败", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, llmgateway.BizError(c.name, wire.Model, "请求构造失败", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, c.translateTransport(wire.Model, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, c.translateTransport(wire.Model, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.translateStatus(wire.Model, resp.StatusCode, respBody)
	}
	return respBody, nil
}
