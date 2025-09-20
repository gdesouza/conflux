package markdown

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Document struct {
	Title    string
	Content  string
	FilePath string
}

func ParseFile(filePath string) (*Document, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	doc := &Document{
		FilePath: filePath,
		Content:  strings.Join(lines, "\n"),
	}

	doc.Title = extractTitle(lines, filePath)

	return doc, nil
}

func extractTitle(lines []string, filePath string) string {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:])
		}
	}

	base := filepath.Base(filePath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func FindMarkdownFiles(dir string, exclude []string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}

		for _, pattern := range exclude {
			if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
				return nil
			}
		}

		files = append(files, path)
		return nil
	})

	return files, err
}

func ConvertToConfluenceFormat(markdown string) string {
	confluence := markdown

	confluence = strings.ReplaceAll(confluence, "```", "{code}")
	confluence = strings.ReplaceAll(confluence, "`", "{{")
	confluence = strings.ReplaceAll(confluence, "{{", "}}")

	return confluence
}
