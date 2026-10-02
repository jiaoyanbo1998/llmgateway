package llmgateway

import (
	"context"
	"fmt"

	"llmgateway/capability"
	"llmgateway/config"
	"llmgateway/govern"
	"llmgateway/provider"
)

// Gateway 对外门面。调用方只指定模型别名，其余由配置和治理链接管。
type Gateway struct {
	cfg       *config.Config
	providers map[string]provider.Provider
	chain     govern.Chain
}

// Option 可选配置。
type Option func(*Gateway)

// WithProvider 注册渠道适配器，名字对应配置 providers 中的 key。
func WithProvider(name string, p provider.Provider) Option {
	return func(g *Gateway) { g.providers[name] = p }
}

// WithMiddleware 追加治理中间件（v3 起逐个挂入）。
func WithMiddleware(mw ...govern.Middleware) Option {
	return func(g *Gateway) { g.chain = append(g.chain, mw...) }
}

// New 创建网关。cfg 必须先通过 config.Load 校验。
func New(cfg *config.Config, opts ...Option) (*Gateway, error) {
	g := &Gateway{cfg: cfg, providers: map[string]provider.Provider{}}
	for _, opt := range opts {
		opt(g)
	}
	// 配置引用的每个渠道都必须注册了适配器
	for alias, m := range cfg.Models {
		for _, ch := range m.Channels {
			if _, ok := g.providers[ch.Provider]; !ok {
				return nil, fmt.Errorf("models.%s：渠道 %q 已配置但未注册适配器", alias, ch.Provider)
			}
		}
	}
	return g, nil
}

// candidate 候选链路上的一跳：渠道 + 渠道侧模型名。
type candidate struct {
	provider provider.Provider
	model    string
}

// candidates 解析模型别名的候选链路。
// v1 只返回第一候选；v4 降级将返回完整链路（同模型跨渠道 → 同等级模型）。
func (g *Gateway) candidates(alias string) ([]candidate, error) {
	m, ok := g.cfg.Models[alias]
	if !ok {
		return nil, BizError("", alias, "模型别名未在配置中定义", nil)
	}
	ch := m.Channels[0]
	return []candidate{{provider: g.providers[ch.Provider], model: ch.Model}}, nil
}

// timeoutOf 模型超时：模型级配置优先，否则按能力类型默认。
func (g *Gateway) timeoutOf(alias string) (capability.Type, config.Duration) {
	m := g.cfg.Models[alias]
	if m.Timeout > 0 {
		return m.Capability, m.Timeout
	}
	return m.Capability, config.Duration(capability.DefaultTimeout(m.Capability))
}

// Chat 同步对话。
func (g *Gateway) Chat(ctx context.Context, alias string, req capability.ChatRequest) (capability.ChatResponse, error) {
	cands, err := g.candidates(alias)
	if err != nil {
		return capability.ChatResponse{}, err
	}
	c := cands[0]

	_, timeout := g.timeoutOf(alias)
	ctx, cancel := context.WithTimeout(ctx, timeout.Std())
	defer cancel()

	var resp capability.ChatResponse
	endpoint := func(ctx context.Context) error {
		r, err := c.provider.Chat(ctx, c.model, req)
		if err != nil {
			if te := TimeoutError(c.provider.Name(), c.model, err); te != nil {
				return te
			}
			return err // 适配器已翻译；未翻译的由 KindOf 默认按渠道故障处理
		}
		resp = r
		return nil
	}
	if err := g.chain.Wrap(endpoint)(ctx); err != nil {
		return capability.ChatResponse{}, err
	}
	return resp, nil
}

// ChatStream 流式对话。
// 吐字前的失败直接返回 error（走治理流程）；吐字后的断流通过
// StreamChunk.Err 传递，不重试不降级。调用方应消费完整个 channel。
func (g *Gateway) ChatStream(ctx context.Context, alias string, req capability.ChatRequest) (<-chan capability.StreamChunk, error) {
	cands, err := g.candidates(alias)
	if err != nil {
		return nil, err
	}
	c := cands[0]

	_, timeout := g.timeoutOf(alias)
	ctx, cancel := context.WithTimeout(ctx, timeout.Std())

	var stream <-chan capability.StreamChunk
	endpoint := func(ctx context.Context) error {
		s, err := c.provider.ChatStream(ctx, c.model, req)
		if err != nil {
			if te := TimeoutError(c.provider.Name(), c.model, err); te != nil {
				return te
			}
			return err
		}
		stream = s
		return nil
	}
	if err := g.chain.Wrap(endpoint)(ctx); err != nil {
		cancel()
		return nil, err
	}

	// 转发流并在结束时 cancel，释放超时资源；
	// 调用方放弃消费时 ctx 超时/取消会终止转发，goroutine 不泄漏。
	out := make(chan capability.StreamChunk, 16)
	go func() {
		defer cancel()
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-stream:
				if !ok {
					return
				}
				select {
				case out <- chunk:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}
