package provider

import (
	"context"

	"llmgateway/capability"
)

// Fake 测试用渠道，按预设行为响应，用于打通链路和合治理演练。
type Fake struct {
	NameValue    string
	Resp         capability.ChatResponse
	Err          error                    // 非 nil 时 Chat 每次都返回该错误
	StreamChunks []capability.StreamChunk // ChatStream 依次吐出的块
	StreamErr    error                    // 非 nil 时 ChatStream 在吐字前直接失败
	OnCall       func(model string, req capability.ChatRequest)
}

func (f *Fake) Name() string { return f.NameValue }

func (f *Fake) Chat(ctx context.Context, model string, req capability.ChatRequest) (capability.ChatResponse, error) {
	if f.OnCall != nil {
		f.OnCall(model, req)
	}
	if f.Err != nil {
		return capability.ChatResponse{}, f.Err
	}
	return f.Resp, nil
}

// ChatStream 返回预设的流式块序列；StreamErr 非 nil 时直接返回错误（模拟吐字前失败）。
func (f *Fake) ChatStream(ctx context.Context, model string, req capability.ChatRequest) (<-chan capability.StreamChunk, error) {
	if f.StreamErr != nil {
		return nil, f.StreamErr
	}
	ch := make(chan capability.StreamChunk, len(f.StreamChunks)+1)
	go func() {
		defer close(ch)
		for _, c := range f.StreamChunks {
			select {
			case <-ctx.Done():
				return
			case ch <- c:
			}
		}
	}()
	return ch, nil
}
