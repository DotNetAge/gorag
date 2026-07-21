package llm

import (
	"fmt"

	"github.com/DotNetAge/gochat/client/openai"
	chat "github.com/DotNetAge/gochat/core"
)

// newChatClient 根据 Config 创建 gochat 客户端。
//
// 设计要点：
//   - 所有 LLM 工具统一通过此函数创建客户端，避免重复代码
//   - 返回 chat.Client 接口，便于上层做 mock 测试
func newChatClient(cfg Config) (chat.Client, error) {
	client, err := openai.NewOpenAI(chat.Config{
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
		BaseURL: cfg.BaseURL,
		Timeout: cfg.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("llm.newChatClient: 创建 gochat 客户端失败: %w", err)
	}
	return client, nil
}
