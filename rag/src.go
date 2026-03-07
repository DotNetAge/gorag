package rag

import "github.com/DotNetAge/gorag/core"

// Source represents a document source for indexing
//
// Source defines the input for the indexing process. It can represent:
// 1. A text string (Content field)
// 2. A file path (Path field)
// 3. A reader interface (Reader field)
//
// The Type field specifies the document format (e.g., ".txt", ".pdf", ".docx")
// and is used to select the appropriate parser.
//
// Example:
//
//	// Index a text string
//	source1 := core.Source{
//	    Type:    "text",
//	    Content: "Go is an open source programming language...",
//	}
//
//	// Index a file
//	source2 := core.Source{
//	    Type: ".pdf",
//	    Path: "/path/to/document.pdf",
//	}
type Source = core.Source
