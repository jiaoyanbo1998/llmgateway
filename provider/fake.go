package provider

import (
	"context"

	"llmgateway/capability"
)

// Fake 测试用渠道，按预设行为响应，用于打通链路和合治理演练。
type Fake struct {
	NameValue string
	Resp      capability.ChatResponse
	Err       error // 非 nil 时每次调用都返回该错误
	OnCall    func(model string, req capability.ChatRequest)
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
