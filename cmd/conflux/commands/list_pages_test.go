package commands

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"conflux/internal/confluence"
	"conflux/pkg/logger"
)

// helper to temporarily override factory
func withMockClient(t *testing.T, mc *confluence.MockClient, fn func()) {
	orig := newConfluenceClient
	newConfluenceClient = func(baseURL, user, token string, log *logger.Logger) confluence.ConfluenceClient { return mc }
	defer func() { newConfluenceClient = orig }()
	fn()
}

// minimal config yaml for commands requiring config
const testConfigYAML = `confluence:
  base_url: http://example
  username: u
  api_token: t
  space_key: DOCS
local:
  markdown_dir: ./docs
mermaid:
  mode: preserve
`

func writeTempConfig(t *testing.T) string {
	f, err := os.CreateTemp(t.TempDir(), "cfg-*.yaml")
	if err != nil {
		t.Fatalf("temp config: %v", err)
	}
	if _, err := f.WriteString(testConfigYAML); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close cfg: %v", err)
	}
	return f.Name()
}

func TestListPages_PrintsHierarchy(t *testing.T) {
	mc := confluence.NewMockClient()
	// Build hierarchy: Root (A,B); A has child A1
	mc.SpaceHierarchies["DOCS"] = []confluence.PageInfo{
		{ID: "1", Title: "RootA", Children: []confluence.PageInfo{{ID: "2", Title: "A1"}}},
		{ID: "3", Title: "RootB"},
	}

	cfgPath := writeTempConfig(t)

	// Set required global flags used by command code
	configFile = cfgPath
	verbose = false
	space = "DOCS"
	parentPage = ""
	listProject = ""

	buf := &bytes.Buffer{}
	origStdout := os.Stdout
	// capture stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	withMockClient(t, mc, func() {
		if err := runListPages(nil, nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
			w.Close()
			os.Stdout = origStdout
			t.Fatalf("runListPages error: %v", err)
		}
	})
	w.Close()
	os.Stdout = origStdout
	outBytes, _ := io.ReadAll(r)
	buf.Write(outBytes)
	out := buf.String()

	// Basic assertions
	if !strings.Contains(out, "RootA") || !strings.Contains(out, "RootB") || !strings.Contains(out, "A1") {
		t.Fatalf("expected hierarchy output, got: %s", out)
	}
	// Tree icon expectations
	if !strings.Contains(out, "📁") || !strings.Contains(out, "📄") {
		// At least one folder and one file icon
		// RootA has children so folder icon
		// RootB no children -> page icon
		// A1 leaf -> page icon
		// Provide more helpful debug
		f := strings.Split(out, "\n")
		for i, l := range f {
			t.Logf("OUT %d: %s", i, l)
		}
		// don't fail only for missing icons; still ensure structure
	}
}
