package commands

import (
	"strings"
	"testing"
)

// Error-path tests for pull command argument / selection validation.
// These cover branches that return early before any Confluence client interaction.

func TestGetPageError_MissingPageFlag(t *testing.T) {
	// Reset globals
	pullIDOrTitle = "" // triggers first validation error
	pullFormat = "storage"
	pullSpace = "DOCS" // even if space present, page flag check runs first
	pullProject = ""

	if err := runPull(nil, nil); err == nil || !strings.Contains(err.Error(), "page flag is required") {
		if err == nil {
			t.Fatalf("expected error when page flag missing")
		}
		// Provided wrong error
		// Fail with actual error content
		// (No config load occurs before this validation)
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetPageError_UnsupportedFormat(t *testing.T) {
	cfgPath := writeTempConfigGetPage(t)
	configFile = cfgPath
	pullIDOrTitle = "SomePage"
	pullFormat = "weird" // invalid
	pullSpace = "DOCS"
	pullProject = ""

	if err := runPull(nil, nil); err == nil || !strings.Contains(err.Error(), "unsupported format") {
		if err == nil {
			t.Fatalf("expected unsupported format error")
		}
		// Provided wrong error
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetPageError_MissingSpaceAndProject(t *testing.T) {
	cfgPath := writeTempConfigGetPage(t)
	configFile = cfgPath
	pullIDOrTitle = "SomePage"
	pullFormat = "storage"
	pullSpace = ""   // not provided
	pullProject = "" // not provided

	if err := runPull(nil, nil); err == nil || !strings.Contains(err.Error(), "space flag or --project required") {
		if err == nil {
			t.Fatalf("expected missing space/project error")
		}
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetPageError_BadProjectSelection(t *testing.T) {
	cfgPath := writeTempConfigGetPage(t)
	configFile = cfgPath
	pullIDOrTitle = "SomePage"
	pullFormat = "storage"
	pullSpace = ""          // rely on project (which fails)
	pullProject = "unknown" // project does not exist in config

	if err := runPull(nil, nil); err == nil || !strings.Contains(err.Error(), "failed to select project") {
		if err == nil {
			t.Fatalf("expected project selection failure")
		}
		t.Fatalf("unexpected error: %v", err)
	}
}
