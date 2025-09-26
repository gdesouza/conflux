package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectsListsAndMarksDefault(t *testing.T) {
	cfgData := `confluence:
  base_url: https://example
  username: u
  api_token: t
projects:
  - name: alpha
    space_key: ALPHA
    local:
      markdown_dir: ./alpha
  - name: beta
    space_key: BETA
    local:
      markdown_dir: ./beta
`
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgData), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	out, _, err := runCmdForTest(t, []string{"projects", "--config", cfgPath})
	if err != nil {
		t.Fatalf("projects command unexpected error: %v", err)
	}

	if !strings.Contains(out, "Configured Projects:") {
		t.Fatalf("expected header in output: %s", out)
	}
	// Default marker must appear exactly once and on the first project name (alpha)
	if !strings.Contains(out, "alpha (default)") {
		t.Fatalf("expected alpha marked default: %s", out)
	}
	if strings.Count(out, "(default)") != 1 {
		t.Fatalf("expected single default marker: %s", out)
	}
	// Should list beta without default marker
	if !strings.Contains(out, "- beta\n  space: BETA") {
		t.Fatalf("expected beta project line: %s", out)
	}
}

func TestProjectsNoProjectsMessage(t *testing.T) {
	cfgData := `confluence:
  base_url: https://example
  username: u
  api_token: t
  space_key: LEGACY
`
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgData), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	out, _, err := runCmdForTest(t, []string{"projects", "--config", cfgPath})
	if err != nil {
		t.Fatalf("projects command unexpected error: %v", err)
	}
	if !strings.Contains(out, "No projects defined") {
		t.Fatalf("expected legacy message, got: %s", out)
	}
}
