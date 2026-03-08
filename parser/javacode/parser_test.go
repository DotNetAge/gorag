package javacode

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Parse(t *testing.T) {
	parser := NewParser()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	javaContent := []byte(`public class HelloWorld {
    public static void main(String[] args) {
        System.out.println("Hello, World!");
    }
}`)

	r := bytes.NewReader(javaContent)
	chunks, err := parser.Parse(ctx, r)
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_ParseWithCallback(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	javaContent := []byte(`public class Calculator {
    public int add(int a, int b) {
        return a + b;
    }
    
    public int subtract(int a, int b) {
        return a - b;
    }
}`)

	var chunkCount int
	var foundMethod bool

	err := parser.ParseWithCallback(ctx, bytes.NewReader(javaContent), func(chunk core.Chunk) error {
		chunkCount++
		assert.NotEmpty(t, chunk.ID)
		assert.Contains(t, chunk.Metadata["type"], "javacode")
		if chunk.Metadata["chunk_type"] == "method" {
			foundMethod = true
		}
		return nil
	})

	require.NoError(t, err)
	assert.Greater(t, chunkCount, 0)
	assert.True(t, foundMethod, "Should find at least one method")
}

func TestParser_MethodExtraction(t *testing.T) {
	parser := NewParser()
	parser.SetExtractMethods(true)
	ctx := context.Background()

	javaContent := []byte(`public class Service {
    public String greet(String name) {
        return "Hello, " + name;
    }
}`)

	var foundGreet bool
	err := parser.ParseWithCallback(ctx, bytes.NewReader(javaContent), func(chunk core.Chunk) error {
		if chunk.Metadata["chunk_type"] == "method" {
			if chunk.Metadata["method_name"] == "greet" {
				foundGreet = true
			}
		}
		return nil
	})

	require.NoError(t, err)
	assert.True(t, foundGreet, "Should extract greet method")
}

func TestParser_ClassExtraction(t *testing.T) {
	parser := NewParser()
	parser.SetExtractClasses(true)
	ctx := context.Background()

	javaContent := []byte(`public class UserService {
    // Class content
}`)

	var foundUserService bool
	err := parser.ParseWithCallback(ctx, bytes.NewReader(javaContent), func(chunk core.Chunk) error {
		if chunk.Metadata["chunk_type"] == "class" {
			if chunk.Metadata["class_name"] == "UserService" {
				foundUserService = true
			}
		}
		return nil
	})

	require.NoError(t, err)
	assert.True(t, foundUserService, "Should find UserService class")
}

func TestParser_EmptyCode(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	javaContent := []byte(``)
	chunks, err := parser.Parse(ctx, bytes.NewReader(javaContent))
	require.NoError(t, err)
	_ = chunks
}

func TestParser_LargeCode(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(100)
	ctx := context.Background()

	var sb strings.Builder
	sb.WriteString("public class LargeClass {\n")
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&sb, "    public void method%d() {\n        System.out.println(\"Method %d\");\n    }\n\n", i, i)
	}
	sb.WriteString("}")

	chunks, err := parser.Parse(ctx, strings.NewReader(sb.String()))
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_ContextCancellation(t *testing.T) {
	parser := NewParser()
	ctx, cancel := context.WithCancel(context.Background())

	var sb strings.Builder
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&sb, "public class Class%d { void method() {} }\n", i)
	}

	cancel()
	_, err := parser.Parse(ctx, strings.NewReader(sb.String()))
	assert.Error(t, err)
}

func TestParser_CallbackError(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(20)
	ctx := context.Background()

	javaContent := []byte(`// Comment 1
// Comment 2
int x = 10;`)

	err := parser.ParseWithCallback(ctx, bytes.NewReader(javaContent), func(chunk core.Chunk) error {
		return assert.AnError
	})
	assert.Error(t, err)
}

func TestParser_ChunkConfiguration(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(600)
	parser.SetChunkOverlap(60)

	assert.Equal(t, 600, parser.chunkSize)
	assert.Equal(t, 60, parser.chunkOverlap)
}

func TestParser_ConfigurationOptions(t *testing.T) {
	parser := NewParser()

	parser.SetExtractMethods(false)
	parser.SetExtractClasses(false)
	parser.SetExtractComments(false)

	assert.False(t, parser.extractMethods)
	assert.False(t, parser.extractClasses)
	assert.False(t, parser.extractComments)
}

func TestParser_SupportedFormats(t *testing.T) {
	parser := NewParser()
	formats := parser.SupportedFormats()
	assert.Len(t, formats, 1)
	assert.Equal(t, ".java", formats[0])
}
