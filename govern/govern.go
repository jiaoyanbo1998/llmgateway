// Package govern 治理层：重试/熔断/降级/限流，以中间件形式挂在调用链上。
package govern

import "context"

// Endpoint 链路的最终执行点（真实渠道调用）。
// 响应通过闭包捕获，因此治理中间件对能力类型（chat/image/...）透明。
type Endpoint func(ctx context.Context) error

// Middleware 治理中间件，包装下一跳。
type Middleware func(next Endpoint) Endpoint

// Chain 中间件链，按声明顺序由外向内包裹。
type Chain []Middleware

// Wrap 将链包到 endpoint 上，返回可执行入口。
func (c Chain) Wrap(e Endpoint) Endpoint {
	for i := len(c) - 1; i >= 0; i-- {
		e = c[i](e)
	}
	return e
}
