package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/DotNetAge/gorag"
	"github.com/spf13/cobra"
)

var (
	watchWorkerCount int // worker pool 大小
)

var watchCmd = &cobra.Command{
	Use:   "watch <dataDir> <indexDir>",
	Short: "启动文件监控服务",
	Long: `启动长期运行的文件监控服务，自动索引新增或修改的文件。

使用方法:
  gorag watch ./my-rag ./documents
  gorag watch ./my-rag ./documents --workers 8

特性:
  - 实时监控文件变化
  - 首次启动执行全量索引
  - 支持优雅关闭（Ctrl+C）
  - 使用 worker pool 并发索引
  - 自动记录已索引文件

信号处理:
  - SIGINT (Ctrl+C): 优雅关闭
  - SIGTERM: 优雅关闭`,
	Args: cobra.ExactArgs(2),
	Run:  runWatch,
}

func init() {
	watchCmd.Flags().IntVarP(&watchWorkerCount, "workers", "w", 4, "worker pool 大小")
}

func runWatch(cmd *cobra.Command, args []string) {
	dataDir := args[0]
	indexDir := args[1]

	ui.Title("🔍 GoRAG 文件监控服务")

	// 验证目录
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		ui.Error("RAG 库不存在: %s", dataDir)
		ui.Info("提示: 先使用 'gorag init %s -t fulltext' 初始化", dataDir)
		os.Exit(1)
	}

	if _, err := os.Stat(indexDir); os.IsNotExist(err) {
		ui.Error("索引目录不存在: %s", indexDir)
		os.Exit(1)
	}

	ui.Section("服务配置")
	ui.KeyValue("RAG 库", dataDir)
	ui.KeyValue("监控目录", indexDir)
	ui.KeyValue("Worker 数量", fmt.Sprintf("%d", watchWorkerCount))

	// 创建服务
	ui.Section("启动服务")
	spinner := ui.NewSpinner("正在初始化...")
	spinner.Start()

	svc, err := gorag.NewRAGService(
		dataDir,
		gorag.WithWatchs(indexDir),
		gorag.WithWorkerCount(watchWorkerCount),
	)
	if err != nil {
		spinner.Stop()
		ui.Error("服务初始化失败: %v", err)
		os.Exit(1)
	}

	spinner.Stop()
	ui.Success("服务初始化成功")

	// 启动监控（后台运行）
	errCh := make(chan error, 1)
	go func() {
		if err := svc.Watch(); err != nil {
			errCh <- err
		}
	}()

	ui.Success("文件监控服务已启动")
	ui.Info("按 Ctrl+C 停止服务")

	// 监听中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		// 收到中断信号
		ui.Info("\n收到关闭信号，正在停止服务...")
		
		// 优雅关闭
		if err := svc.Stop(); err != nil {
			ui.Error("服务关闭时出错: %v", err)
			os.Exit(1)
		}
		
		ui.Success("服务已优雅关闭")
		
	case err := <-errCh:
		// 监控服务出错
		ui.Error("服务运行错误: %v", err)
		os.Exit(1)
	}
}
