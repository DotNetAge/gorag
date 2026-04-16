package main

import (
	"context"
	"fmt"
	"os"

	"github.com/DotNetAge/gorag"
	"github.com/DotNetAge/gorag/core"
	"github.com/spf13/cobra"
)

var (
	addFile string // 文件路径
)

var addCmd = &cobra.Command{
	Use:   "add <dataDir> [text]",
	Short: "添加内容到 RAG 库",
	Long: `向 RAG 库添加文本内容或文件进行索引。

使用方法:
  gorag ./my-rag add "这是要索引的文本"
  gorag ./my-rag add -f document.txt

注意:
  - 文本内容和文件路径不能同时指定
  - 文件必须是文本格式（支持 UTF-8 编码）`,
	Args: cobra.MinimumNArgs(1),
	Run:  runAdd,
}

func init() {
	addCmd.Flags().StringVarP(&addFile, "file", "f", "", "文件路径")
}

func runAdd(cmd *cobra.Command, args []string) {
	dataDir := args[0]

	// 获取文本内容（如果有）
	var textContent string
	if len(args) > 1 {
		textContent = args[1]
	}

	// 验证输入
	if err := validateAddInput(textContent, addFile); err != nil {
		ui.Error("%s", err.Error())
		os.Exit(1)
	}

	// 打开 RAG 库
	spinner := ui.NewSpinner("正在加载 RAG 库...")
	spinner.Start()

	idx, err := gorag.Open(dataDir)
	if err != nil {
		spinner.Stop()
		ui.Error("打开 RAG 库失败: %v", err)
		os.Exit(1)
	}

	spinner.Stop()
	ui.Success("RAG 库已加载")

	// 根据参数添加内容
	ctx := context.Background()

	if addFile != "" {
		addFromFile(ctx, idx, addFile)
	} else {
		addFromText(ctx, idx, textContent)
	}

	// 关闭 RAG 库
	if closer, ok := idx.(interface{ Close() error }); ok {
		closer.Close()
	}
}

// validateAddInput 验证输入参数
func validateAddInput(text, file string) error {
	if text == "" && file == "" {
		return fmt.Errorf("必须指定文本内容或文件路径")
	}
	if text != "" && file != "" {
		return fmt.Errorf("不能同时指定文本内容和文件路径，请选择其中一种方式")
	}
	return nil
}

// addFromText 从文本内容添加
func addFromText(ctx context.Context, idx core.Indexer, text string) {
	ui.Info("添加文本内容 (%d 字符)", len(text))

	spinner := ui.NewSpinner("正在索引...")
	spinner.Start()

	chunk, err := idx.Add(ctx, text)
	if err != nil {
		spinner.Stop()
		ui.Error("索引失败: %v", err)
		os.Exit(1)
	}

	spinner.Stop()
	ui.Success("索引成功")

	ui.Section("索引信息")
	ui.KeyValue("Chunk ID", chunk.ID)
	ui.KeyValue("文档 ID", chunk.DocID)
	ui.KeyValue("内容长度", fmt.Sprintf("%d 字符", len(chunk.Content)))
}

// addFromFile 从文件添加
func addFromFile(ctx context.Context, idx core.Indexer, filePath string) {
	// 检查文件是否存在
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		ui.Error("文件不存在: %s", filePath)
		os.Exit(1)
	}
	if err != nil {
		ui.Error("无法访问文件: %v", err)
		os.Exit(1)
	}

	// 检查是否为目录
	if info.IsDir() {
		ui.Error("不支持目录索引，请指定文件路径")
		ui.Info("提示: 使用 'gorag %s add -f <file>' 索引单个文件", ".")
		os.Exit(1)
	}

	ui.Info("添加文件: %s", filePath)
	ui.Item("文件大小: %s", formatBytes(info.Size()))

	spinner := ui.NewSpinner("正在索引...")
	spinner.Start()

	chunk, err := idx.AddFile(ctx, filePath)
	if err != nil {
		spinner.Stop()
		ui.Error("索引失败: %v", err)
		os.Exit(1)
	}

	spinner.Stop()
	ui.Success("索引成功")

	ui.Section("索引信息")
	ui.KeyValue("Chunk ID", chunk.ID)
	ui.KeyValue("文档 ID", chunk.DocID)
	ui.KeyValue("文件路径", filePath)
	ui.KeyValue("内容长度", fmt.Sprintf("%d 字符", len(chunk.Content)))
}
