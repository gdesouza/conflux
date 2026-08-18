package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidatesCredentialsAndProfiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := `confluence:
  base_url: https://example.atlassian.net
  username: user@example.com
  api_token: token
projects:
  - name: docs
    space_key: DOCS
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Projects) != 1 || cfg.Projects[0].SpaceKey != "DOCS" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestLoadRejectsIncompleteConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("confluence: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "base_url is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRuntimeAllowsSpaceSelectionLater(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "confluence:\n  base_url: https://example\n  username: user\n  api_token: token\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRuntime(path); err != nil {
		t.Fatal(err)
	}
}
