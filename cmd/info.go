package main

import (
	"fmt"
	"os"
	"strings"

	gorag "github.com/DotNetAge/gorag/v2"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "查看 RAG 库信息",
	Args:  cobra.NoArgs,
	Run:   runInfo,
}

func runInfo(cmd *cobra.Command, args []string) {
	ragDir, err := findRAGInCWD()
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	svc, err := gorag.NewRAGService(ragDir)
	if err != nil {
		ui.Error("打开 RAG 库失败: %v", err)
		os.Exit(1)
	}
	defer svc.Stop()

	info, err := svc.Info()
	if err != nil {
		ui.Error("获取信息失败: %v", err)
		os.Exit(1)
	}

	ui.Title("RAG 库信息")

	// 配置
	ui.Section("配置")
	ui.KeyValue("版本", fmt.Sprintf("%d", info.Config.Version))
	ui.KeyValue("绝对路径", info.AbsPath)

	if strings.TrimSpace(info.ConfigYAML) != "" {
		fmt.Printf("\n  %sconfig.yml:%s\n", ui.colors.Highlight, ui.colors.Reset)
		for _, line := range strings.Split(info.ConfigYAML, "\n") {
			if line != "" {
				fmt.Printf("    %s%s%s\n", ui.colors.Dim, line, ui.colors.Reset)
			}
		}
	}

	// 存储
	ui.Section("存储")
	totalSize := info.Sizes["total"]
	ui.KeyValue("总大小", formatBytes(totalSize))

	subDirs := []string{"vectors", "graphs", "logs"}
	for _, sub := range subDirs {
		if size, ok := info.Sizes[sub]; ok && size > 0 {
			ui.KeyValue("  "+sub+"/", formatBytes(size))
		}
	}

	// 索引统计
	ui.Section("索引统计")
	if info.VectorCount >= 0 {
		ui.KeyValue("向量索引 (vectors)", fmt.Sprintf("%d 条", info.VectorCount))
	} else {
		ui.KeyValue("向量索引 (vectors)", "未创建")
	}
	if info.GraphNodes >= 0 {
		ui.KeyValue("图索引 (graphs)", fmt.Sprintf("%d 个节点, %d 条边", info.GraphNodes, info.GraphEdges))
	} else {
		ui.KeyValue("图索引 (graphs)", "未创建")
	}

	fmt.Println()
}
