package llm

import (
	"context"
	"time"

	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/v2/logging"
)

// UsageRecorder 记录 LLM 调用 token 用量的回调。
// model 为本次调用的模型名，usage 可能为 nil（部分提供商不返回用量信息）。
// label 标识调用方（如 "Summarizer(单条)" / "Refiller"）。
type UsageRecorder func(ctx context.Context, model string, usage *chat.Usage, label string)

// timedChat 包装 chat.Client.Chat，自动统计耗时并输出 Info 日志。
// label 用于标识调用方（如 "Summarizer" / "Refiller"）。
// recorder 为可选参数，非 nil 时在成功后记录 token 用量。
func timedChat(ctx context.Context, client chat.Client, messages []chat.Message, logger logging.Logger, label string, recorder UsageRecorder) (*chat.Response, error) {
	start := time.Now()
	resp, err := client.Chat(ctx, messages)
	duration := time.Since(start)
	ms := duration.Round(time.Millisecond)
	if err != nil {
		logger.Warn("LLM 调用失败",
			"label", label,
			"duration", ms.String())
		return nil, err
	}
	logger.Info("LLM 调用完成",
		"label", label,
		"duration", ms.String())

	if recorder != nil && resp.Usage != nil {
		recorder(ctx, resp.Model, resp.Usage, label)
	}

	return resp, nil
}
