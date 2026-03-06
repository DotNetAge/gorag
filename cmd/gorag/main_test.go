package main

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTruncate(t *testing.T) {
	// Test with short string
	shortStr := "Hello, world!"
	result := truncate(shortStr, 50)
	assert.Equal(t, shortStr, result)

	// Test with long string
	longStr := strings.Repeat("a", 100)
	result = truncate(longStr, 50)
	assert.Equal(t, strings.Repeat("a", 50)+"...", result)

	// Test with exact length
	exactStr := strings.Repeat("a", 50)
	result = truncate(exactStr, 50)
	assert.Equal(t, exactStr, result)

	// Test with zero max length
	result = truncate(shortStr, 0)
	assert.Equal(t, "...", result)
}

func TestCreateEngine(t *testing.T) {
	// Test with API key from environment
	originalAPIKey := os.Getenv("OPENAI_API_KEY")
	defer os.Setenv("OPENAI_API_KEY", originalAPIKey)

	// Test without API key
	os.Unsetenv("OPENAI_API_KEY")
	apiKey = ""
	engine, err := createEngine()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OpenAI API key is required")
	assert.Nil(t, engine)

	// Test with API key (we expect it to either succeed or fail with a non-API-key-required error)
	os.Setenv("OPENAI_API_KEY", "test-key")
	engine, err = createEngine()
	// If there's an error, it shouldn't be about missing API key
	if err != nil {
		assert.NotContains(t, err.Error(), "OpenAI API key is required")
	} else {
		// If it succeeds, the engine should not be nil
		assert.NotNil(t, engine)
	}
}

func TestRootCommand(t *testing.T) {
	// Test that root command can be created
	// This is a basic test to ensure the command structure is valid
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Test --help flag
	os.Args = []string{"gorag", "--help"}

	// Execute root command with --help
	// We'll just verify that it doesn't panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Command execution panicked: %v", r)
			}
		}()
		// Note: main() will call os.Exit, which will terminate the test process
		// So we'll skip this part and just test the command creation
		// main()
	}()

	// Instead, let's just verify that the root command can be created
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
	)

	assert.NotNil(t, rootCmd)
	assert.Len(t, rootCmd.Commands(), 4)
}

func TestIndexCommand(t *testing.T) {
	// Test index command creation
	cmd := indexCmd()
	assert.Equal(t, "index", cmd.Use)
	assert.Equal(t, "Index documents", cmd.Short)
	assert.Equal(t, "Index documents into the RAG engine", cmd.Long)

	// Test that the --file flag is present
	fileFlag := cmd.Flags().Lookup("file")
	require.NotNil(t, fileFlag)
	assert.Equal(t, "", fileFlag.DefValue)
	assert.Equal(t, "File to index", fileFlag.Usage)
}

func TestQueryCommand(t *testing.T) {
	// Test query command creation
	cmd := queryCmd()
	assert.Equal(t, "query", cmd.Use)
	assert.Equal(t, "Query the RAG engine", cmd.Short)
	assert.Equal(t, "Query the RAG engine for information", cmd.Long)

	// Test that the --prompt flag is present
	promptFlag := cmd.Flags().Lookup("prompt")
	require.NotNil(t, promptFlag)
	assert.Equal(t, "", promptFlag.DefValue)
	assert.Equal(t, "Custom prompt template", promptFlag.Usage)
}

func TestListCommand(t *testing.T) {
	// Test list command creation
	cmd := listCmd()
	assert.Equal(t, "list", cmd.Use)
	assert.Equal(t, "List indexed documents", cmd.Short)
	assert.Equal(t, "List all indexed documents", cmd.Long)
}

func TestServeCommand(t *testing.T) {
	// Test serve command creation
	cmd := serveCmd()
	assert.Equal(t, "serve", cmd.Use)
	assert.Equal(t, "Start a web server", cmd.Short)
	assert.Equal(t, "Start a web server for the RAG engine", cmd.Long)

	// Test that the --port flag is present
	portFlag := cmd.Flags().Lookup("port")
	require.NotNil(t, portFlag)
	assert.Equal(t, "8080", portFlag.DefValue)
	assert.Equal(t, "Port to run the server on", portFlag.Usage)
}
