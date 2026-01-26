package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const syncTestConfigYAML = `confluence:
  base_url: https://example.atlassian.net
  username: testuser
  api_token: testtoken
  space_key: DOCS
local:
  markdown_dir: ./docs
mermaid:
  mode: preserve
`

const syncTestConfigNoSpaceYAML = `confluence:
  base_url: https://example.atlassian.net
  username: testuser
  api_token: testtoken
local:
  markdown_dir: ./docs
mermaid:
  mode: preserve
`

const syncTestConfigWithProjectsYAML = `confluence:
  base_url: https://example.atlassian.net
  username: testuser
  api_token: testtoken
local:
  markdown_dir: ./docs
projects:
  - name: main
    space_key: MAIN
    local:
      markdown_dir: ./main-docs
  - name: api
    space_key: API
    local:
      markdown_dir: ./api-docs
mermaid:
  mode: preserve
`

func writeSyncTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "sync-cfg-*.yaml")
	if err != nil {
		t.Fatalf("temp config: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close cfg: %v", err)
	}
	return f.Name()
}

func writeSyncTempConfigInDir(t *testing.T, dir, content string) string {
	t.Helper()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	return cfgPath
}

func resetSyncFlags() {
	dryRun = false
	force = false
	noCache = false
	forceStubs = false
	detectRenames = false
	docsDir = "."
	spaceKey = ""
	projectName = ""
}

func TestRunSync_ConfigLoadFailure(t *testing.T) {
	resetSyncFlags()

	configFile = "/nonexistent/path/config.yaml"
	verbose = false

	err := runSync(syncCmd, nil)
	if err == nil {
		t.Fatal("expected error for missing config")
	}
	if !strings.Contains(err.Error(), "failed to load config") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSync_MissingSpaceKeyReturnsError(t *testing.T) {
	resetSyncFlags()

	configFile = writeSyncTempConfig(t, syncTestConfigNoSpaceYAML)
	verbose = false

	err := runSync(syncCmd, nil)
	if err == nil {
		t.Fatal("expected error for missing space key")
	}
	if !strings.Contains(err.Error(), "space key is required") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRunSync_ProjectSelectionWorks(t *testing.T) {
	resetSyncFlags()

	tmp := t.TempDir()
	mainDocsDir := filepath.Join(tmp, "main-docs")
	if err := os.MkdirAll(mainDocsDir, 0755); err != nil {
		t.Fatalf("failed to create main-docs dir: %v", err)
	}

	configFile = writeSyncTempConfigInDir(t, tmp, syncTestConfigWithProjectsYAML)
	verbose = false
	projectName = "main"
	dryRun = true

	err := runSync(syncCmd, nil)
	if err != nil && strings.Contains(err.Error(), "space key is required") {
		t.Fatalf("project selection should provide space key: %v", err)
	}
}

func TestRunSync_DefaultProjectAppliedWhenNoSpaceKey(t *testing.T) {
	resetSyncFlags()

	tmp := t.TempDir()
	mainDocsDir := filepath.Join(tmp, "main-docs")
	if err := os.MkdirAll(mainDocsDir, 0755); err != nil {
		t.Fatalf("failed to create main-docs dir: %v", err)
	}

	configFile = writeSyncTempConfigInDir(t, tmp, syncTestConfigWithProjectsYAML)
	verbose = false
	dryRun = true

	err := runSync(syncCmd, nil)
	if err != nil && strings.Contains(err.Error(), "space key is required") {
		t.Fatalf("default project should provide space key: %v", err)
	}
}

func TestRunSync_SpaceFlagOverridesConfig(t *testing.T) {
	resetSyncFlags()

	tmp := t.TempDir()
	docsPath := filepath.Join(tmp, "docs")
	if err := os.MkdirAll(docsPath, 0755); err != nil {
		t.Fatalf("failed to create docs dir: %v", err)
	}

	configFile = writeSyncTempConfigInDir(t, tmp, syncTestConfigNoSpaceYAML)
	verbose = false
	spaceKey = "OVERRIDE"
	dryRun = true

	err := runSync(syncCmd, nil)
	if err != nil && strings.Contains(err.Error(), "space key is required") {
		t.Fatalf("--space flag should override config: %v", err)
	}
}

func TestRunSync_DocsFlagWithDirectoryPath(t *testing.T) {
	resetSyncFlags()

	tmp := t.TempDir()
	customDocsDir := filepath.Join(tmp, "custom-docs")
	if err := os.MkdirAll(customDocsDir, 0755); err != nil {
		t.Fatalf("failed to create custom-docs dir: %v", err)
	}

	configFile = writeSyncTempConfig(t, syncTestConfigYAML)
	verbose = false
	docsDir = customDocsDir
	dryRun = true

	err := runSync(syncCmd, nil)
	if err != nil && strings.Contains(err.Error(), "failed to access path") {
		t.Fatalf("--docs with valid directory should not fail path access: %v", err)
	}
}

func TestRunSync_DocsFlagWithSingleFilePath(t *testing.T) {
	resetSyncFlags()

	tmp := t.TempDir()
	testFile := filepath.Join(tmp, "test.md")
	if err := os.WriteFile(testFile, []byte("# Test\n\nContent"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	configFile = writeSyncTempConfig(t, syncTestConfigYAML)
	verbose = false
	docsDir = testFile
	dryRun = true

	err := runSync(syncCmd, nil)
	if err != nil && strings.Contains(err.Error(), "failed to access path") {
		t.Fatalf("--docs with valid file should not fail path access: %v", err)
	}
}

func TestRunSync_DocsFlagWithNonExistentPathReturnsError(t *testing.T) {
	resetSyncFlags()

	configFile = writeSyncTempConfig(t, syncTestConfigYAML)
	verbose = false
	docsDir = "/nonexistent/path/to/docs"

	err := runSync(syncCmd, nil)
	if err == nil {
		t.Fatal("expected error for non-existent docs path")
	}
	if !strings.Contains(err.Error(), "failed to access path") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRunSync_DryRunFlag(t *testing.T) {
	resetSyncFlags()

	tmp := t.TempDir()
	docsPath := filepath.Join(tmp, "docs")
	if err := os.MkdirAll(docsPath, 0755); err != nil {
		t.Fatalf("failed to create docs dir: %v", err)
	}
	testFile := filepath.Join(docsPath, "test.md")
	if err := os.WriteFile(testFile, []byte("# Test\n\nContent"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	configFile = writeSyncTempConfigInDir(t, tmp, syncTestConfigYAML)
	verbose = false
	docsDir = docsPath
	dryRun = true

	err := runSync(syncCmd, nil)
	if err != nil {
		t.Fatalf("dry run should complete without error: %v", err)
	}
}

func TestRunSync_DetectRenamesFlag(t *testing.T) {
	resetSyncFlags()

	tmp := t.TempDir()
	docsPath := filepath.Join(tmp, "docs")
	if err := os.MkdirAll(docsPath, 0755); err != nil {
		t.Fatalf("failed to create docs dir: %v", err)
	}
	testFile := filepath.Join(docsPath, "test.md")
	if err := os.WriteFile(testFile, []byte("# Test\n\nContent"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	configFile = writeSyncTempConfigInDir(t, tmp, syncTestConfigYAML)
	verbose = false
	docsDir = docsPath
	detectRenames = true

	err := runSync(syncCmd, nil)
	if err != nil {
		t.Fatalf("detect renames should complete without error: %v", err)
	}
}

func TestRunSync_InvalidProjectReturnsError(t *testing.T) {
	resetSyncFlags()

	tmp := t.TempDir()
	configFile = writeSyncTempConfigInDir(t, tmp, syncTestConfigWithProjectsYAML)
	verbose = false
	projectName = "nonexistent"

	err := runSync(syncCmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid project name")
	}
	if !strings.Contains(err.Error(), "failed to select project") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSyncCmd_FlagsAreRegistered(t *testing.T) {
	flags := []struct {
		name      string
		shorthand string
	}{
		{"dry-run", ""},
		{"force", ""},
		{"no-cache", ""},
		{"detect-renames", ""},
		{"force-stubs", ""},
		{"docs", "d"},
		{"space", "s"},
		{"project", "P"},
	}

	for _, f := range flags {
		flag := syncCmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("flag --%s should be registered", f.name)
			continue
		}
		if f.shorthand != "" && flag.Shorthand != f.shorthand {
			t.Errorf("flag --%s should have shorthand -%s, got -%s", f.name, f.shorthand, flag.Shorthand)
		}
	}
}

func TestSyncCmd_FlagDefaults(t *testing.T) {
	tests := []struct {
		name         string
		expectedVal  string
		expectedBool bool
		isBool       bool
	}{
		{"dry-run", "", false, true},
		{"force", "", false, true},
		{"no-cache", "", false, true},
		{"detect-renames", "", false, true},
		{"force-stubs", "", false, true},
		{"docs", ".", false, false},
		{"space", "", false, false},
		{"project", "", false, false},
	}

	for _, tt := range tests {
		flag := syncCmd.Flags().Lookup(tt.name)
		if flag == nil {
			t.Errorf("flag --%s should be registered", tt.name)
			continue
		}

		if tt.isBool {
			if flag.DefValue != "false" {
				t.Errorf("flag --%s should default to false, got %s", tt.name, flag.DefValue)
			}
		} else {
			if flag.DefValue != tt.expectedVal {
				t.Errorf("flag --%s should default to %q, got %q", tt.name, tt.expectedVal, flag.DefValue)
			}
		}
	}
}

func TestRunSync_ProjectOverridesMarkdownDir(t *testing.T) {
	resetSyncFlags()

	tmp := t.TempDir()

	apiDocsDir := filepath.Join(tmp, "api-docs")
	if err := os.MkdirAll(apiDocsDir, 0755); err != nil {
		t.Fatalf("failed to create api-docs dir: %v", err)
	}
	testFile := filepath.Join(apiDocsDir, "api.md")
	if err := os.WriteFile(testFile, []byte("# API\n\nAPI docs"), 0600); err != nil {
		t.Fatalf("failed to create api file: %v", err)
	}

	configWithAbsPath := `confluence:
  base_url: https://example.atlassian.net
  username: testuser
  api_token: testtoken
local:
  markdown_dir: ./docs
projects:
  - name: api
    space_key: API
    local:
      markdown_dir: ` + apiDocsDir + `
mermaid:
  mode: preserve
`

	configFile = writeSyncTempConfigInDir(t, tmp, configWithAbsPath)
	verbose = false
	projectName = "api"
	dryRun = true

	err := runSync(syncCmd, nil)
	if err != nil {
		t.Fatalf("project with valid markdown dir should work: %v", err)
	}
}

func TestRunSync_SpaceFlagOverridesProject(t *testing.T) {
	resetSyncFlags()

	tmp := t.TempDir()
	mainDocsDir := filepath.Join(tmp, "main-docs")
	if err := os.MkdirAll(mainDocsDir, 0755); err != nil {
		t.Fatalf("failed to create main-docs dir: %v", err)
	}

	configFile = writeSyncTempConfigInDir(t, tmp, syncTestConfigWithProjectsYAML)
	verbose = false
	projectName = "main"
	spaceKey = "CUSTOM"
	dryRun = true

	err := runSync(syncCmd, nil)
	if err != nil && strings.Contains(err.Error(), "space key is required") {
		t.Fatalf("space flag should work with project: %v", err)
	}
}

func TestRunSync_InvalidConfigYAMLReturnsError(t *testing.T) {
	resetSyncFlags()

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("invalid: yaml: content: ["), 0600); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	configFile = cfgPath
	verbose = false

	err := runSync(syncCmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "failed to load config") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRunSync_MissingCredentialsReturnsError(t *testing.T) {
	resetSyncFlags()

	incompleteConfig := `confluence:
  base_url: https://example.atlassian.net
  space_key: DOCS
local:
  markdown_dir: ./docs
mermaid:
  mode: preserve
`

	configFile = writeSyncTempConfig(t, incompleteConfig)
	verbose = false

	err := runSync(syncCmd, nil)
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
	if !strings.Contains(err.Error(), "failed to load config") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
