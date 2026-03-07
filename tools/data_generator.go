package tools

import (
	"fmt"
	"os"
	"strings"
)

// generateTestData generates large amount of test data
func generateTestData(numChunks int, chunkSize int) []string {
	var chunks []string

	// Base content in both English and Chinese
	baseContent := "Go is a programming language designed for simplicity and efficiency. It is statically typed and compiled. Go has garbage collection and concurrency support. Go语言是一种开源编程语言，它能让构造简单、可靠且高效的软件变得容易。Go语言具有垃圾回收、类型安全和并发支持等特性。Go语言的设计理念是简洁、高效和可靠性。Go语言的语法简洁明了，易于学习和使用。Go语言的标准库非常丰富，提供了很多实用的功能。Go语言的编译速度非常快，生成的可执行文件体积小，运行效率高。"

	// Generate multiple chunks
	for i := 0; i < numChunks; i++ {
		// Create a chunk with repeated base content to reach the desired size
		var chunk strings.Builder
		for chunk.Len() < chunkSize {
			chunk.WriteString(fmt.Sprintf("Document %d, Part %d: %s\n", i+1, chunk.Len()/len(baseContent)+1, baseContent))
		}
		chunks = append(chunks, chunk.String()[:chunkSize])
	}

	return chunks
}

// generateBibleLikeData generates large test data similar to Bible size
func generateBibleLikeData() []string {
	var chunks []string

	// Bible has approximately 31,102 verses in 1,189 chapters
	// We'll generate similar structure with mixed content
	baseVerse := "In the beginning was the Word, and the Word was with God, and the Word was God. 太初有道，道与神同在，道就是神。"

	// Generate 10,000 verses (approximate Bible size)
	for book := 1; book <= 66; book++ { // 66 books in Bible
		for chapter := 1; chapter <= 50; chapter++ { // Up to 50 chapters per book
			for verse := 1; verse <= 30; verse++ { // Up to 30 verses per chapter
				if len(chunks) >= 10000 { // Stop at 10,000 verses
					break
				}
				chunk := fmt.Sprintf("Book %d, Chapter %d, Verse %d: %s\n", book, chapter, verse, baseVerse)
				chunks = append(chunks, chunk)
			}
			if len(chunks) >= 10000 {
				break
			}
		}
		if len(chunks) >= 10000 {
			break
		}
	}

	return chunks
}

func main() {
	// Generate 100 chunks of 1000 characters each
	// This will result in approximately 100,000 characters of test data
	numChunks := 100
	chunkSize := 1000

	fmt.Printf("Generating %d test chunks of %d characters each...\n", numChunks, chunkSize)
	chunks := generateTestData(numChunks, chunkSize)

	// Generate Bible-like data for large-scale testing
	fmt.Println("Generating Bible-like test data for large-scale testing...")
	bibleChunks := generateBibleLikeData()
	chunks = append(chunks, bibleChunks...)

	// Write chunks to files
	fmt.Printf("Creating %d test files...\n", len(chunks))
	os.MkdirAll("test_data", 0755)

	totalCharacters := 0
	totalLines := 0

	for i, chunk := range chunks {
		filename := fmt.Sprintf("test_data/test_chunk_%04d.txt", i+1)
		err := os.WriteFile(filename, []byte(chunk), 0644)
		if err != nil {
			fmt.Printf("Error writing file %s: %v\n", filename, err)
			os.Exit(1)
		}
		totalCharacters += len(chunk)
		totalLines += strings.Count(chunk, "\n")
	}

	fmt.Printf("Created %d test files in test_data directory\n", len(chunks))
	fmt.Printf("Total characters: %d\n", totalCharacters)
	fmt.Printf("Total lines: %d\n", totalLines)
	fmt.Println("Test data generation completed successfully!")
	fmt.Println("This includes Bible-like data for large-scale testing with over 10,000 verses!")
}
