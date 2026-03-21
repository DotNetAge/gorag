package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check if the GoRAG environment is ready",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🔍 \033[1mGoRAG Environment Doctor\033[0m")
		fmt.Println(strings.Repeat("-", 40))

		allReady := true

		// 1. Check Environment Variables
		fmt.Print("🔑 \033[1mAPI Keys:\033[0m ")
		if os.Getenv("DASHSCOPE_API_KEY") != "" {
			fmt.Println("\033[32m[PASSED]\033[0m DASHSCOPE_API_KEY is set")
		} else {
			fmt.Println("\033[31m[FAILED]\033[0m DASHSCOPE_API_KEY is missing")
			allReady = false
		}

		// 2. Check Local Models
		fmt.Println("🤖 \033[1mLocal Models:\033[0m")
		models := []struct {
			name string
			file string
		}{
			{"bge-small-zh-v1.5", "model_fp16.onnx"},
			{"clip-vit-base-patch32", "text_model_fp16.onnx"},
		}

		modelDir := ".test/models"
		for _, m := range models {
			path := filepath.Join(modelDir, m.name, m.file)
			fmt.Printf("   - %-25s ", m.name)
			if _, err := os.Stat(path); err == nil {
				fmt.Println("\033[32m[FOUND]\033[0m")
			} else {
				fmt.Println("\033[31m[MISSING]\033[0m")
				allReady = false
			}
		}

		// 3. Check Persistence Directories
		fmt.Println("📂 \033[1mPersistence Paths:\033[0m")
		paths := []string{".test/vectors", ".test/docsdb"}
		for _, p := range paths {
			fmt.Printf("   - %-25s ", p)
			if err := os.MkdirAll(p, 0755); err == nil {
				fmt.Println("\033[32m[READY]\033[0m")
			} else {
				fmt.Printf("\033[31m[ERROR: %v]\033[0m\n", err)
				allReady = false
			}
		}

		fmt.Println(strings.Repeat("-", 40))
		if allReady {
			fmt.Println("🚀 \033[1;32mEverything is READY! You can now run 'make test'.\033[0m")
		} else {
			fmt.Println("⚠️  \033[1;33mEnvironment is NOT READY. Please resolve the issues above.\033[0m")
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
