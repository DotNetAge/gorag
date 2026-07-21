package gorag

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveAPIKey 解析 LLM API Key，按四级回退策略：
//
//	1. 环境变量 GORAG_API_KEY（最高优先级，CI/CD 友好）
//	2. .rag/.api_key 文件（grag config llm APIKey 写入位置，权限 600）
//	3. 外部文件引用（cfg.LLM.APIKeyFile，自定义路径）
//	4. 系统 keychain（macOS，可选）
//
// API Key 不进 config.yml，独立存于 .rag/.api_key 文件。
func ResolveAPIKey(ragDir string) (string, error) {
	// 1. 环境变量 GORAG_API_KEY
	if k := os.Getenv(GORAG_API_KEY); k != "" {
		return k, nil
	}

	// 2. .rag/.api_key 文件
	apiKeyFile := filepath.Join(ragDir, ".api_key")
	if data, err := os.ReadFile(apiKeyFile); err == nil {
		if k := strings.TrimSpace(string(data)); k != "" {
			return k, nil
		}
	}

	// 3. 外部文件引用（通过 config.yml 的 llm.api_key_file 配置）
	cfg, err := loadConfig(ragDir)
	if err == nil && cfg.LLM.APIKeyFile != "" {
		if data, err := os.ReadFile(cfg.LLM.APIKeyFile); err == nil {
			if k := strings.TrimSpace(string(data)); k != "" {
				return k, nil
			}
		}
	}

	// 4. 系统 keychain（macOS，可选，暂未实现）

	return "", fmt.Errorf("未找到 API Key，请运行: grag config llm APIKey <key>")
}

// WriteAPIKey 写入 API Key 到 .rag/.api_key 文件（权限 600）。
// API Key 文件权限强制 600（仅 owner 可读写）。
func WriteAPIKey(ragDir, apiKey string) error {
	apiKeyFile := filepath.Join(ragDir, ".api_key")
	if err := os.WriteFile(apiKeyFile, []byte(apiKey), 0600); err != nil {
		return fmt.Errorf("写入 .api_key 失败: %w", err)
	}
	return nil
}

// HasAPIKey 检查是否已设置 API Key（不返回具体值，用于 grag doctor）。
func HasAPIKey(ragDir string) bool {
	_, err := ResolveAPIKey(ragDir)
	return err == nil
}
