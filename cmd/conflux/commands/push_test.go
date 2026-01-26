package commands

import (
	"os"
	"path/filepath"
	"testing"

	"conflux/internal/confluence"
	"conflux/pkg/logger"
)

const pushTestConfigYAML = `confluence:
  base_url: http://example
  username: u
  api_token: t
  space_key: DOCS
local:
  markdown_dir: ./docs
mermaid:
  mode: preserve
`

func writePushTempConfig(t *testing.T) string {
	f, err := os.CreateTemp(t.TempDir(), "cfg-*.yaml")
	if err != nil {
		t.Fatalf("temp config: %v", err)
	}
	if _, err := f.WriteString(pushTestConfigYAML); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close cfg: %v", err)
	}
	return f.Name()
}

func TestPushCreatesNewPage(t *testing.T) {
	// Prepare a temporary markdown file
	dir := t.TempDir()
	file := filepath.Join(dir, "test.md")
	content := "# Test Title\n\nSome body text." // Title should be "Test Title"
	if err := os.WriteFile(file, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// Temp config
	configFile = writePushTempConfig(t)
	verbose = false
	// Substitute global flags
	pushFile = file
	pushSpace = "DOCS"
	pushParent = ""

	// Use a mock client explicitly to inspect results
	mock := confluence.NewMockClient()
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient { return mock }

	// Run command logic
	if err := runPush(pushCmd, nil); err != nil {
		t.Fatalf("runPush returned error: %v", err)
	}

	if len(mock.CreateCalls) != 1 {
		t.Fatalf("expected 1 create call, got %d", len(mock.CreateCalls))
	}
	if mock.CreateCalls[0] != "Test Title" {
		t.Fatalf("unexpected created title: %s", mock.CreateCalls[0])
	}
}

func TestPushUpdatesExistingPage(t *testing.T) {
	mock := confluence.NewMockClient()
	// Seed existing page
	_, _ = mock.CreatePage("DOCS", "Existing Page", "old content")

	// Prepare file with same title (extracted from heading)
	dir := t.TempDir()
	file := filepath.Join(dir, "page.md")
	content := "# Existing Page\n\nNew body." // Title matches existing
	if err := os.WriteFile(file, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// Temp config
	configFile = writePushTempConfig(t)
	verbose = false

	pushFile = file
	pushSpace = "DOCS"
	pushParent = ""

	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient { return mock }

	if err := runPush(pushCmd, nil); err != nil {
		t.Fatalf("runPush returned error: %v", err)
	}

	if len(mock.UpdateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(mock.UpdateCalls))
	}
	if mock.UpdateCalls[0] != "Existing Page" {
		t.Fatalf("unexpected updated title: %s", mock.UpdateCalls[0])
	}
}

func TestPushParentResolutionNumeric(t *testing.T) {
	mock := confluence.NewMockClient()

	// temp file
	dir := t.TempDir()
	file := filepath.Join(dir, "child.md")
	if err := os.WriteFile(file, []byte("# Child\nBody"), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// Temp config
	configFile = writePushTempConfig(t)
	verbose = false

	pushFile = file
	pushSpace = "DOCS"
	pushParent = "12345" // numeric treated as ID

	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient { return mock }

	if err := runPush(pushCmd, nil); err != nil {
		t.Fatalf("runPush returned error: %v", err)
	}

	if len(mock.CreateCalls) != 1 {
		t.Fatalf("expected create call, got %d", len(mock.CreateCalls))
	}
}

func TestPushParentResolutionByTitle(t *testing.T) {
	mock := confluence.NewMockClient()
	// Seed parent page
	parent, _ := mock.CreatePage("DOCS", "Parent", "content")
	if parent == nil {
		t.Fatalf("failed to seed parent page")
	}

	// temp file
	dir := t.TempDir()
	file := filepath.Join(dir, "child2.md")
	if err := os.WriteFile(file, []byte("# Child2\nBody"), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	configFile = writePushTempConfig(t)
	verbose = false

	pushFile = file
	pushSpace = "DOCS"
	pushParent = "Parent" // title should resolve to ID

	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient { return mock }

	if err := runPush(pushCmd, nil); err != nil {
		t.Fatalf("runPush returned error: %v", err)
	}

	if len(mock.CreateCalls) == 0 {
		t.Fatalf("expected create call")
	}
}
