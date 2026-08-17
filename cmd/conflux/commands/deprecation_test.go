package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestDeprecatedCommandsIdentifyReplacementAndRemoval(t *testing.T) {
	for _, command := range []*cobra.Command{syncCmd, projectsCmd} {
		if !strings.Contains(command.Deprecated, "v2.0") || !strings.Contains(command.Deprecated, "conflux") {
			t.Fatalf("%s deprecation is not actionable: %q", command.Name(), command.Deprecated)
		}
	}
}

func TestConfigProfilesListsConfiguredProfiles(t *testing.T) {
	configFile = writePagesTestConfig(t, pagesTestConfigWithProjectsYAML)
	var output bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&output)
	if err := runConfigProfiles(command, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "myproject (default)") || !strings.Contains(output.String(), "OTHER") {
		t.Fatalf("profiles output:\n%s", output.String())
	}
}
