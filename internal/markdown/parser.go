package markdown

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"conflux/internal/config"
	"conflux/internal/confluence"
	"conflux/internal/images"
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
		var absPath string
		absPath, err = filepath.Abs(path)
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

	for i := 0; i < len(lines); i++ {
		line := lines[i]

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

		// Handle tables (GFM pipe tables): a header row followed by a delimiter row
		if isTableRow(line) && i+1 < len(lines) && isTableDelimiterRow(lines[i+1]) {
			closeOpenLists(&result, &inUnorderedList, &inOrderedList)
			tableLines := []string{line}
			end := i + 2
			for end < len(lines) && isTableRow(lines[end]) {
				tableLines = append(tableLines, lines[end])
				end++
			}
			result = append(result, convertTable(tableLines))
			i = end - 1
			continue
		}

		// Handle raw HTML / Confluence storage markup - pass it through untouched
		// so users can drop in <ac:...> macros, comments and plain HTML blocks.
		if isRawHTMLLine(line) {
			closeOpenLists(&result, &inUnorderedList, &inOrderedList)
			trimmed := strings.TrimSpace(line)

			// A bare <details> may be followed by <summary> on the next
			// non-empty line; that summary is the expand macro's title.
			if m := detailsOpenPattern.FindStringSubmatch(trimmed); m != nil && m[1] == "" {
				if next := nextNonEmptyLine(lines, i+1); next != -1 {
					if s := summaryOnlyPattern.FindStringSubmatch(strings.TrimSpace(lines[next])); s != nil {
						result = append(result, expandMacroOpen(s[1]))
						i = next
						continue
					}
				}
			}

			result = append(result, convertRawHTMLLine(trimmed))
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
	text = convertUnderscoreItalicFromEscaped(text)
	// Handle inline code
	text = convertInlineCodeFromEscaped(text)
	// Handle links last so formatting inside the link text is already converted
	text = convertLinksFromEscaped(text)
	return text
}

// convertLinksFromEscaped converts [text](url) links and ![alt](url) images.
// Bracket matching is done by hand rather than with a regex so that nested
// forms such as a linked badge - [![alt](image)](target) - survive intact.
//
// Images pointing at a remote URL become <ri:url> image macros. Images pointing
// at a local file are left as markdown, because ConvertToConfluenceFormatWithImages
// replaces those later by matching on the original markdown syntax.
func convertLinksFromEscaped(text string) string {
	var b strings.Builder

	for i := 0; i < len(text); {
		start := i
		isImage := text[i] == '!' && i+1 < len(text) && text[i+1] == '['
		open := i
		if isImage {
			open = i + 1
		}

		labelEnd := matchDelimiter(text, open, '[', ']')
		if text[open] != '[' || labelEnd == -1 || labelEnd+1 >= len(text) || text[labelEnd+1] != '(' {
			b.WriteByte(text[start])
			i = start + 1
			continue
		}

		urlEnd := matchDelimiter(text, labelEnd+1, '(', ')')
		if urlEnd == -1 {
			b.WriteByte(text[start])
			i = start + 1
			continue
		}

		label := text[open+1 : labelEnd]
		target := strings.TrimSpace(text[labelEnd+2 : urlEnd])

		switch {
		case isImage && isRemoteURL(target):
			fmt.Fprintf(&b, `<ac:image ac:alt="%s"><ri:url ri:value="%s"/></ac:image>`, stripHTMLTags(label), target)
		case isImage:
			// Local image - leave the markdown for the attachment processor.
			b.WriteString(text[start : urlEnd+1])
		default:
			fmt.Fprintf(&b, `<a href="%s">%s</a>`, target, convertLinksFromEscaped(label))
		}
		i = urlEnd + 1
	}

	return b.String()
}

// matchDelimiter returns the index of the delimiter closing the one at openIdx,
// accounting for nesting, or -1 when there is no match.
func matchDelimiter(s string, openIdx int, open, close byte) int {
	if openIdx >= len(s) || s[openIdx] != open {
		return -1
	}
	depth := 0
	for i := openIdx; i < len(s); i++ {
		switch s[i] {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func isRemoteURL(target string) bool {
	return strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://")
}

// isTableRow reports whether the line looks like a pipe table row.
func isTableRow(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "|")
}

var tableDelimiterCell = regexp.MustCompile(`^:?-+:?$`)

// isTableDelimiterRow reports whether the line is the |---|---| separator that
// follows a pipe table's header row.
func isTableDelimiterRow(line string) bool {
	if !isTableRow(line) {
		return false
	}
	cells := splitTableRow(line)
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		if !tableDelimiterCell.MatchString(cell) {
			return false
		}
	}
	return true
}

// splitTableRow splits a pipe table row into its cells, honouring \| escapes.
func splitTableRow(line string) []string {
	s := strings.TrimSpace(line)
	s = strings.TrimPrefix(s, "|")
	s = strings.TrimSuffix(s, "|")

	var cells []string
	var current strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == '|' {
			current.WriteByte('|')
			i++
			continue
		}
		if s[i] == '|' {
			cells = append(cells, strings.TrimSpace(current.String()))
			current.Reset()
			continue
		}
		current.WriteByte(s[i])
	}
	cells = append(cells, strings.TrimSpace(current.String()))
	return cells
}

// convertTable renders a pipe table (header row, delimiter row, body rows) as
// Confluence storage format. Header cells use <th>, matching the markup
// Confluence itself produces.
func convertTable(tableLines []string) string {
	var b strings.Builder
	b.WriteString("<table><tbody>")

	b.WriteString("<tr>")
	for _, cell := range splitTableRow(tableLines[0]) {
		fmt.Fprintf(&b, "<th>%s</th>", convertInlineFormatting(cell))
	}
	b.WriteString("</tr>")

	for _, row := range tableLines[1:] {
		b.WriteString("<tr>")
		for _, cell := range splitTableRow(row) {
			fmt.Fprintf(&b, "<td>%s</td>", convertInlineFormatting(cell))
		}
		b.WriteString("</tr>")
	}

	b.WriteString("</tbody></table>")
	return b.String()
}

// rawHTMLLinePattern matches a line that starts with an HTML/XML tag or comment.
var rawHTMLLinePattern = regexp.MustCompile(`^<(!--|/?[A-Za-z][A-Za-z0-9]*(:[A-Za-z][A-Za-z0-9-]*)?)`)

// isRawHTMLLine reports whether the line should be passed through as markup
// rather than escaped into a paragraph. The line must both open with a tag and
// close it, so prose that merely starts with a "<" is still escaped.
func isRawHTMLLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !rawHTMLLinePattern.MatchString(trimmed) {
		return false
	}
	if strings.HasPrefix(trimmed, "<!--") {
		return strings.Contains(trimmed, "-->")
	}
	return strings.Contains(trimmed, ">")
}

func nextNonEmptyLine(lines []string, from int) int {
	for i := from; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			return i
		}
	}
	return -1
}

var (
	detailsOpenPattern = regexp.MustCompile(`(?is)^<details[^>]*>\s*(?:<summary[^>]*>(.*?)</summary>)?\s*$`)
	summaryOnlyPattern = regexp.MustCompile(`(?is)^<summary[^>]*>(.*?)</summary>\s*$`)
	htmlTagPattern     = regexp.MustCompile(`<[^>]*>`)
)

// convertRawHTMLLine passes raw markup through, translating the <details> /
// <summary> pair into Confluence's expand macro. Confluence storage format has
// no <details> element, so a verbatim copy would be dropped by the server.
func convertRawHTMLLine(line string) string {
	if m := detailsOpenPattern.FindStringSubmatch(line); m != nil {
		return expandMacroOpen(m[1])
	}
	if m := summaryOnlyPattern.FindStringSubmatch(line); m != nil {
		// A <summary> on its own line, immediately after a bare <details>.
		return fmt.Sprintf("<p><strong>%s</strong></p>", escapeHTML(stripHTMLTags(m[1])))
	}
	if strings.EqualFold(strings.TrimSpace(line), "</details>") {
		return "</ac:rich-text-body></ac:structured-macro>"
	}
	return line
}

func expandMacroOpen(summary string) string {
	title := strings.TrimSpace(stripHTMLTags(summary))
	if title == "" {
		title = "Details"
	}
	return fmt.Sprintf(`<ac:structured-macro ac:name="expand" ac:schema-version="1"><ac:parameter ac:name="title">%s</ac:parameter><ac:rich-text-body>`, escapeHTML(title))
}

func stripHTMLTags(text string) string {
	return htmlTagPattern.ReplaceAllString(text, "")
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

// convertUnderscoreItalicFromEscaped handles _italic_. Underscores only open or
// close emphasis at a word boundary, so identifiers such as perception_tools or
// avidbots_laser_manager are left alone.
func convertUnderscoreItalicFromEscaped(text string) string {
	for i := 0; i < len(text); i++ {
		if !isUnderscoreOpener(text, i) {
			continue
		}
		closer := -1
		for j := i + 1; j < len(text); j++ {
			if isUnderscoreCloser(text, j) {
				closer = j
				break
			}
		}
		if closer == -1 {
			continue
		}
		italic := text[i+1 : closer]
		text = text[:i] + "<em>" + italic + "</em>" + text[closer+1:]
		i += len("<em>") + len(italic) + len("</em>")
	}
	return text
}

func isUnderscoreOpener(text string, i int) bool {
	if text[i] != '_' {
		return false
	}
	// Not adjacent to another underscore (avoids mangling __bold__).
	if (i > 0 && text[i-1] == '_') || (i+1 < len(text) && text[i+1] == '_') {
		return false
	}
	// Preceded by a boundary, followed by content.
	if i > 0 && isWordByte(text[i-1]) {
		return false
	}
	return i+1 < len(text) && text[i+1] != ' '
}

func isUnderscoreCloser(text string, j int) bool {
	if text[j] != '_' {
		return false
	}
	if (j > 0 && text[j-1] == '_') || (j+1 < len(text) && text[j+1] == '_') {
		return false
	}
	// Preceded by content, followed by a boundary.
	if j == 0 || text[j-1] == ' ' {
		return false
	}
	return j+1 >= len(text) || !isWordByte(text[j+1])
}

func isWordByte(c byte) bool {
	return c == '_' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c >= 0x80 // treat multi-byte UTF-8 as word content
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
		_ = processor.Cleanup(result) // Best effort cleanup, ignore errors
		return fmt.Sprintf(`<ac:structured-macro ac:name="code" ac:schema-version="1"><ac:parameter ac:name="language">mermaid</ac:parameter><ac:plain-text-body><![CDATA[%s]]></ac:plain-text-body></ac:structured-macro>`, content)
	}

	// Upload image as attachment
	attachment, err := client.UploadAttachment(pageID, result.ImagePath)
	if err != nil {
		// Cleanup temp file and return as code block
		_ = processor.Cleanup(result) // Best effort cleanup, ignore errors
		return fmt.Sprintf(`<ac:structured-macro ac:name="code" ac:schema-version="1"><ac:parameter ac:name="language">mermaid</ac:parameter><ac:plain-text-body><![CDATA[%s]]></ac:plain-text-body></ac:structured-macro>`, content)
	}

	// Determine the filename to use for the attachment reference
	filename := attachment.Title
	if filename == "" {
		// Fallback to the generated filename if Confluence API doesn't return it
		filename = result.Filename
	}
	if filename == "" {
		// Final fallback to Title field if available
		filename = attachment.Title
	}

	// Cleanup temp file
	_ = processor.Cleanup(result) // Best effort cleanup, ignore errors

	// Return Confluence image macro with full page width
	return fmt.Sprintf(`<ac:image ac:width="100%%"><ri:attachment ri:filename="%s"/></ac:image>`, filename)
}

// ConvertToConfluenceFormatWithImages processes markdown with image attachment support
func ConvertToConfluenceFormatWithImages(markdown string, cfg *config.Config, client *confluence.Client, pageID string, markdownFilePath string) (string, error) {
	// Process mermaid diagrams first
	content := ConvertToConfluenceFormatWithMermaid(markdown, cfg, client, pageID)

	// Now process images if we have the necessary components
	if cfg == nil || client == nil || pageID == "" || markdownFilePath == "" {
		// No image processing possible - return content as-is
		return content, nil
	}

	// Create image processor
	imageProcessor := images.NewProcessor(&cfg.Images, nil)

	// Get directory of the markdown file to resolve relative image paths
	markdownDir := filepath.Dir(markdownFilePath)

	// Find image references in the original markdown content
	imageRefs, err := imageProcessor.FindImageReferences(markdown, markdownDir)
	if err != nil {
		// Log error but continue without image processing
		return content, nil
	}

	// Validate image references
	validRefs, err := imageProcessor.ValidateImageReferences(imageRefs)
	if err != nil {
		// Log error but continue without image processing
		return content, nil
	}

	// If no valid image references, return content as-is
	if len(validRefs) == 0 {
		return content, nil
	}

	// Process each valid image reference
	for _, ref := range validRefs {
		// Upload image as attachment
		attachment, err := client.UploadAttachment(pageID, ref.AbsolutePath)
		if err != nil {
			// Skip this image on upload error, continue with others
			continue
		}

		// Determine the filename for the attachment reference
		filename := attachment.Title
		if filename == "" {
			filename = attachment.Title
		}
		if filename == "" {
			filename = images.GetImageFilename(ref.AbsolutePath)
		}

		// Replace the markdown image syntax with Confluence image macro
		confluenceImageMacro := fmt.Sprintf(`<ac:image><ri:attachment ri:filename="%s"/></ac:image>`, filename)

		// Replace in the content (note: we're working with already processed content that may have HTML)
		// We need to be careful to replace the original markdown syntax, not HTML
		content = strings.ReplaceAll(content, ref.MarkdownSyntax, confluenceImageMacro)
	}

	return content, nil
}
