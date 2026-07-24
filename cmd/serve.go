package main

import (
	"os"
	"path/filepath"

	gorag "github.com/DotNetAge/gorag/v2"
	"github.com/DotNetAge/gorag/v2/cmd/webapi"
	"github.com/spf13/cobra"
)

// serve 命令参数
var (
	servePort      string
	serveRAGDir    string
	serveInitDir   string
	serveModelPath string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "启动 Web API 服务",
	Long: `启动 HTTP 服务，提供与 CLI 命令对应的 REST API 端点。

默认监听 8080 端口。若不指定 --rag-dir，自动检测当前目录下的 .rag 库。
使用 --init-dir 可在启动前自动初始化一个新的 RAG 库。

示例:
  grag serve
  grag serve --port 9000
  grag serve --port 9000 --rag-dir /path/to/.rag
  grag serve --port 9000 --init-dir /path/to/project
  grag serve --init-dir . --model-path /path/to/model.onnx`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// 如果指定了初始目录，先执行 initRAG
		if serveInitDir != "" {
			absDir, err := filepath.Abs(serveInitDir)
			if err != nil {
				ui.Error("解析初始目录失败: %v", err)
				os.Exit(1)
			}

			if err := os.MkdirAll(absDir, 0755); err != nil {
				ui.Error("创建初始目录失败: %v", err)
				os.Exit(1)
			}

			ui.Title("初始化 RAG 库")
			ragDir := filepath.Join(absDir, ".rag")
			ui.KeyValue("目录", ragDir)

			var opts gorag.InitOptions
			if serveModelPath != "" {
				absModel, mErr := filepath.Abs(serveModelPath)
				if mErr != nil {
					ui.Error("解析模型路径失败: %v", mErr)
					os.Exit(1)
				}
				ui.KeyValue("模型", absModel)
				opts = gorag.InitOptions{
					RagDir:    ragDir,
					IndexType: "hyper",
					ModelPath: absModel,
				}
			} else {
				opts = gorag.InitOptions{
					RagDir:    ragDir,
					IndexType: "hyper",
					ModelID:   "Xenova/chinese-clip-vit-base-patch16",
					ModelFile: "onnx/model.onnx",
				}
			}
			result, initErr := gorag.InitRAG(opts)
			if initErr != nil {
				ui.Error("初始化 RAG 库失败: %v", initErr)
				os.Exit(1)
			}

			ui.Success("RAG 库初始化成功")
			if result.ModelPath != "" {
				ui.KeyValue("模型", result.ModelPath)
			}

			// 自动将 rag-dir 指向新初始化的库
			if serveRAGDir == "" {
				serveRAGDir = ragDir
			}
		}

		if err := webapi.Start(servePort, serveRAGDir); err != nil {
			ui.Error("服务关闭异常: %v", err)
		}
	},
}

func init() {
	serveCmd.Flags().StringVarP(&servePort, "port", "p", "8080", "监听端口")
	serveCmd.Flags().StringVarP(&serveRAGDir, "rag-dir", "r", "", ".rag 库目录（默认自动检测）")
	serveCmd.Flags().StringVarP(&serveInitDir, "init-dir", "i", "", "初始目录：启动前自动在该目录初始化 RAG 库（不传路径时默认当前工作目录）")
	serveCmd.Flags().Lookup("init-dir").NoOptDefVal = "."
	serveCmd.Flags().StringVarP(&serveModelPath, "model-path", "m", "", "本地 embedding 模型文件路径（仅与 --init-dir 配合使用）")
}
