package llm

import (
	"context"
	"time"

	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/v2/logging"
)

// timedChat 包装 chat.Client.Chat，自动统计耗时并输出 Info 日志。
// label 用于标识调用方（如 "Summarizer" / "Refiller"）。
func timedChat(ctx context.Context, client chat.Client, messages []chat.Message, logger logging.Logger, label string) (*chat.Response, error) {
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
	return resp, nil
}
