package gorag

import (
	"context"
	"fmt"
	"os"

	"github.com/DotNetAge/gorag/v2/indexer"
	"github.com/DotNetAge/gorag/v2/utils"
)

// InitOptions 初始化 RAG 库的配置选项。
type InitOptions struct {
	RagDir    string                 // .rag 库目录
	IndexType string                 // 索引器类型: semantic/graph/hyper
	ModelPath string                 // 本地模型文件路径（与 ModelID 二选一）
	ModelID   string                 // HuggingFace 模型 ID（与 ModelPath 二选一）
	ModelFile string                 // HuggingFace 模型文件名
	Observer  utils.DownloadObserver // 下载进度观察者（可选）
}

// InitResult 初始化结果。
type InitResult struct {
	RagDir      string
	IndexType   string
	ModelPath   string
	IndexerName string
	Config      *Config
	ConfigYAML  string
}

// InitRAG 初始化 RAG 库。
// 包含完整的初始化流程：模型下载、目录创建、配置写入、索引器验证。
func InitRAG(opts InitOptions) (*InitResult, error) {
	// 1. 确定模型路径
	modelPath, err := resolveModelPath(opts)
	if err != nil {
		return nil, err
	}

	// 2. 创建 .rag 目录结构
	if err := Init(opts.RagDir); err != nil {
		return nil, fmt.Errorf("创建 RAG 库失败: %w", err)
	}

	// 3. 写入配置
	cfg, err := LoadConfig(opts.RagDir)
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}
	cfg.Indexer.Type = opts.IndexType
	if modelPath != "" {
		cfg.Embedding.ModelFile = modelPath
	}
	if err := SaveConfig(opts.RagDir, cfg); err != nil {
		return nil, fmt.Errorf("保存配置失败: %w", err)
	}

	// 4. 打开索引器验证（仅当有模型路径时）
	var indexerName string
	if modelPath != "" {
		idx, err := Open(opts.RagDir)
		if err == nil {
			indexerName = idx.Name()
			if closer, ok := idx.(indexer.IndexerCloser); ok {
				closer.Close(context.Background())
			}
		}
	}

	// 5. 返回结果
	_, raw, _ := loadConfigRaw(opts.RagDir)

	return &InitResult{
		RagDir:      opts.RagDir,
		IndexType:   opts.IndexType,
		ModelPath:   modelPath,
		IndexerName: indexerName,
		Config:      cfg,
		ConfigYAML:  raw,
	}, nil
}

// resolveModelPath 确定模型路径。
// 策略：优先使用 ModelID 从 HuggingFace 下载，其次使用 ModelPath。
func resolveModelPath(opts InitOptions) (string, error) {
	if !needsModel(opts.IndexType) {
		return "", nil
	}

	if opts.ModelID != "" {
		modelFile := opts.ModelFile
		if modelFile == "" {
			modelFile = "onnx/model.onnx"
		}
		path, err := utils.CheckAndDownload(opts.ModelID, modelFile, opts.Observer)
		if err != nil {
			return "", fmt.Errorf("模型下载失败: %w", err)
		}
		return path, nil
	}

	if opts.ModelPath != "" {
		if _, err := os.Stat(opts.ModelPath); os.IsNotExist(err) {
			return "", fmt.Errorf("模型文件不存在: %s", opts.ModelPath)
		}
		return opts.ModelPath, nil
	}

	return "", fmt.Errorf("%s 索引器需要模型，请指定模型路径或 HuggingFace 模型 ID", opts.IndexType)
}

// needsModel 判断索引器类型是否需要模型。
func needsModel(indexerType string) bool {
	return indexerType == "hyper" || indexerType == "semantic"
}
