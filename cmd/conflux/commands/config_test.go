package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test non-interactive config usage with --set and --add-project and --print
func TestConfigNonInteractivePrint(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")

	args := []string{"config",
		"--config", cfgPath,
		"--non-interactive",
		"--yes",
		"--print",
		"--set", "confluence.base_url=https://example",
		"--set", "confluence.username=user",
		"--set", "confluence.api_token=tok",
		"--set", "mermaid.mode=preserve",
		"--set", "mermaid.format=png",
		"--add-project", "name=docs,space_key=DOCS,markdown_dir=./docs,exclude=README.md",
	}
	out, _, err := runCmdForTest(t, args)
	if err != nil { // validation should pass because project supplies space key
		t.Fatalf("config command error: %v", err)
	}
	// Should not write file because --print.
	if _, statErr := os.Stat(cfgPath); !os.IsNotExist(statErr) {
		t.Fatalf("print mode wrote the config file or returned an unexpected stat error: %v", statErr)
	}
	// Output YAML should contain key sections and applied overrides
	mustContain := []string{
		"confluence:",
		"base_url: https://example",
		"username: user",
		"api_token: tok",
		"projects:",
		"- name: docs",
		"space_key: DOCS",
		"markdown_dir: ./docs",
		"exclude:",
		"mermaid:",
		"mode: preserve",
		"format: png",
	}
	for _, m := range mustContain {
		if !strings.Contains(out, m) {
			t.Fatalf("expected output to contain %q. Full output: %s", m, out)
		}
	}
	if strings.Contains(out, "Configuration saved") {
		t.Fatalf("did not expect save confirmation in print mode: %s", out)
	}
}

// Test that running config without --print writes the file
func TestConfigWritesFile(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	args := []string{"config",
		"--config", cfgPath,
		"--non-interactive",
		"--yes",
		"--set", "confluence.base_url=https://example",
		"--set", "confluence.username=user",
		"--set", "confluence.api_token=tok",
		"--set", "confluence.space_key=SPACE",
	}
	out, _, err := runCmdForTest(t, args)
	if err != nil {
		t.Fatalf("config command error: %v", err)
	}
	if !strings.Contains(out, "Configuration saved") {
		t.Fatalf("expected save confirmation, got: %s", out)
	}
	data, readErr := os.ReadFile(cfgPath)
	if readErr != nil {
		t.Fatalf("expected config file written: %v", readErr)
	}
	content := string(data)
	if !strings.Contains(content, "base_url: https://example") || !strings.Contains(content, "space_key: SPACE") {
		t.Fatalf("written config missing expected fields: %s", content)
	}
}

// Test invalid --set key returns an error
func TestConfigInvalidSetKey(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	args := []string{"config",
		"--config", cfgPath,
		"--non-interactive",
		"--yes",
		"--set", "confluence.unknown_field=value",
	}
	_, _, err := runCmdForTest(t, args)
	if err == nil || !strings.Contains(err.Error(), "unsupported key") {
		t.Fatalf("expected unsupported key error, got: %v", err)
	}
}
