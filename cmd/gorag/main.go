package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/DotNetAge/gorag/rag"
	"github.com/DotNetAge/gorag/parser/text"
	"github.com/DotNetAge/gorag/vectorstore/memory"
	embedder "github.com/DotNetAge/gorag/embedding/openai"
	llm "github.com/DotNetAge/gochat/pkg/client/openai"
	"github.com/DotNetAge/gochat/pkg/client/base"
)

var (
	apiKey  string
	topK    int
	stream  bool
	verbose bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "gorag",
		Short: "GoRAG - Production-ready RAG framework for Go",
		Long:  "GoRAG is a production-ready RAG (Retrieval-Augmented Generation) framework for Go",
	}

	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "OpenAI API key")
	rootCmd.PersistentFlags().IntVar(&topK, "top-k", 5, "Number of top results to retrieve")
	rootCmd.PersistentFlags().BoolVar(&stream, "stream", false, "Enable streaming response")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose output")

	rootCmd.AddCommand(
		indexCmd(),
		queryCmd(),
		listCmd(),
		serveCmd(),
		exportCmd(),
		importCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func indexCmd() *cobra.Command {
	var file string
	var contentType string

	cmd := &cobra.Command{
		Use:   "index",
		Short: "Index documents",
		Long:  "Index documents into the RAG engine",
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, err := createEngine()
			if err != nil {
				return err
			}

			ctx := context.Background()

			if file != "" {
				// Read from file
				content, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("failed to read file: %w", err)
				}
				
				// Determine content type based on file extension if not specified
				if contentType == "" {
					contentType = getContentTypeFromFile(file)
				}
				
				err = engine.Index(ctx, rag.Source{
					Type:    contentType,
					Content: string(content),
				})
				if err != nil {
					return fmt.Errorf("failed to index file: %w", err)
				}
				fmt.Printf("Indexed file: %s\n", file)
			} else if len(args) > 0 {
				// Read from arguments
				for _, arg := range args {
					err = engine.Index(ctx, rag.Source{
						Type:    contentType,
						Content: arg,
					})
					if err != nil {
						return fmt.Errorf("failed to index content: %w", err)
					}
					fmt.Printf("Indexed content: %s...\n", truncate(arg, 50))
				}
			} else {
				// Read from stdin
				content, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read stdin: %w", err)
				}
				err = engine.Index(ctx, rag.Source{
					Type:    contentType,
					Content: string(content),
				})
				if err != nil {
					return fmt.Errorf("failed to index stdin: %w", err)
				}
				fmt.Println("Indexed content from stdin")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "File to index")
	cmd.Flags().StringVar(&contentType, "type", "text", "Content type (text, pdf, docx, html, json, yaml, excel, ppt, image)")

	return cmd
}

func queryCmd() *cobra.Command {
	var promptTemplate string

	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query the RAG engine",
		Long:  "Query the RAG engine for information",
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, err := createEngine()
			if err != nil {
				return err
			}

			ctx := context.Background()
			question := strings.Join(args, " ")

			if question == "" {
				// Read from stdin
				content, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read stdin: %w", err)
				}
				question = string(content)
			}

			if question == "" {
				return fmt.Errorf("question is required")
			}

			if stream {
				// Streaming mode
				ch, err := engine.QueryStream(ctx, question, rag.QueryOptions{
					TopK:           topK,
					PromptTemplate: promptTemplate,
					Stream:         true,
				})
				if err != nil {
					return err
				}

				fmt.Print("Answer: ")
				for resp := range ch {
					if resp.Error != nil {
						return resp.Error
					}
					fmt.Print(resp.Chunk)
				}
				fmt.Println()
			} else {
				// Non-streaming mode
				resp, err := engine.Query(ctx, question, rag.QueryOptions{
					TopK:           topK,
					PromptTemplate: promptTemplate,
					Stream:         false,
				})
				if err != nil {
					return err
				}

				fmt.Printf("Answer: %s\n", resp.Answer)
				if verbose && len(resp.Sources) > 0 {
					fmt.Println("\nSources:")
					for i, source := range resp.Sources {
						fmt.Printf("[%d] Score: %.4f - %s...\n", i+1, source.Score, truncate(source.Content, 50))
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&promptTemplate, "prompt", "", "Custom prompt template")

	return cmd
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List indexed documents",
		Long:  "List all indexed documents",
		RunE: func(cmd *cobra.Command, args []string) error {
			// This is a placeholder - in a real implementation, we'd need to add a List method to the engine
			fmt.Println("List functionality not implemented yet")
			return nil
		},
	}
}

func serveCmd() *cobra.Command {
	var port string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start a web server",
		Long:  "Start a web server for the RAG engine",
		RunE: func(cmd *cobra.Command, args []string) error {
			// This is a placeholder - in a real implementation, we'd start a web server
			fmt.Printf("Web server functionality not implemented yet (would run on port %s)\n", port)
			return nil
		},
	}

	cmd.Flags().StringVar(&port, "port", "8080", "Port to run the server on")

	return cmd
}

func exportCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export indexed documents",
		Long:  "Export all indexed documents to a file",
		RunE: func(cmd *cobra.Command, args []string) error {
			// This is a placeholder - in a real implementation, we'd need to add an Export method to the engine
			fmt.Printf("Export functionality not implemented yet (would export to %s)\n", file)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "gorag_export.json", "File to export to")

	return cmd
}

func importCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import documents",
		Long:  "Import documents from a file",
		RunE: func(cmd *cobra.Command, args []string) error {
			// This is a placeholder - in a real implementation, we'd need to add an Import method to the engine
			fmt.Printf("Import functionality not implemented yet (would import from %s)\n", file)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "gorag_export.json", "File to import from")

	return cmd
}

func createEngine() (*rag.Engine, error) {
	if apiKey == "" {
		// Try to get from environment
		apiKey = os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required. Use --api-key or set OPENAI_API_KEY environment variable")
		}
	}

	// Create embedder
	embedderInstance, err := embedder.New(embedder.Config{APIKey: apiKey})
	if err != nil {
		return nil, err
	}

	// Create LLM client
	llmInstance, err := llm.New(llm.Config{
		Config: base.Config{
			APIKey: apiKey,
			Model: "gpt-4", // Provide a default model
		},
	})
	if err != nil {
		return nil, err
	}

	engine, err := rag.New(
		rag.WithParser(text.NewParser()),
		rag.WithVectorStore(memory.NewStore()),
		rag.WithEmbedder(embedderInstance),
		rag.WithLLM(llmInstance),
	)
	if err != nil {
		return nil, err
	}

	return engine, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func getContentTypeFromFile(file string) string {
	// Get file extension
	ext := ""
	for i := len(file) - 1; i >= 0; i-- {
		if file[i] == '.' {
			ext = file[i+1:]
			break
		}
	}

	// Map extensions to content types
	switch ext {
	case "txt", "md":
		return "text"
	case "pdf":
		return "pdf"
	case "docx":
		return "docx"
	case "html", "htm":
		return "html"
	case "json":
		return "json"
	case "yaml", "yml":
		return "yaml"
	case "xlsx", "xls":
		return "excel"
	case "pptx", "ppt":
		return "ppt"
	case "jpg", "jpeg", "png", "gif", "webp":
		return "image"
	default:
		return "text"
	}
}
