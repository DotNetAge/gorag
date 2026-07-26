package llm

import (
	"fmt"
	"time"
)

// =====================================================================
// LLM 工具公共配置
// =====================================================================

// Config 是 Summarizer / Refiller 等 LLM 工具的通用配置。
//
// 设计要点：
//   - 与具体 LLM 客户端解耦，仅描述调用参数
//   - 调用方负责从环境变量、.rag 配置或 keychain 中读取真实密钥
//   - Timeout 为 0 时使用默认 10 分钟
type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// validate 校验必填参数。
func (c Config) validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("llm.Config: APIKey 不能为空")
	}
	if c.BaseURL == "" {
		return fmt.Errorf("llm.Config: BaseURL 不能为空")
	}
	if c.Model == "" {
		return fmt.Errorf("llm.Config: Model 不能为空")
	}
	return nil
}

// withDefaults 为 0 值填充默认值。
func (c Config) withDefaults() Config {
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Minute
	}
	return c
}
