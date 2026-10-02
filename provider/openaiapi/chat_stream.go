package openaiapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"llmgateway"
	"llmgateway/capability"
)

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type streamResponse struct {
	Choices []struct {
		Delta chatMessage `json:"delta"`
	} `json:"choices"`
	Usage *usage `json:"usage"` // 仅末块（include_usage 时）携带
}

// ---------- 流式调用 ----------

// ChatStream 流式对话。
// 语义约定：第一个 token 之前的失败直接返回 error（可走治理流程）；
// 开始吐字后的断流不重试不降级，错误通过 StreamChunk.Err 传递给调用方。
func (c *Client) ChatStream(ctx context.Context, model string, req capability.ChatRequest) (<-chan capability.StreamChunk, error) {
	wire := toWire(model, req)
	wire.Stream = true
	wire.StreamOptions = &streamOptions{IncludeUsage: true}

	body, err := json.Marshal(wire)
	if err != nil {
		return nil, llmgateway.BizError(c.name, model, "请求序列化失败", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, llmgateway.BizError(c.name, model, "请求构造失败", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.hc.Do(httpReq)
	if err != nil {
		return nil, c.translateTransport(model, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, c.translateStatus(model, resp.StatusCode, respBody)
	}

	ch := make(chan capability.StreamChunk, 16)
	go c.readSSE(resp.Body, model, ch)
	return ch, nil
}

// readSSE 逐行读取 SSE 流并投递增量块，结束时关闭 channel（释放 body）。
func (c *Client) readSSE(body io.ReadCloser, model string, ch chan<- capability.StreamChunk) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue // 跳过空行、注释行
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return
		}
		var sr streamResponse
		if err := json.Unmarshal([]byte(data), &sr); err != nil {
			// 单块损坏：吐字已开始，按约定不中断，跳过该块
			continue
		}
		var chunk capability.StreamChunk
		if len(sr.Choices) > 0 {
			chunk.Delta = sr.Choices[0].Delta.Content
		}
		if sr.Usage != nil {
			chunk.Usage = &capability.Usage{
				PromptTokens:     sr.Usage.PromptTokens,
				CompletionTokens: sr.Usage.CompletionTokens,
				TotalTokens:      sr.Usage.TotalTokens,
			}
		}
		ch <- chunk
	}
	// 读流结束：区分正常结束与中途断流
	if err := scanner.Err(); err != nil {
		ch <- capability.StreamChunk{Err: llmgateway.ChannelDownError(c.name, model, "流式响应中途断开", err)}
	}
}
