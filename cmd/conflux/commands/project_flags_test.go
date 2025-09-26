package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper run command with args capturing output/error
func runCmdForTest(t *testing.T, args []string) (stdout string, stderr string, err error) {
	t.Helper()
	// Cobra uses the same rootCmd singleton; replace its output writers
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs(args)
	err = rootCmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

func writeConfig(t *testing.T, dir string, data string) string {
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(data), 0600); err != nil {
		t.Fatalf("failed writing config: %v", err)
	}
	return p
}

func TestGetPageProjectInferenceMissingSpace(t *testing.T) {
	// minimal config with project and mock creds
	cfgData := `confluence:\n  base_url: https://example\n  username: u\n  api_token: t\nprojects:\n  - name: site\n    space_key: SITE\n    local:\n      markdown_dir: ./docs\n`
	tmp := t.TempDir()
	_ = writeConfig(t, tmp, cfgData)

	// We cannot actually hit Confluence; we just assert early validation error about page requirement shows project inference happened for space (space error should not occur). We request format=storage with page missing.
	args := []string{"get-page", "--config", filepath.Join(tmp, "config.yaml"), "--project", "site", "--page", "123"}

	// Command will attempt network access after resolving config and before failing if page not found. We expect an error referencing fetch failure (since client will attempt). To avoid real network, base_url is https://example (won't resolve). We only assert it tried to use space SITE (no explicit output includes space before fetch). To ensure deterministic behavior, we accept any error but not the error complaining about missing space flag.
	_, _, err := runCmdForTest(t, args)
	if err == nil {
		t.Fatalf("expected error (network or not found) but got none")
	}
	if strings.Contains(err.Error(), "space flag") {
		t.Fatalf("unexpected space flag error, project inference failed: %v", err)
	}
}

func TestInspectProjectInferenceRequiresNoSpace(t *testing.T) {
	cfgData := `confluence:\n  base_url: https://example\n  username: u\n  api_token: t\nprojects:\n  - name: core\n    space_key: CORE\n    local:\n      markdown_dir: ./core\n`
	tmp := t.TempDir()
	_ = writeConfig(t, tmp, cfgData)

	args := []string{"inspect", "--config", filepath.Join(tmp, "config.yaml"), "--project", "core"}
	_, _, err := runCmdForTest(t, args)
	if err == nil {
		// Network likely attempted; treat nil as pass (unlikely) because we cannot complete without remote.
		return
	}
	if strings.Contains(err.Error(), "space flag") {
		t.Fatalf("unexpected space requirement error: %v", err)
	}
}
