package commands

import (
	"io"
	"os"
	"strings"
	"testing"

	"conflux/internal/confluence"
	"conflux/pkg/logger"
)

func writeTempConfigGetPage(t *testing.T) string {
	const cfg = `confluence:
  base_url: http://example
  username: u
  api_token: t
  space_key: DOCS
local:
  markdown_dir: ./docs
mermaid:
  mode: preserve
`
	f, err := os.CreateTemp(t.TempDir(), "cfg-*.yaml")
	if err != nil {
		t.Fatalf("temp config: %v", err)
	}
	if _, err := f.WriteString(cfg); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close cfg: %v", err)
	}
	return f.Name()
}

func captureStdoutGetPage(fn func()) string {
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig
	b, _ := io.ReadAll(r)
	return string(b)
}

func withMockClientGetPage(t *testing.T, mc *confluence.MockClient, fn func()) {
	orig := newConfluenceClient
	newConfluenceClient = func(baseURL, user, token string, log *logger.Logger) confluence.ConfluenceClient { return mc }
	defer func() { newConfluenceClient = orig }()
	fn()
}

func TestGetPage_ByID(t *testing.T) {
	mc := confluence.NewMockClient()
	p := &confluence.Page{ID: "42", Title: "Answer"}
	p.Body.Storage.Value = "<p>Life</p>"
	mc.Pages[p.ID] = p
	mc.PagesByTitle["DOCS:"+p.Title] = p

	cfgPath := writeTempConfigGetPage(t)
	configFile = cfgPath
	verbose = false
	pullSpace = "DOCS"
	pullProject = ""     // ensure no leftover project selection
	pullIDOrTitle = "42" // numeric triggers ID path
	pullFormat = "storage"

	out := captureStdoutGetPage(func() {
		withMockClientGetPage(t, mc, func() {
			if err := runPull(nil, nil); err != nil {
				t.Fatalf("runPull: %v", err)
			}
		})
	})
	if !strings.Contains(out, "# Answer (ID: 42)") {
		t.Fatalf("missing header: %s", out)
	}
	if !strings.Contains(out, "Life") {
		t.Fatalf("missing content: %s", out)
	}
}

func TestGetPage_FallbackToTitle(t *testing.T) {
	mc := confluence.NewMockClient()
	p := &confluence.Page{ID: "50", Title: "Guide"}
	p.Body.Storage.Value = "<p>Guide Content</p>"
	mc.PagesByTitle["DOCS:"+p.Title] = p

	cfgPath := writeTempConfigGetPage(t)
	configFile = cfgPath
	verbose = false
	pullSpace = "DOCS"
	pullProject = ""        // ensure no leftover project selection
	pullIDOrTitle = "Guide" // non-numeric triggers title search
	pullFormat = "html"

	out := captureStdoutGetPage(func() {
		withMockClientGetPage(t, mc, func() {
			if err := runPull(nil, nil); err != nil {
				t.Fatalf("runPull: %v", err)
			}
		})
	})
	if !strings.Contains(out, "# Guide (ID: 50)") {
		t.Fatalf("missing header: %s", out)
	}
	if !strings.Contains(out, "Guide Content") {
		t.Fatalf("missing content: %s", out)
	}
}

func TestGetPage_PageNotFound(t *testing.T) {
	mc := confluence.NewMockClient()
	cfgPath := writeTempConfigGetPage(t)
	configFile = cfgPath
	verbose = false
	pullSpace = "DOCS"
	pullProject = "" // ensure no leftover project selection
	pullIDOrTitle = "Missing"
	pullFormat = "storage"

	withMockClientGetPage(t, mc, func() {
		if err := runPull(nil, nil); err == nil {
			t.Fatalf("expected error for missing page")
		}
	})
}
