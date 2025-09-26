package commands

import "testing"
import "strings"

// Error-path tests for list-pages command.

func TestListPagesError_MissingSpaceAndProject(t *testing.T) {
	cfgPath := writeTempConfig(t)
	configFile = cfgPath
	space = "" // neither space nor project provided
	listProject = ""
	parentPage = ""

	if err := runListPages(nil, nil); err == nil || !strings.Contains(err.Error(), "space flag or --project required") {
		if err == nil {
			t.Fatalf("expected missing space/project error")
		}
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListPagesError_BadProjectSelection(t *testing.T) {
	cfgPath := writeTempConfig(t)
	configFile = cfgPath
	space = ""          // will attempt project selection
	listProject = "bad" // project does not exist in config
	parentPage = ""

	if err := runListPages(nil, nil); err == nil || !strings.Contains(err.Error(), "failed to select project") {
		if err == nil {
			t.Fatalf("expected project selection failure")
		}
		t.Fatalf("unexpected error: %v", err)
	}
}
