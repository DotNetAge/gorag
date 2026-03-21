package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/spf13/cobra"
)

var (
	downloadModelName string
	downloadOutputDir string
)

// renderSmoothProgressBar 绘制一个亚像素级丝滑的图形化进度条
func renderSmoothProgressBar(fileName string, downloaded, total int64) {
	const barLength = 30
	var percent float64
	if total > 0 {
		percent = float64(downloaded) / float64(total)
	}

	// 8阶微动块，实现 Retina 级别的平滑感
	blocks := []string{" ", "▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}
	
	totalSteps := float64(barLength) * percent
	fullBlocks := int(totalSteps)
	remainder := int((totalSteps - float64(fullBlocks)) * 8)

	// 1. 构建已完成的完整块
	completed := strings.Repeat("█", fullBlocks)
	
	// 2. 构建正在流动的微动块
	var partial string
	if fullBlocks < barLength {
		partial = blocks[remainder]
	}
	
	// 3. 构建剩余的空白位
	remaining := strings.Repeat("░", barLength-fullBlocks-len(partial))
	if barLength-fullBlocks-len(partial) < 0 {
		remaining = ""
	}

	// 限制文件名显示
	displayFile := fileName
	if len(displayFile) > 20 {
		displayFile = "..." + displayFile[len(displayFile)-17:]
	}

	// 采用青色/绿色色调增强“图形化”工业感
	// \033[32m 是绿色, \033[0m 是重置
	fmt.Printf("\r\033[36m%-20s\033[0m [\033[32m%s%s\033[90m%s\033[0m] %5.1f%%", 
		displayFile, completed, partial, remaining, percent*100)
}

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download local ONNX models with retina-smooth progress",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("🚀 \033[1mInitializing High-Res Download:\033[0m %s\n", downloadModelName)
		fmt.Printf("📂 \033[90mGrounding Path:\033[0m %s\n", downloadOutputDir)
		
		dl := embedding.NewDownloader(downloadOutputDir)
		
		callback := func(modelName, fileName string, downloaded, total int64) {
			renderSmoothProgressBar(fileName, downloaded, total)
			if downloaded == total && total > 0 {
				fmt.Print(" \033[32m[COMPLETE]\033[0m\n")
			}
		}

		modelPath, err := dl.DownloadModel(downloadModelName, callback)
		if err != nil {
			log.Fatalf("\n\033[31m❌ Download stalled: %v\033[0m\n", err)
		}
		
		fmt.Printf("\n✨ \033[1;32mAsset Grounded Successfully:\033[0m %s\n", modelPath)
	},
}

func init() {
	downloadCmd.Flags().StringVarP(&downloadModelName, "model", "m", "bge-small-zh-v1.5", "Model name to download")
	downloadCmd.Flags().StringVarP(&downloadOutputDir, "output", "o", ".test/models", "Output directory")
	rootCmd.AddCommand(downloadCmd)
}
