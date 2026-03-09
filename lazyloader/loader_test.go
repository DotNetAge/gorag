package lazyloader

import (
	"context"
	"os"
	"testing"
)

func TestLazyDocument(t *testing.T) {
	// Create a temporary file
	tempFile, err := os.CreateTemp("", "test-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write test content
	testContent := "Hello, World!"
	_, err = tempFile.WriteString(testContent)
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	// Create lazy document
	doc := NewLazyDocument(tempFile.Name())

	// Test Load
	content, err := doc.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if string(content) != testContent {
		t.Errorf("Expected content '%s', got '%s'", testContent, string(content))
	}

	// Test IsLoaded
	if !doc.IsLoaded() {
		t.Error("Expected document to be loaded")
	}

	// Test Size
	size, err := doc.Size()
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	if size == 0 {
		t.Error("Expected size to be greater than 0")
	}

	// Test Unload
	doc.Unload()
	if doc.IsLoaded() {
		t.Error("Expected document to be unloaded")
	}

	// Test LoadWithContext
	ctx := context.Background()
	content, err = doc.LoadWithContext(ctx)
	if err != nil {
		t.Fatalf("LoadWithContext failed: %v", err)
	}
	if string(content) != testContent {
		t.Errorf("Expected content '%s', got '%s'", testContent, string(content))
	}
}

func TestLazyDocumentManager(t *testing.T) {
	// Create a temporary file
	tempFile, err := os.CreateTemp("", "test-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write test content
	testContent := "Hello, World!"
	_, err = tempFile.WriteString(testContent)
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	// Create manager with small memory limit
	manager := NewLazyDocumentManager(100) // 100 bytes

	// Add document
	manager.AddDocument(tempFile.Name())

	// Test GetDocument
	retrievedDoc, found := manager.GetDocument(tempFile.Name())
	if !found {
		t.Error("Expected to find document")
	}
	if retrievedDoc == nil {
		t.Error("Expected to retrieve document")
	}

	// Test GetMemoryUsage
	usage := manager.GetMemoryUsage()
	if usage < 0 {
		t.Error("Expected memory usage to be non-negative")
	}

	// Test UnloadAll
	manager.UnloadAll()
	usage = manager.GetMemoryUsage()
	if usage != 0 {
		t.Errorf("Expected memory usage to be 0 after UnloadAll, got %d", usage)
	}
}
