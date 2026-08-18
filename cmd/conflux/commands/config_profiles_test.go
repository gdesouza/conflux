package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigProfilesListsSortedProfilesAndMarksConfiguredDefault(t *testing.T) {
	path := writeProfilesConfig(t, `
projects:
  - name: zeta
    space_key: ZETA
  - name: alpha
    space_key: ALPHA
`)

	stdout, _, err := runCmdForTest(t, []string{"config", "profiles", "--config", path})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "Configured Profiles:") {
		t.Fatalf("missing heading: %q", stdout)
	}
	if !strings.Contains(stdout, "- zeta (default)\n  space: ZETA") {
		t.Fatalf("configured default was not marked: %q", stdout)
	}
	if strings.Index(stdout, "- alpha") > strings.Index(stdout, "- zeta") {
		t.Fatalf("profiles were not sorted by name: %q", stdout)
	}
}

func TestConfigProfilesReportsEmptyProfiles(t *testing.T) {
	path := writeProfilesConfig(t, "")

	stdout, _, err := runCmdForTest(t, []string{"config", "profiles", "--config", path})
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "No profiles configured.\n" {
		t.Fatalf("unexpected output: %q", stdout)
	}
}

func TestConfigProfilesWrapsLoadFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")

	_, _, err := runCmdForTest(t, []string{"config", "profiles", "--config", path})
	if err == nil || !strings.Contains(err.Error(), "failed to load config") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeProfilesConfig(t *testing.T, projects string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := `confluence:
  base_url: https://example.atlassian.net
  username: user@example.com
  api_token: token
` + projects
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
