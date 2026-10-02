// Package config 配置加载、校验与 key 注入。
// 铁律：启动时全量校验，fail fast，配错不留到运行时。
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"llmgateway/capability"
)

// Duration 支持 yaml 中 "45s" 形式的时长。
type Duration time.Duration

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("非法的时长格式 %q: %w", s, err)
	}
	*d = Duration(v)
	return nil
}

// Std 转为标准库 Duration。
func (d Duration) Std() time.Duration { return time.Duration(d) }

// Provider 渠道定义。
type Provider struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"` // 支持 ${ENV_VAR} 引用
}

// Channel 模型在某个渠道上的部署。
type Channel struct {
	Provider string `yaml:"provider"` // 引用 providers 中的 key
	Model    string `yaml:"model"`    // 渠道侧模型名
}

// Model 模型别名定义。
type Model struct {
	Capability     capability.Type `yaml:"capability"`
	Channels       []Channel       `yaml:"channels"`
	FallbackModels []string        `yaml:"fallback_models"` // 同等级替代模型别名，v4 降级用
	Timeout        Duration        `yaml:"timeout"`         // 零值则用能力类型默认超时
}

// RetryConfig 重试参数，v3 启用。
type RetryConfig struct {
	MaxAttempts int      `yaml:"max_attempts"` // 最大尝试次数（含首次）
	BaseBackoff Duration `yaml:"base_backoff"` // 退避基准间隔，指数退避 + 抖动
}

// BreakerConfig 熔断参数，v3 启用。
type BreakerConfig struct {
	FailureRate    float64  `yaml:"failure_rate"`     // 触发断开的失败率阈值，取值 0-1
	WindowSize     int      `yaml:"window_size"`      // 滑动窗口内的请求数
	Cooldown       Duration `yaml:"cooldown"`         // 断开后进入半开前的冷却时长
	HalfOpenProbes int      `yaml:"half_open_probes"` // 半开状态放行的探测请求数
}

// RateLimitConfig 限流参数，v5 启用。
type RateLimitConfig struct {
	RPM  int  `yaml:"rpm"`  // 每分钟请求数上限
	TPM  int  `yaml:"tpm"`  // 每分钟 token 数上限
	Wait bool `yaml:"wait"` // true 排队等待令牌，false 超限直接拒绝
}

// Govern 治理参数，v3/v5 逐个启用。
type Govern struct {
	Retry     RetryConfig     `yaml:"retry"`
	Breaker   BreakerConfig   `yaml:"breaker"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// Config 顶层配置。
type Config struct {
	Providers map[string]Provider `yaml:"providers"`
	Models    map[string]Model    `yaml:"models"`
	Govern    Govern              `yaml:"govern"`
}

// Load 读取并校验配置，任何错误立即返回，绝不产出带病配置。
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置 %s 校验失败: %w", path, err)
	}
	return &cfg, nil
}

// Validate 全量校验：渠道引用、能力类型、降级目标、超时，逐条检查。
func (c *Config) Validate() error {
	if len(c.Providers) == 0 {
		return fmt.Errorf("providers：至少需要配置一个渠道")
	}
	for name, p := range c.Providers {
		if p.BaseURL == "" {
			return fmt.Errorf("providers.%s：base_url 不能为空", name)
		}
		// key 支持 ${ENV_VAR} 引用，展开后必须非空
		p.APIKey = os.ExpandEnv(p.APIKey)
		if p.APIKey == "" {
			return fmt.Errorf("providers.%s：api_key 为空（如使用 ${ENV} 引用，请检查对应环境变量是否已设置）", name)
		}
		c.Providers[name] = p
	}
	if len(c.Models) == 0 {
		return fmt.Errorf("models：至少需要配置一个模型别名")
	}
	for alias, m := range c.Models {
		if !m.Capability.Valid() {
			return fmt.Errorf("models.%s：capability %q 非法，必须是 %v 之一", alias, m.Capability, capability.Types)
		}
		if len(m.Channels) == 0 {
			return fmt.Errorf("models.%s：channels 不能为空，至少需要一个渠道部署", alias)
		}
		for i, ch := range m.Channels {
			if _, ok := c.Providers[ch.Provider]; !ok {
				return fmt.Errorf("models.%s.channels[%d]：渠道 %q 未在 providers 中定义", alias, i, ch.Provider)
			}
			if ch.Model == "" {
				return fmt.Errorf("models.%s.channels[%d]：model 不能为空", alias, i)
			}
		}
		for _, fb := range m.FallbackModels {
			if fb == alias {
				return fmt.Errorf("models.%s：fallback_models 不能包含自身", alias)
			}
			if _, ok := c.Models[fb]; !ok {
				return fmt.Errorf("models.%s：降级目标模型 %q 未在 models 中定义", alias, fb)
			}
		}
		if m.Timeout < 0 {
			return fmt.Errorf("models.%s：timeout 不能为负数", alias)
		}
	}
	return nil
}
