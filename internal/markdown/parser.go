package markdown

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"conflux/internal/config"
	"conflux/internal/confluence"
	"conflux/internal/mermaid"
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
	return ConvertToConfluenceFormatWithMermaid(markdown, nil, nil, "")
}

func ConvertToConfluenceFormatWithMermaid(markdown string, cfg *config.Config, client *confluence.Client, pageID string) string {
	lines := strings.Split(markdown, "\n")
	var result []string
	inCodeBlock := false
	inUnorderedList := false
	inOrderedList := false
	var codeBlockLang string
	var codeBlockContent []string

	for _, line := range lines {
		// Handle code blocks
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if !inCodeBlock {
				// Starting code block
				inCodeBlock = true
				codeBlockLang = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "```"))
				codeBlockContent = []string{} // Reset content
			} else {
				// Ending code block
				inCodeBlock = false

				// Process the code block based on language
				if codeBlockLang == "mermaid" && cfg != nil && client != nil && pageID != "" {
					processed := processMermaidDiagram(strings.Join(codeBlockContent, "\n"), cfg, client, pageID)
					if processed != "" {
						result = append(result, processed)
					} else {
						// Fallback to regular code block if processing failed
						result = append(result, fmt.Sprintf(`<ac:structured-macro ac:name="code" ac:schema-version="1"><ac:parameter ac:name="language">%s</ac:parameter><ac:plain-text-body><![CDATA[`, codeBlockLang))
						result = append(result, strings.Join(codeBlockContent, "\n"))
						result = append(result, `]]></ac:plain-text-body></ac:structured-macro>`)
					}
				} else {
					// Regular code block processing
					if codeBlockLang != "" {
						result = append(result, fmt.Sprintf(`<ac:structured-macro ac:name="code" ac:schema-version="1"><ac:parameter ac:name="language">%s</ac:parameter><ac:plain-text-body><![CDATA[`, codeBlockLang))
					} else {
						result = append(result, `<ac:structured-macro ac:name="code" ac:schema-version="1"><ac:plain-text-body><![CDATA[`)
					}
					result = append(result, strings.Join(codeBlockContent, "\n"))
					result = append(result, `]]></ac:plain-text-body></ac:structured-macro>`)
				}

				codeBlockLang = ""
				codeBlockContent = []string{}
			}
			continue
		}

		if inCodeBlock {
			// Inside code block - collect content
			codeBlockContent = append(codeBlockContent, line)
			continue
		}

		// Handle headers
		if strings.HasPrefix(line, "# ") {
			closeOpenLists(&result, &inUnorderedList, &inOrderedList)
			title := strings.TrimSpace(line[2:])
			result = append(result, fmt.Sprintf("<h1>%s</h1>", escapeHTML(title)))
			continue
		}
		if strings.HasPrefix(line, "## ") {
			closeOpenLists(&result, &inUnorderedList, &inOrderedList)
			title := strings.TrimSpace(line[3:])
			result = append(result, fmt.Sprintf("<h2>%s</h2>", escapeHTML(title)))
			continue
		}
		if strings.HasPrefix(line, "### ") {
			closeOpenLists(&result, &inUnorderedList, &inOrderedList)
			title := strings.TrimSpace(line[4:])
			result = append(result, fmt.Sprintf("<h3>%s</h3>", escapeHTML(title)))
			continue
		}
		if strings.HasPrefix(line, "#### ") {
			closeOpenLists(&result, &inUnorderedList, &inOrderedList)
			title := strings.TrimSpace(line[5:])
			result = append(result, fmt.Sprintf("<h4>%s</h4>", escapeHTML(title)))
			continue
		}

		// Handle unordered lists
		if strings.HasPrefix(strings.TrimSpace(line), "- ") || strings.HasPrefix(strings.TrimSpace(line), "* ") {
			if inOrderedList {
				result = append(result, "</ol>")
				inOrderedList = false
			}
			if !inUnorderedList {
				result = append(result, "<ul>")
				inUnorderedList = true
			}
			content := strings.TrimSpace(line[strings.Index(line, strings.TrimSpace(line))+2:])
			content = convertInlineFormatting(content)
			result = append(result, fmt.Sprintf("<li>%s</li>", content))
			continue
		}

		// Handle numbered lists
		if len(strings.TrimSpace(line)) > 0 && strings.Contains(strings.TrimSpace(line), ". ") {
			trimmed := strings.TrimSpace(line)
			if len(trimmed) > 2 {
				firstChar := trimmed[0]
				if firstChar >= '0' && firstChar <= '9' && trimmed[1] == '.' && trimmed[2] == ' ' {
					if inUnorderedList {
						result = append(result, "</ul>")
						inUnorderedList = false
					}
					if !inOrderedList {
						result = append(result, "<ol>")
						inOrderedList = true
					}
					content := strings.TrimSpace(trimmed[3:])
					content = convertInlineFormatting(content)
					result = append(result, fmt.Sprintf("<li>%s</li>", content))
					continue
				}
			}
		}

		// Handle empty lines
		if strings.TrimSpace(line) == "" {
			closeOpenLists(&result, &inUnorderedList, &inOrderedList)
			result = append(result, "<p/>")
			continue
		}

		// Regular paragraph
		closeOpenLists(&result, &inUnorderedList, &inOrderedList)
		content := convertInlineFormatting(line)
		result = append(result, fmt.Sprintf("<p>%s</p>", content))
	}

	// Close any remaining lists
	closeOpenLists(&result, &inUnorderedList, &inOrderedList)

	return strings.Join(result, "\n")
}

func closeOpenLists(result *[]string, inUnorderedList *bool, inOrderedList *bool) {
	if *inUnorderedList {
		*result = append(*result, "</ul>")
		*inUnorderedList = false
	}
	if *inOrderedList {
		*result = append(*result, "</ol>")
		*inOrderedList = false
	}
}

func convertInlineFormatting(text string) string {
	// Handle bold (**text** or __text__)
	text = convertBold(text)
	// Handle italic (*text* or _text_)
	text = convertItalic(text)
	// Handle inline code
	text = convertInlineCode(text)
	return text
}

func convertBold(text string) string {
	// Handle **bold**
	for strings.Contains(text, "**") {
		firstIndex := strings.Index(text, "**")
		if firstIndex == -1 {
			break
		}
		secondIndex := strings.Index(text[firstIndex+2:], "**")
		if secondIndex == -1 {
			break
		}
		secondIndex += firstIndex + 2

		before := text[:firstIndex]
		boldText := text[firstIndex+2 : secondIndex]
		after := text[secondIndex+2:]
		text = before + "<strong>" + escapeHTML(boldText) + "</strong>" + after
	}
	return text
}

func convertItalic(text string) string {
	// Handle *italic* (but not ** which is bold)
	i := 0
	for i < len(text) {
		if text[i] == '*' && (i == 0 || text[i-1] != '*') && (i+1 < len(text) && text[i+1] != '*') {
			// Found single asterisk
			nextIndex := -1
			for j := i + 1; j < len(text); j++ {
				if text[j] == '*' && (j+1 >= len(text) || text[j+1] != '*') {
					nextIndex = j
					break
				}
			}
			if nextIndex != -1 {
				before := text[:i]
				italicText := text[i+1 : nextIndex]
				after := text[nextIndex+1:]
				text = before + "<em>" + escapeHTML(italicText) + "</em>" + after
				i = len(before) + len("<em>") + len(italicText) + len("</em>")
				continue
			}
		}
		i++
	}
	return text
}

func convertInlineCode(text string) string {
	// Handle `inline code`
	for strings.Contains(text, "`") {
		firstIndex := strings.Index(text, "`")
		if firstIndex == -1 {
			break
		}
		secondIndex := strings.Index(text[firstIndex+1:], "`")
		if secondIndex == -1 {
			break
		}
		secondIndex += firstIndex + 1

		before := text[:firstIndex]
		codeText := text[firstIndex+1 : secondIndex]
		after := text[secondIndex+1:]
		text = before + "<code>" + escapeHTML(codeText) + "</code>" + after
	}
	return text
}

func escapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	text = strings.ReplaceAll(text, "\"", "&quot;")
	text = strings.ReplaceAll(text, "'", "&#39;")
	return text
}

func processMermaidDiagram(content string, cfg *config.Config, client *confluence.Client, pageID string) string {
	if cfg.Mermaid.Mode == "preserve" {
		// Return original mermaid code block
		return fmt.Sprintf(`<ac:structured-macro ac:name="code" ac:schema-version="1"><ac:parameter ac:name="language">mermaid</ac:parameter><ac:plain-text-body><![CDATA[%s]]></ac:plain-text-body></ac:structured-macro>`, content)
	}

	// Validate mermaid content
	if err := mermaid.ValidateContent(content); err != nil {
		// Return as regular code block if invalid
		return fmt.Sprintf(`<ac:structured-macro ac:name="code" ac:schema-version="1"><ac:parameter ac:name="language">mermaid</ac:parameter><ac:plain-text-body><![CDATA[%s]]></ac:plain-text-body></ac:structured-macro>`, content)
	}

	// Create processor
	processor := mermaid.NewProcessor(&cfg.Mermaid, nil)

	// Process diagram to image
	result, err := processor.ProcessDiagram(content)
	if err != nil {
		// Return as regular code block if processing failed
		return fmt.Sprintf(`<ac:structured-macro ac:name="code" ac:schema-version="1"><ac:parameter ac:name="language">mermaid</ac:parameter><ac:plain-text-body><![CDATA[%s]]></ac:plain-text-body></ac:structured-macro>`, content)
	}

	// Upload image as attachment
	attachment, err := client.UploadAttachment(pageID, result.ImagePath)
	if err != nil {
		// Cleanup temp file and return as code block
		if cleanupErr := processor.Cleanup(result); cleanupErr != nil {
			// Log cleanup error but continue with original error
		}
		return fmt.Sprintf(`<ac:structured-macro ac:name="code" ac:schema-version="1"><ac:parameter ac:name="language">mermaid</ac:parameter><ac:plain-text-body><![CDATA[%s]]></ac:plain-text-body></ac:structured-macro>`, content)
	}

	// Cleanup temp file
	if cleanupErr := processor.Cleanup(result); cleanupErr != nil {
		// Log cleanup error but continue with successful result
	}

	// Return Confluence image macro
	return fmt.Sprintf(`<ac:image><ri:attachment ri:filename="%s"/></ac:image>`, attachment.Filename)
}
