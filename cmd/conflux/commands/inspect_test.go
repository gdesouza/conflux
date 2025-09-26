package commands

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"conflux/internal/confluence"
	"conflux/pkg/logger"
)

// withMockClient reused from list_pages_test.go when in same package; define here if run independently
func withMockClientInspect(t *testing.T, mc *confluence.MockClient, fn func()) {
	orig := newConfluenceClient
	newConfluenceClient = func(baseURL, user, token string, log *logger.Logger) confluence.ConfluenceClient { return mc }
	defer func() { newConfluenceClient = orig }()
	fn()
}

// writeTempConfig reused; replicate minimal to keep tests independent
func writeTempConfigInspect(t *testing.T) string {
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

func captureStdout(fn func()) string {
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig
	b, _ := io.ReadAll(r)
	return string(b)
}

func TestInspect_SpaceOverview(t *testing.T) {
	mc := confluence.NewMockClient()
	mc.SpaceHierarchies["DOCS"] = []confluence.PageInfo{
		{ID: "1", Title: "RootA", Children: []confluence.PageInfo{{ID: "2", Title: "ChildA1"}}},
		{ID: "3", Title: "RootB"},
	}
	cfgPath := writeTempConfigInspect(t)
	configFile = cfgPath
	verbose = false
	inspectSpace = "DOCS"
	inspectProject = "" // ensure no leftover project
	inspectPage = ""    // overview

	out := captureStdout(func() {
		withMockClientInspect(t, mc, func() {
			if err := runInspect(nil, nil); err != nil {
				t.Fatalf("runInspect: %v", err)
			}
		})
	})

	if !strings.Contains(out, "Inspecting Space: DOCS") {
		t.Fatalf("expected space header, got: %s", out)
	}
	if !strings.Contains(out, "RootA") || !strings.Contains(out, "RootB") || !strings.Contains(out, "ChildA1") {
		fmt.Println(out)
		t.Fatalf("expected hierarchy output, got: %s", out)
	}
	if !strings.Contains(out, "Summary:") {
		t.Fatalf("expected summary section, got: %s", out)
	}
}

func TestInspect_PageDetails_ByTitle(t *testing.T) {
	mc := confluence.NewMockClient()
	// Target page and its context
	page := &confluence.Page{ID: "10", Title: "Target"}
	page.Body.Storage.Value = "<p>Content</p>"
	mc.Pages[page.ID] = page
	// Register by title for lookup
	mc.PagesByTitle["DOCS:"+page.Title] = page
	mc.Ancestors[page.ID] = []confluence.PageInfo{{ID: "1", Title: "Root"}}
	mc.Children[page.ID] = []confluence.PageInfo{{ID: "11", Title: "Child1"}}

	cfgPath := writeTempConfigInspect(t)
	configFile = cfgPath
	verbose = false
	inspectSpace = "DOCS"
	inspectPage = "Target"
	showDetails = true

	out := captureStdout(func() {
		withMockClientInspect(t, mc, func() {
			if err := runInspect(nil, nil); err != nil {
				t.Fatalf("runInspect: %v", err)
			}
		})
	})

	if !strings.Contains(out, "Inspecting Page: Target") {
		t.Fatalf("missing page header: %s", out)
	}
	if !strings.Contains(out, "Parent Chain") {
		t.Fatalf("missing parent chain: %s", out)
	}
	if !strings.Contains(out, "Root") {
		t.Fatalf("missing ancestor: %s", out)
	}
	if !strings.Contains(out, "Child1") {
		t.Fatalf("missing child listing: %s", out)
	}
	if !strings.Contains(out, "Content Length") {
		t.Fatalf("expected details output: %s", out)
	}
}

func TestInspect_PageNotFound(t *testing.T) {
	mc := confluence.NewMockClient()
	cfgPath := writeTempConfigInspect(t)
	configFile = cfgPath
	verbose = false
	inspectSpace = "DOCS"
	inspectPage = "DoesNotExist"

	withMockClientInspect(t, mc, func() {
		if err := runInspect(nil, nil); err == nil {
			// Expect error
			if err == nil {
				t.Fatalf("expected error for missing page")
			}
		}
	})
}
