package commands

import (
	"os"
	"path/filepath"
	"testing"

	"conflux/internal/confluence"
	"conflux/internal/content"
	"conflux/pkg/logger"
)

const pullTestConfigYAML = `confluence:
  base_url: http://example
  username: u
  api_token: t
  space_key: DOCS
local:
  markdown_dir: ./docs
projects:
  - name: myproject
    space_key: PROJ
    local:
      markdown_dir: ./proj-docs
`

func writePullTempConfig(t *testing.T) string {
	f, err := os.CreateTemp(t.TempDir(), "cfg-*.yaml")
	if err != nil {
		t.Fatalf("temp config: %v", err)
	}
	if _, err := f.WriteString(pullTestConfigYAML); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close cfg: %v", err)
	}
	return f.Name()
}

func resetPullFlags() {
	pullSpace = ""
	pullIDOrTitle = ""
	pullFormat = ""
	pullProject = ""
	pullOutput = ""
	pullForce = false
}

// --- runPull tests ---

func TestRunPull_MissingPageFlag(t *testing.T) {
	resetPullFlags()
	configFile = writePullTempConfig(t)
	verbose = false
	pullSpace = "DOCS"
	pullIDOrTitle = ""

	err := runPull(pullCmd, nil)
	if err == nil {
		t.Fatal("expected error for missing page flag")
	}
	if err.Error() != "page flag is required for pull command" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPull_UnsupportedFormat(t *testing.T) {
	resetPullFlags()
	configFile = writePullTempConfig(t)
	verbose = false
	pullSpace = "DOCS"
	pullIDOrTitle = "123"
	pullFormat = "invalid"

	err := runPull(pullCmd, nil)
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
	if err.Error() != "unsupported format: invalid" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPull_ValidFormats(t *testing.T) {
	formats := []string{"", "storage", "html", "markdown"}
	for _, format := range formats {
		t.Run("format_"+format, func(t *testing.T) {
			resetPullFlags()
			configFile = writePullTempConfig(t)
			verbose = false
			pullSpace = "DOCS"
			pullIDOrTitle = "123"
			pullFormat = format

			mock := confluence.NewMockClient()
			page := &confluence.Page{ID: "123", Title: "Test Page"}
			page.Body.Storage.Value = "<p>storage content</p>"
			page.Body.View.Value = "<p>view content</p>"
			mock.Pages["123"] = page
			newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
				return mock
			}

			err := runPull(pullCmd, nil)
			if err != nil {
				t.Fatalf("unexpected error for format %q: %v", format, err)
			}
		})
	}
}

func TestRunPull_MissingSpace(t *testing.T) {
	resetPullFlags()
	configFile = writePullTempConfig(t)
	verbose = false
	pullSpace = ""
	pullIDOrTitle = "123"
	pullFormat = "storage"
	pullProject = ""

	err := runPull(pullCmd, nil)
	if err == nil {
		t.Fatal("expected error for missing space")
	}
	if err.Error() != "space flag or --project required for pull command" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPull_ProjectSelectionInfersSpace(t *testing.T) {
	resetPullFlags()
	configFile = writePullTempConfig(t)
	verbose = false
	pullSpace = ""
	pullIDOrTitle = "123"
	pullFormat = "storage"
	pullProject = "myproject"

	mock := confluence.NewMockClient()
	page := &confluence.Page{ID: "123", Title: "Test Page"}
	page.Body.Storage.Value = "content"
	mock.Pages["123"] = page
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
		return mock
	}

	err := runPull(pullCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPull_PageLookupByNumericID(t *testing.T) {
	resetPullFlags()
	configFile = writePullTempConfig(t)
	verbose = false
	pullSpace = "DOCS"
	pullIDOrTitle = "12345"
	pullFormat = "storage"

	mock := confluence.NewMockClient()
	page := &confluence.Page{ID: "12345", Title: "Page By ID"}
	page.Body.Storage.Value = "content"
	mock.Pages["12345"] = page
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
		return mock
	}

	err := runPull(pullCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPull_PageLookupByTitle(t *testing.T) {
	resetPullFlags()
	configFile = writePullTempConfig(t)
	verbose = false
	pullSpace = "DOCS"
	pullIDOrTitle = "My Page Title"
	pullFormat = "storage"

	mock := confluence.NewMockClient()
	page := &confluence.Page{ID: "999", Title: "My Page Title"}
	page.Body.Storage.Value = "content"
	mock.PagesByTitle["DOCS:My Page Title"] = page
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
		return mock
	}

	err := runPull(pullCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPull_EditableArtifactRefreshesTitleResult(t *testing.T) {
	resetPullFlags()
	configFile = writePullTempConfig(t)
	verbose = false
	pullSpace = "DOCS"
	pullIDOrTitle = "My Page Title"
	pullOutput = filepath.Join(t.TempDir(), "page.md")

	mock := confluence.NewMockClient()
	mock.PagesByTitle["DOCS:My Page Title"] = &confluence.Page{ID: "999", Title: "My Page Title"}
	page := &confluence.Page{ID: "999", Title: "My Page Title"}
	page.Version.Number = 4
	page.Body.Storage.Value = "<p>editable content</p>"
	mock.Pages[page.ID] = page
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
		return mock
	}

	if err := runPull(pullCmd, nil); err != nil {
		t.Fatalf("runPull returned error: %v", err)
	}
	metadata, err := content.LoadArtifactMetadata(pullOutput)
	if err != nil {
		t.Fatalf("load artifact metadata: %v", err)
	}
	if metadata.Page.BaseVersion != 4 {
		t.Fatalf("base version = %d, want 4", metadata.Page.BaseVersion)
	}
}

func TestRunPull_PageNotFound(t *testing.T) {
	resetPullFlags()
	configFile = writePullTempConfig(t)
	verbose = false
	pullSpace = "DOCS"
	pullIDOrTitle = "nonexistent"
	pullFormat = "storage"

	mock := confluence.NewMockClient()
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
		return mock
	}

	err := runPull(pullCmd, nil)
	if err == nil {
		t.Fatal("expected error for page not found")
	}
	expected := "page 'nonexistent' not found in space 'DOCS'"
	if err.Error() != expected {
		t.Fatalf("unexpected error: got %q, want %q", err.Error(), expected)
	}
}

// --- preprocessConfluenceImages tests ---

func TestPreprocessConfluenceImages_BasicReplacement(t *testing.T) {
	input := `<ac:image><ri:attachment ri:filename="image.png" /></ac:image>`
	expected := `![image.png](attachments/image.png)`
	result := preprocessConfluenceImages(input)
	if result != expected {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

func TestPreprocessConfluenceImages_SpacesInFilename(t *testing.T) {
	input := `<ac:image><ri:attachment ri:filename="my image file.png" /></ac:image>`
	expected := `![my image file.png](attachments/my%20image%20file.png)`
	result := preprocessConfluenceImages(input)
	if result != expected {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

func TestPreprocessConfluenceImages_NoMatch(t *testing.T) {
	input := `<p>No image here</p>`
	result := preprocessConfluenceImages(input)
	if result != input {
		t.Fatalf("expected no change, got %q", result)
	}
}

func TestPreprocessConfluenceImages_MultipleImages(t *testing.T) {
	input := `<ac:image><ri:attachment ri:filename="a.png" /></ac:image>text<ac:image><ri:attachment ri:filename="b.jpg" /></ac:image>`
	result := preprocessConfluenceImages(input)
	if result != `![a.png](attachments/a.png)text![b.jpg](attachments/b.jpg)` {
		t.Fatalf("unexpected result: %q", result)
	}
}

// --- preprocessConfluenceMacros tests ---

func TestPreprocessConfluenceMacros_TOCRemoval(t *testing.T) {
	input := `<p>Before</p><ac:structured-macro ac:name="toc" ac:schema-version="1"/><p>After</p>`
	result := preprocessConfluenceMacros(input)
	expected := `<p>Before</p><p>After</p>`
	if result != expected {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

func TestPreprocessConfluenceMacros_InfoNoteToBlockquote(t *testing.T) {
	input := `<ac:structured-macro ac:name="info"><ac:rich-text-body>Important note</ac:rich-text-body></ac:structured-macro>`
	result := preprocessConfluenceMacros(input)
	if result != "\n> Important note\n" {
		t.Fatalf("unexpected result: %q", result)
	}
}

func TestPreprocessConfluenceMacros_InlineCommentRemoval(t *testing.T) {
	input := `<p>Text<ac:inline-comment-marker>comment</ac:inline-comment-marker>more</p>`
	result := preprocessConfluenceMacros(input)
	expected := `<p>Textmore</p>`
	if result != expected {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

func TestPreprocessConfluenceMacros_ViewFileToLink(t *testing.T) {
	input := `<ac:structured-macro ac:name="view-file"><ac:parameter ac:name="name"><ri:attachment ri:filename="doc.pdf" /></ac:parameter></ac:structured-macro>`
	result := preprocessConfluenceMacros(input)
	expected := `[doc.pdf](attachments/doc.pdf)`
	if result != expected {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

func TestPreprocessConfluenceMacros_CodeWithLanguage(t *testing.T) {
	input := `<ac:structured-macro ac:name="code"><ac:parameter ac:name="language">python</ac:parameter><ac:plain-text-body><![CDATA[print("hello")]]></ac:plain-text-body></ac:structured-macro>`
	result := preprocessConfluenceMacros(input)
	expected := `<pre><code class="language-python">print(&#34;hello&#34;)</code></pre>`
	if result != expected {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

func TestPreprocessConfluenceMacros_CodeWithoutLanguage(t *testing.T) {
	input := `<ac:structured-macro ac:name="code"><ac:plain-text-body><![CDATA[echo test]]></ac:plain-text-body></ac:structured-macro>`
	result := preprocessConfluenceMacros(input)
	expected := `<pre><code>echo test</code></pre>`
	if result != expected {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

// --- generatePageOutput tests ---

func TestGeneratePageOutput_StorageFormat(t *testing.T) {
	page := &confluence.Page{}
	page.Body.Storage.Value = "<p>storage value</p>"
	page.Body.View.Value = "<p>view value</p>"
	result, err := generatePageOutput(page, "storage")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "<p>storage value</p>" {
		t.Fatalf("got %q, want %q", result, "<p>storage value</p>")
	}
}

func TestGeneratePageOutput_HTMLFormatWithView(t *testing.T) {
	page := &confluence.Page{}
	page.Body.Storage.Value = "<p>storage value</p>"
	page.Body.View.Value = "<p>view value</p>"
	result, err := generatePageOutput(page, "html")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "<p>view value</p>" {
		t.Fatalf("got %q, want %q", result, "<p>view value</p>")
	}
}

func TestGeneratePageOutput_HTMLFormatFallsBackToStorage(t *testing.T) {
	page := &confluence.Page{}
	page.Body.Storage.Value = "<p>storage value</p>"
	page.Body.View.Value = ""
	result, err := generatePageOutput(page, "html")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "<p>storage value</p>" {
		t.Fatalf("got %q, want %q", result, "<p>storage value</p>")
	}
}

func TestGeneratePageOutput_MarkdownFormat(t *testing.T) {
	page := &confluence.Page{}
	page.Body.Storage.Value = ""
	page.Body.View.Value = "<p>Hello <strong>world</strong></p>"
	result, err := generatePageOutput(page, "markdown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello **world**" {
		t.Fatalf("got %q", result)
	}
}

func TestGeneratePageOutput_UnsupportedFormat(t *testing.T) {
	page := &confluence.Page{}
	_, err := generatePageOutput(page, "xml")
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
	if err.Error() != "unsupported format: xml" {
		t.Fatalf("unexpected error: %v", err)
	}
}
