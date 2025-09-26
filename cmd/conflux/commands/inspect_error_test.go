package commands

import "testing"
import "strings"

// Error-path tests for inspect command.

func TestInspectError_MissingSpaceAndProject(t *testing.T) {
	cfgPath := writeTempConfigInspect(t)
	configFile = cfgPath
	inspectSpace = "" // neither space nor project
	inspectProject = ""
	inspectPage = "" // overview path, but still needs space or project

	if err := runInspect(nil, nil); err == nil || !strings.Contains(err.Error(), "space flag or --project required") {
		if err == nil {
			t.Fatalf("expected missing space/project error")
		}
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectError_BadProjectSelection(t *testing.T) {
	cfgPath := writeTempConfigInspect(t)
	configFile = cfgPath
	inspectSpace = ""       // rely on project selection
	inspectProject = "nope" // invalid project name
	inspectPage = ""        // overview path

	if err := runInspect(nil, nil); err == nil || !strings.Contains(err.Error(), "failed to select project") {
		if err == nil {
			t.Fatalf("expected project selection failure")
		}
		t.Fatalf("unexpected error: %v", err)
	}
}
