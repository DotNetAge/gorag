package gorag

import (
	"context"
	"time"

	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/v2/llm"
	"github.com/DotNetAge/gorag/v2/store/meta"
)

// LLMService 负责 LLM 组件注入与使用记录。
type LLMService struct {
	svc *IndexingService
}

// SetUsageRecorder 为 Summarizer 和 Refiller 设置 token 用量记录回调。
// 在每次成功 LLM 调用后自动将 token 用量写入 meta.db 的 usages 表。
// 通过类型断言检测传入对象是否支持 SetUsageRecorder，不支持时静默跳过。
func (s *LLMService) SetUsageRecorder(summarizer llm.Summarizer, refiller llm.Refiller) {
	recorder := func(ctx context.Context, model string, usage *chat.Usage, label string) {
		if usage == nil {
			return
		}

		u := &meta.Usage{
			Model:            model,
			Label:            label,
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
			CreatedAt:        time.Now(),
		}
		if usage.PromptTokensDetails != nil {
			u.CachedTokens = usage.PromptTokensDetails.CachedTokens
			u.PromptAudioTokens = usage.PromptTokensDetails.AudioTokens
		}
		if usage.CompletionTokensDetails != nil {
			u.ReasoningTokens = usage.CompletionTokensDetails.ReasoningTokens
			u.CompletionAudioTokens = usage.CompletionTokensDetails.AudioTokens
			u.AcceptedPredictionTokens = usage.CompletionTokensDetails.AcceptedPredictionTokens
			u.RejectedPredictionTokens = usage.CompletionTokensDetails.RejectedPredictionTokens
		}

		if err := s.svc.metaStore.SaveUsage(u); err != nil {
			s.svc.logger.Warn("保存 token 用量记录失败", "error", err)
		}
	}

	if sr, ok := summarizer.(interface{ SetUsageRecorder(llm.UsageRecorder) }); ok {
		sr.SetUsageRecorder(recorder)
	}
	if rf, ok := refiller.(interface{ SetUsageRecorder(llm.UsageRecorder) }); ok {
		rf.SetUsageRecorder(recorder)
	}
}

// QueryUsages 查询最近的 token 用量记录，按时间倒序。
// limit 限制返回条数，<= 0 时返回全部。
func (s *LLMService) QueryUsages(limit int) ([]*meta.Usage, error) {
	return s.svc.metaStore.QueryUsages(limit)
}
