package markdown

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"conflux/internal/config"
)

func TestParseFile(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "conflux-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test markdown file
	testFilePath := filepath.Join(tempDir, "test.md")
	content := `# Test Document

This is a test markdown document with **bold** and *italic* text.

## Section 2

- Item 1
- Item 2

## Code Example

` + "```" + `go
func main() {
    fmt.Println("Hello, world!")
}
` + "```" + `
`

	err = os.WriteFile(testFilePath, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Parse the file
	doc, err := ParseFile(testFilePath)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if doc == nil {
		t.Fatal("Expected document, got nil")
	}

	if doc.Title != "Test Document" {
		t.Errorf("Expected title 'Test Document', got '%s'", doc.Title)
	}

	if doc.FilePath != testFilePath {
		t.Errorf("Expected file path '%s', got '%s'", testFilePath, doc.FilePath)
	}

	if !strings.Contains(doc.Content, "This is a test markdown document") {
		t.Error("Expected content to contain test text")
	}

	if !strings.Contains(doc.Content, "```go") {
		t.Error("Expected content to contain code block")
	}
}

func TestParseFileNoTitle(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "conflux-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test markdown file without title
	testFilePath := filepath.Join(tempDir, "no-title.md")
	content := `This is a document without a title.

Just some regular content.`

	err = os.WriteFile(testFilePath, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Parse the file
	doc, err := ParseFile(testFilePath)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if doc.Title != "no-title" {
		t.Errorf("Expected title 'no-title' (from filename), got '%s'", doc.Title)
	}
}

func TestParseFileNotFound(t *testing.T) {
	doc, err := ParseFile("/nonexistent/file.md")
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}

	if doc != nil {
		t.Error("Expected nil document on error")
	}

	if !strings.Contains(err.Error(), "failed to open file") {
		t.Errorf("Expected 'failed to open file' error, got: %v", err)
	}
}

func TestExtractTitle(t *testing.T) {
	testCases := []struct {
		name     string
		lines    []string
		filePath string
		expected string
	}{
		{
			name:     "H1 title",
			lines:    []string{"# My Title", "Content here"},
			filePath: "/path/to/test.md",
			expected: "My Title",
		},
		{
			name:     "H1 title with extra spaces",
			lines:    []string{"", "#    My Spaced Title   ", "Content"},
			filePath: "/path/to/test.md",
			expected: "My Spaced Title",
		},
		{
			name:     "No title - use filename",
			lines:    []string{"Just content", "## This is H2, not title"},
			filePath: "/path/to/my-document.md",
			expected: "my-document",
		},
		{
			name:     "Empty file - use filename",
			lines:    []string{},
			filePath: "/path/to/empty-file.md",
			expected: "empty-file",
		},
		{
			name:     "H1 later in document",
			lines:    []string{"Some intro", "", "# Actual Title", "Content"},
			filePath: "/path/to/test.md",
			expected: "Actual Title",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractTitle(tc.lines, tc.filePath)
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestFindMarkdownFiles(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "conflux-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	files := []struct {
		path    string
		content string
	}{
		{"file1.md", "# File 1"},
		{"file2.md", "# File 2"},
		{"README.md", "# README"},
		{"subdir/file3.md", "# File 3"},
		{"subdir/file4.txt", "Not markdown"},
		{"ignore-me.md", "# Should be ignored"},
	}

	for _, f := range files {
		fullPath := filepath.Join(tempDir, f.path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		err = os.WriteFile(fullPath, []byte(f.content), 0600)
		if err != nil {
			t.Fatalf("Failed to write file %s: %v", f.path, err)
		}
	}

	t.Run("Find all markdown files in directory", func(t *testing.T) {
		files, err := FindMarkdownFiles(tempDir, []string{})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(files) != 5 { // file1.md, file2.md, README.md, subdir/file3.md, ignore-me.md = 5 markdown files
			t.Errorf("Expected 5 markdown files, got %d", len(files))
		}

		// Check that we found the expected files
		foundFiles := make(map[string]bool)
		for _, f := range files {
			foundFiles[filepath.Base(f)] = true
		}

		expectedFiles := []string{"file1.md", "file2.md", "README.md", "file3.md", "ignore-me.md"}
		for _, expected := range expectedFiles {
			if !foundFiles[expected] {
				t.Errorf("Expected to find file %s", expected)
			}
		}
	})

	t.Run("Find with exclude patterns", func(t *testing.T) {
		files, err := FindMarkdownFiles(tempDir, []string{"README.md", "ignore-*.md"})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Should exclude README.md and ignore-me.md
		if len(files) != 3 { // file1.md, file2.md, file3.md
			t.Errorf("Expected 3 files after exclusion, got %d", len(files))
		}

		// Check that excluded files are not present
		for _, f := range files {
			name := filepath.Base(f)
			if name == "README.md" || name == "ignore-me.md" {
				t.Errorf("Found excluded file: %s", name)
			}
		}
	})

	t.Run("Single file", func(t *testing.T) {
		singleFile := filepath.Join(tempDir, "file1.md")
		files, err := FindMarkdownFiles(singleFile, []string{})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(files) != 1 {
			t.Errorf("Expected 1 file, got %d", len(files))
		}

		if !strings.HasSuffix(files[0], "file1.md") {
			t.Errorf("Expected file1.md, got %s", files[0])
		}
	})

	t.Run("Single non-markdown file", func(t *testing.T) {
		nonMdFile := filepath.Join(tempDir, "subdir", "file4.txt")
		files, err := FindMarkdownFiles(nonMdFile, []string{})
		if err == nil {
			t.Fatal("Expected error for non-markdown file")
		}

		if files != nil {
			t.Error("Expected nil files on error")
		}

		if !strings.Contains(err.Error(), "is not a markdown file") {
			t.Errorf("Expected 'is not a markdown file' error, got: %v", err)
		}
	})

	t.Run("Single file with exclusion pattern", func(t *testing.T) {
		excludedFile := filepath.Join(tempDir, "ignore-me.md")
		files, err := FindMarkdownFiles(excludedFile, []string{"ignore-*.md"})
		if err == nil {
			t.Fatal("Expected error for excluded file")
		}

		if files != nil {
			t.Error("Expected nil files on error")
		}

		if !strings.Contains(err.Error(), "matches exclude pattern") {
			t.Errorf("Expected 'matches exclude pattern' error, got: %v", err)
		}
	})
}

func TestFindMarkdownFilesNonexistent(t *testing.T) {
	files, err := FindMarkdownFiles("/nonexistent/path", []string{})
	if err == nil {
		t.Fatal("Expected error for nonexistent path")
	}

	if files != nil {
		t.Error("Expected nil files on error")
	}

	if !strings.Contains(err.Error(), "failed to access path") {
		t.Errorf("Expected 'failed to access path' error, got: %v", err)
	}
}

func TestConvertToConfluenceFormat(t *testing.T) {
	testCases := []struct {
		name     string
		markdown string
		expected string
	}{
		{
			name:     "Headers",
			markdown: "# H1\n## H2\n### H3\n#### H4",
			expected: "<h1>H1</h1>\n<h2>H2</h2>\n<h3>H3</h3>\n<h4>H4</h4>",
		},
		{
			name:     "Bold and italic",
			markdown: "This is **bold** and *italic* text.",
			expected: "<p>This is <strong>bold</strong> and <em>italic</em> text.</p>",
		},
		{
			name:     "Inline code",
			markdown: "Use `fmt.Println()` function.",
			expected: "<p>Use <code>fmt.Println()</code> function.</p>",
		},
		{
			name:     "Unordered list",
			markdown: "- Item 1\n- Item 2\n- Item 3",
			expected: "<ul>\n<li>Item 1</li>\n<li>Item 2</li>\n<li>Item 3</li>\n</ul>",
		},
		{
			name:     "Ordered list",
			markdown: "1. First\n2. Second\n3. Third",
			expected: "<ol>\n<li>First</li>\n<li>Second</li>\n<li>Third</li>\n</ol>",
		},
		{
			name:     "Mixed lists",
			markdown: "- Unordered item\n1. Ordered item\n2. Another ordered",
			expected: "<ul>\n<li>Unordered item</li>\n</ul>\n<ol>\n<li>Ordered item</li>\n<li>Another ordered</li>\n</ol>",
		},
		{
			name:     "Code block",
			markdown: "```go\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n```",
			expected: `<ac:structured-macro ac:name="code" ac:schema-version="1"><ac:parameter ac:name="language">go</ac:parameter><ac:plain-text-body><![CDATA[func main() {
    fmt.Println("Hello")
}]]></ac:plain-text-body></ac:structured-macro>`,
		},
		{
			name:     "Code block without language",
			markdown: "```\nsome code\n```",
			expected: `<ac:structured-macro ac:name="code" ac:schema-version="1"><ac:plain-text-body><![CDATA[some code]]></ac:plain-text-body></ac:structured-macro>`,
		},
		{
			name:     "Empty lines",
			markdown: "Paragraph 1\n\nParagraph 2",
			expected: "<p>Paragraph 1</p>\n<p/>\n<p>Paragraph 2</p>",
		},
		{
			name:     "HTML escaping",
			markdown: "Text with <html> & \"quotes\" and 'apostrophes'",
			expected: "<p>Text with &lt;html&gt; &amp; &quot;quotes&quot; and &#39;apostrophes&#39;</p>",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ConvertToConfluenceFormat(tc.markdown)
			if result != tc.expected {
				t.Errorf("Expected:\n%s\n\nGot:\n%s", tc.expected, result)
			}
		})
	}
}

func TestConvertInlineFormatting(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Bold with asterisks",
			input:    "This is **bold** text",
			expected: "This is <strong>bold</strong> text",
		},
		{
			name:     "Multiple bold sections",
			input:    "**First** and **second** bold",
			expected: "<strong>First</strong> and <strong>second</strong> bold",
		},
		{
			name:     "Italic with single asterisk",
			input:    "This is *italic* text",
			expected: "This is <em>italic</em> text",
		},
		{
			name:     "Multiple italic sections",
			input:    "*First* and *second* italic",
			expected: "<em>First</em> and <em>second</em> italic",
		},
		{
			name:     "Mixed bold and italic",
			input:    "**Bold** and *italic* together",
			expected: "<strong>Bold</strong> and <em>italic</em> together",
		},
		{
			name:     "Inline code",
			input:    "Use `code` here",
			expected: "Use <code>code</code> here",
		},
		{
			name:     "Multiple inline code",
			input:    "`first` and `second` code",
			expected: "<code>first</code> and <code>second</code> code",
		},
		{
			name:     "All formatting types",
			input:    "**Bold** *italic* and `code`",
			expected: "<strong>Bold</strong> <em>italic</em> and <code>code</code>",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := convertInlineFormatting(tc.input)
			if result != tc.expected {
				t.Errorf("Expected: %s\nGot: %s", tc.expected, result)
			}
		})
	}
}

func TestEscapeHTML(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"<html>", "&lt;html&gt;"},
		{"&amp;", "&amp;amp;"},
		{`"quotes"`, "&quot;quotes&quot;"},
		{"'apostrophe'", "&#39;apostrophe&#39;"},
		{"<tag attr=\"value\">", "&lt;tag attr=&quot;value&quot;&gt;"},
		{"Normal text", "Normal text"},
		{"", ""},
	}

	for _, tc := range testCases {
		result := escapeHTML(tc.input)
		if result != tc.expected {
			t.Errorf("escapeHTML(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestConvertBold(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"**bold**", "<strong>bold</strong>"},
		{"**first** and **second**", "<strong>first</strong> and <strong>second</strong>"},
		{"no bold here", "no bold here"},
		{"**incomplete bold", "**incomplete bold"},
		{"**nested **bold** text**", "<strong>nested **bold** text</strong>"},
	}

	for _, tc := range testCases {
		result := convertBold(tc.input)
		if result != tc.expected {
			t.Errorf("convertBold(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestConvertItalic(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"*italic*", "<em>italic</em>"},
		{"*first* and *second*", "<em>first</em> and <em>second</em>"},
		{"no italic here", "no italic here"},
		{"*incomplete italic", "*incomplete italic"},
		{"**bold** not *italic*", "**bold** not <em>italic</em>"},
	}

	for _, tc := range testCases {
		result := convertItalic(tc.input)
		if result != tc.expected {
			t.Errorf("convertItalic(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestConvertInlineCode(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"`code`", "<code>code</code>"},
		{"`first` and `second`", "<code>first</code> and <code>second</code>"},
		{"no code here", "no code here"},
		{"`incomplete code", "`incomplete code"},
		{"`code with <html>`", "<code>code with &lt;html&gt;</code>"},
	}

	for _, tc := range testCases {
		result := convertInlineCode(tc.input)
		if result != tc.expected {
			t.Errorf("convertInlineCode(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestProcessMermaidDiagram(t *testing.T) {
	// Create a basic config for testing
	cfg := &config.Config{
		Mermaid: config.MermaidConfig{
			Mode: "preserve",
		},
	}

	t.Run("Preserve mode", func(t *testing.T) {
		content := "graph TD\n    A --> B"
		result := processMermaidDiagram(content, cfg, nil, "")

		expected := `<ac:structured-macro ac:name="code" ac:schema-version="1"><ac:parameter ac:name="language">mermaid</ac:parameter><ac:plain-text-body><![CDATA[graph TD
    A --> B]]></ac:plain-text-body></ac:structured-macro>`

		if result != expected {
			t.Errorf("Expected preserve mode to return code block, got: %s", result)
		}
	})

	// Note: Testing image conversion mode would require mocking the mermaid processor
	// and setting up more complex test infrastructure. For now, we test the preserve mode
	// which is the most straightforward path.
}

func TestConvertToConfluenceFormatWithMermaid(t *testing.T) {
	cfg := &config.Config{
		Mermaid: config.MermaidConfig{
			Mode: "preserve",
		},
	}

	markdown := "# Test\n\n```mermaid\ngraph TD\n    A --> B\n```\n\nRegular text."

	result := ConvertToConfluenceFormatWithMermaid(markdown, cfg, nil, "")

	if !strings.Contains(result, "<h1>Test</h1>") {
		t.Error("Expected H1 header in result")
	}

	if !strings.Contains(result, `ac:name="code"`) {
		t.Error("Expected mermaid to be converted to code block")
	}

	if !strings.Contains(result, `ac:parameter ac:name="language">mermaid`) {
		t.Error("Expected mermaid language parameter")
	}

	if !strings.Contains(result, "<p>Regular text.</p>") {
		t.Error("Expected regular paragraph")
	}
}

// Test edge cases and error handling

func TestFindMarkdownFilesEmptyDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "conflux-test-empty-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	files, err := FindMarkdownFiles(tempDir, []string{})
	if err != nil {
		t.Fatalf("Expected no error for empty directory, got %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Expected 0 files in empty directory, got %d", len(files))
	}
}

func TestConvertToConfluenceFormatComplexLists(t *testing.T) {
	// Test mixed content with lists and other elements
	markdown := `# Document

Some intro text.

- First item
- Second item

Regular paragraph.

1. Numbered item
2. Another numbered

## Section

More content.`

	result := ConvertToConfluenceFormat(markdown)

	// Check that lists are properly closed before other elements
	if !strings.Contains(result, "</ul>") {
		t.Error("Expected unordered list to be closed")
	}

	if !strings.Contains(result, "</ol>") {
		t.Error("Expected ordered list to be closed")
	}

	if !strings.Contains(result, "<h1>Document</h1>") {
		t.Error("Expected h1 header")
	}

	if !strings.Contains(result, "<h2>Section</h2>") {
		t.Error("Expected h2 header")
	}
}
