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

func FindMarkdownFiles(path string, exclude []string) ([]string, error) {
	var files []string

	// Check if the path is a single file
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to access path %s: %w", path, err)
	}

	if !info.IsDir() {
		// Handle single file
		if strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil, fmt.Errorf("file %s is not a markdown file (.md)", path)
		}

		// Check if file matches any exclude pattern
		for _, pattern := range exclude {
			if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
				return nil, fmt.Errorf("file %s matches exclude pattern %s", path, pattern)
			}
		}

		// Convert to absolute path
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for %s: %w", path, err)
		}

		return []string{absPath}, nil
	}

	// Handle directory (original logic)
	err = filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.ToLower(filepath.Ext(walkPath)) != ".md" {
			return nil
		}

		for _, pattern := range exclude {
			if matched, _ := filepath.Match(pattern, filepath.Base(walkPath)); matched {
				return nil
			}
		}

		files = append(files, walkPath)
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
				if codeBlockLang == "mermaid" && cfg != nil {
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
					codeContent := strings.TrimSpace(strings.Join(codeBlockContent, "\n"))
					if codeBlockLang != "" {
						result = append(result, fmt.Sprintf(`<ac:structured-macro ac:name="code" ac:schema-version="1"><ac:parameter ac:name="language">%s</ac:parameter><ac:plain-text-body><![CDATA[%s]]></ac:plain-text-body></ac:structured-macro>`, codeBlockLang, codeContent))
					} else {
						result = append(result, fmt.Sprintf(`<ac:structured-macro ac:name="code" ac:schema-version="1"><ac:plain-text-body><![CDATA[%s]]></ac:plain-text-body></ac:structured-macro>`, codeContent))
					}
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
	// First escape HTML in the entire text
	text = escapeHTML(text)
	// Handle bold (**text** or __text__)
	text = convertBoldFromEscaped(text)
	// Handle italic (*text* or _text_)
	text = convertItalicFromEscaped(text)
	// Handle inline code
	text = convertInlineCodeFromEscaped(text)
	return text
}

func convertBoldFromEscaped(text string) string {
	// Handle **bold** - back to simple approach (text already escaped)
	result := text

	for strings.Contains(result, "**") {
		firstIndex := strings.Index(result, "**")
		if firstIndex == -1 {
			break
		}

		// Find the next ** after the first one
		secondIndex := strings.Index(result[firstIndex+2:], "**")
		if secondIndex == -1 {
			break
		}
		secondIndex += firstIndex + 2

		before := result[:firstIndex]
		boldContent := result[firstIndex+2 : secondIndex]
		after := result[secondIndex+2:]

		result = before + "<strong>" + boldContent + "</strong>" + after
	}
	return result
}

func convertItalicFromEscaped(text string) string {
	// Handle *italic* (but not ** which is bold) - working with escaped text
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
				text = before + "<em>" + italicText + "</em>" + after
				i = len(before) + len("<em>") + len(italicText) + len("</em>")
				continue
			}
		}
		i++
	}
	return text
}

func convertInlineCodeFromEscaped(text string) string {
	// Handle `inline code` - working with escaped text
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
		text = before + "<code>" + codeText + "</code>" + after
	}
	return text
}

func convertBold(text string) string {
	result := text

	for {
		// Skip any ** that are inside existing <strong> tags to prevent recursive processing
		firstIndex := -1
		for i := 0; i < len(result)-1; i++ {
			if result[i:i+2] == "**" {
				// Check if this ** is inside a <strong> tag
				beforeThis := result[:i]
				strongOpen := strings.LastIndex(beforeThis, "<strong>")
				strongClose := strings.LastIndex(beforeThis, "</strong>")

				// If the last <strong> is more recent than the last </strong>, we're inside a tag
				if strongOpen != -1 && (strongClose == -1 || strongOpen > strongClose) {
					continue // Skip this **, it's inside a strong tag
				}

				firstIndex = i
				break
			}
		}

		if firstIndex == -1 {
			break
		}

		// Find all ** positions after the first one
		remaining := result[firstIndex+2:]
		if !strings.Contains(remaining, "**") {
			break
		}

		positions := []int{}
		searchPos := 0
		for {
			pos := strings.Index(remaining[searchPos:], "**")
			if pos == -1 {
				break
			}
			actualPos := firstIndex + 2 + searchPos + pos

			// Check if this position is inside a strong tag
			beforeThis := result[:actualPos]
			strongOpen := strings.LastIndex(beforeThis, "<strong>")
			strongClose := strings.LastIndex(beforeThis, "</strong>")

			if strongOpen != -1 && (strongClose == -1 || strongOpen > strongClose) {
				searchPos += pos + 2
				continue // Skip this **, it's inside a strong tag
			}

			positions = append(positions, actualPos)
			searchPos += pos + 2
		}

		if len(positions) == 0 {
			break
		}

		var secondIndex int

		if len(positions) == 1 {
			// Simple case - only one closing **
			secondIndex = positions[0]
		} else if len(positions) == 3 {
			// Check pattern for nested vs separate
			firstClose := positions[0]
			secondOpen := positions[1]
			lastClose := positions[2]

			betweenSections := result[firstClose+2 : secondOpen]

			// Separate sections if there's meaningful content with spaces
			if len(strings.TrimSpace(betweenSections)) > 2 && strings.Contains(betweenSections, " ") {
				secondIndex = firstClose // **first** and **second**
			} else {
				secondIndex = lastClose // **nested **bold** text**
			}
		} else {
			// Default to first closing
			secondIndex = positions[0]
		}

		before := result[:firstIndex]
		boldContent := result[firstIndex+2 : secondIndex]
		after := result[secondIndex+2:]

		result = before + "<strong>" + escapeHTML(boldContent) + "</strong>" + after
	}
	return result
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

	// Check if we have a pageID for attachment upload
	if pageID == "" || client == nil {
		// For new pages or when client is not available, fall back to code block
		if cleanupErr := processor.Cleanup(result); cleanupErr != nil {
			// Log cleanup error but continue
		}
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
