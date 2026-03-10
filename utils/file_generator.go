package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// GenerateLargeTextFile generates a large text file with the specified size in MB
//
// This function generates a large text file by writing repetitive content until
// it reaches the specified size.
//
// Parameters:
// - filePath: Path to the output file
// - sizeMB: Size of the file in megabytes
//
// Returns:
// - error: Error if file generation fails
//
// Example:
//
//     err := utils.GenerateLargeTextFile("test.txt", 100) // Generate 100MB file
//     if err != nil {
//         log.Fatal(err)
//     }
func GenerateLargeTextFile(filePath string, sizeMB int) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	defer writer.Flush()

	// Calculate the number of bytes to write
	targetSize := int64(sizeMB) * 1024 * 1024
	written := int64(0)

	// Generate repetitive content
	content := "This is a test line for generating large text files. "
	contentRepeat := strings.Repeat(content, 100) + "\n"
	contentSize := int64(len(contentRepeat))

	// Write content until we reach the target size
	for written < targetSize {
		writeSize := contentSize
		if written+writeSize > targetSize {
			writeSize = targetSize - written
		}

		_, err := writer.WriteString(contentRepeat[:writeSize])
		if err != nil {
			return err
		}

		written += writeSize

		// Print progress every 10%
		if written%int64(targetSize/10) == 0 {
			fmt.Printf("Generated %.1f%% of %dMB file\n", float64(written)/float64(targetSize)*100, sizeMB)
		}
	}

	fmt.Printf("Successfully generated %dMB file: %s\n", sizeMB, filePath)
	return nil
}

// GenerateLargeJSONFile generates a large JSON file with the specified size in MB
//
// This function generates a large JSON file by writing repetitive JSON objects until
// it reaches the specified size.
//
// Parameters:
// - filePath: Path to the output file
// - sizeMB: Size of the file in megabytes
//
// Returns:
// - error: Error if file generation fails
//
// Example:
//
//     err := utils.GenerateLargeJSONFile("test.json", 50) // Generate 50MB JSON file
//     if err != nil {
//         log.Fatal(err)
//     }
func GenerateLargeJSONFile(filePath string, sizeMB int) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	defer writer.Flush()

	// Calculate the number of bytes to write
	targetSize := int64(sizeMB) * 1024 * 1024
	written := int64(0)

	// Write opening bracket
	writer.WriteString(`{"data": [`)
	written += 8

	// Generate repetitive JSON objects
	object := `{"id": %d, "name": "Test Item %d", "description": "This is a test description for generating large JSON files."}`

	// Write objects until we reach the target size
	for i := 1; written < targetSize-2; i++ {
		jsonObj := fmt.Sprintf(object, i, i)
		if i > 1 {
			jsonObj = "," + jsonObj
		}
		jsonObj += "\n"

		objSize := int64(len(jsonObj))
		if written+objSize > targetSize-2 {
			break
		}

		_, err := writer.WriteString(jsonObj)
		if err != nil {
			return err
		}

		written += objSize

		// Print progress every 10%
		if written%int64(targetSize/10) == 0 {
			fmt.Printf("Generated %.1f%% of %dMB JSON file\n", float64(written)/float64(targetSize)*100, sizeMB)
		}
	}

	// Write closing brackets
	writer.WriteString(`]}`)
	written += 2

	fmt.Printf("Successfully generated %dMB JSON file: %s\n", sizeMB, filePath)
	return nil
}

// GenerateLargeHTMLFile generates a large HTML file with the specified size in MB
//
// This function generates a large HTML file by writing repetitive HTML sections until
// it reaches the specified size.
//
// Parameters:
// - filePath: Path to the output file
// - sizeMB: Size of the file in megabytes
//
// Returns:
// - error: Error if file generation fails
//
// Example:
//
//     err := utils.GenerateLargeHTMLFile("test.html", 25) // Generate 25MB HTML file
//     if err != nil {
//         log.Fatal(err)
//     }
func GenerateLargeHTMLFile(filePath string, sizeMB int) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	defer writer.Flush()

	// Calculate the number of bytes to write
	targetSize := int64(sizeMB) * 1024 * 1024
	written := int64(0)

	// Write HTML header
	header := `<!DOCTYPE html>
<html>
<head>
	<title>Large HTML File</title>
</head>
<body>
	<h1>Large HTML File Test</h1>
`
	writer.WriteString(header)
	written += int64(len(header))

	// Generate repetitive content
	content := `<div class="section">
	<h2>Section %d</h2>
	<p>This is a test paragraph for generating large HTML files. </p>
</div>
`

	// Write content until we reach the target size
	for i := 1; written < targetSize-14; i++ {
		htmlSection := fmt.Sprintf(content, i)
		sectionSize := int64(len(htmlSection))

		if written+sectionSize > targetSize-14 {
			break
		}

		_, err := writer.WriteString(htmlSection)
		if err != nil {
			return err
		}

		written += sectionSize

		// Print progress every 10%
		if written%int64(targetSize/10) == 0 {
			fmt.Printf("Generated %.1f%% of %dMB HTML file\n", float64(written)/float64(targetSize)*100, sizeMB)
		}
	}

	// Write HTML footer
	footer := `</body>
</html>`
	writer.WriteString(footer)
	written += int64(len(footer))

	fmt.Printf("Successfully generated %dMB HTML file: %s\n", sizeMB, filePath)
	return nil
}
