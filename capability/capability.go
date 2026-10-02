package capability

import "time"

// Type 能力类型。
type Type string

const (
	Chat       Type = "chat"
	Image      Type = "image"
	Vision     Type = "vision"
	Video      Type = "video"
	Multimodal Type = "multimodal"
)

// Types 全部合法能力类型，供配置校验使用。
var Types = []Type{Chat, Image, Vision, Video, Multimodal}

func (t Type) Valid() bool {
	switch t {
	case Chat, Image, Vision, Video, Multimodal:
		return true
	}
	return false
}

// DefaultTimeout 按能力类型的默认超时，模型级配置可覆盖。
func DefaultTimeout(t Type) time.Duration {
	switch t {
	case Image:
		return 120 * time.Second
	case Video:
		return 600 * time.Second
	default: // chat / vision / multimodal
		return 60 * time.Second
	}
}

// Message 对话消息，对齐 OpenAI 格式。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 统一 chat 请求。
type ChatRequest struct {
	Messages    []Message
	Temperature *float64
	MaxTokens   int
	Stream      bool
}

// Usage token 用量。
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// ChatResponse 统一 chat 响应。
type ChatResponse struct {
	Content string
	Usage   Usage
}
