package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"conflux/internal/confluence"
	"conflux/pkg/logger"
)

const pagesTestConfigYAML = `confluence:
  base_url: http://example
  username: u
  api_token: t
  space_key: DOCS
local:
  markdown_dir: ./docs
`

const pagesTestConfigWithProjectsYAML = `confluence:
  base_url: http://example
  username: u
  api_token: t
projects:
  - name: myproject
    space_key: PROJ
    local:
      markdown_dir: ./proj
  - name: other
    space_key: OTHER
    local:
      markdown_dir: ./other
`

func writePagesTestConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "cfg-*.yaml")
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

func resetPagesFlags() {
	pagesSpace = ""
	pagesParent = ""
	pagesProject = ""
	pagesShowPage = ""
	pagesShowDetails = false
}

func TestRunPages_MissingSpaceReturnsError(t *testing.T) {
	resetPagesFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigWithProjectsYAML)
	verbose = false

	err := runPages(pagesCmd, nil)
	if err == nil {
		t.Fatal("expected error for missing space")
	}
	if !strings.Contains(err.Error(), "space flag or --project required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPages_ProjectSelectionInfersSpace(t *testing.T) {
	resetPagesFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigWithProjectsYAML)
	verbose = false
	pagesProject = "myproject"

	mock := confluence.NewMockClient()
	mock.SpaceHierarchies["PROJ"] = []confluence.PageInfo{
		{ID: "1", Title: "Root Page", Children: nil},
	}
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
		return mock
	}

	outBuf := &bytes.Buffer{}
	rootCmd.SetOut(outBuf)

	err := runPages(pagesCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPages_ListsPagesSuccessfully(t *testing.T) {
	resetPagesFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigYAML)
	verbose = false
	pagesSpace = "DOCS"

	mock := confluence.NewMockClient()
	mock.SpaceHierarchies["DOCS"] = []confluence.PageInfo{
		{ID: "1", Title: "Page One", Children: nil},
		{ID: "2", Title: "Page Two", Children: []confluence.PageInfo{
			{ID: "3", Title: "Child Page", Children: nil},
		}},
	}
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
		return mock
	}

	err := runPages(pagesCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPages_WithParentFilter(t *testing.T) {
	resetPagesFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigYAML)
	verbose = false
	pagesSpace = "DOCS"
	pagesParent = "Parent Page"

	mock := confluence.NewMockClient()
	mock.SpaceHierarchies["DOCS"] = []confluence.PageInfo{
		{ID: "1", Title: "Parent Page", Children: []confluence.PageInfo{
			{ID: "2", Title: "Child A", Children: nil},
			{ID: "3", Title: "Child B", Children: nil},
		}},
	}
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
		return mock
	}

	err := runPages(pagesCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPagesShow_MissingSpaceReturnsError(t *testing.T) {
	resetPagesFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigWithProjectsYAML)
	verbose = false

	err := runPagesShow(pagesShowCmd, nil)
	if err == nil {
		t.Fatal("expected error for missing space")
	}
	if !strings.Contains(err.Error(), "space flag or --project required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPagesShow_ProjectSelectionInfersSpace(t *testing.T) {
	resetPagesFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigWithProjectsYAML)
	verbose = false
	pagesProject = "other"

	mock := confluence.NewMockClient()
	mock.SpaceHierarchies["OTHER"] = []confluence.PageInfo{
		{ID: "1", Title: "Root", Children: nil},
	}
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
		return mock
	}

	err := runPagesShow(pagesShowCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPagesShow_SpaceOverviewWhenNoPageSpecified(t *testing.T) {
	resetPagesFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigYAML)
	verbose = false
	pagesSpace = "DOCS"

	mock := confluence.NewMockClient()
	mock.SpaceHierarchies["DOCS"] = []confluence.PageInfo{
		{ID: "1", Title: "First", Children: nil},
		{ID: "2", Title: "Second", Children: nil},
	}
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
		return mock
	}

	err := runPagesShow(pagesShowCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPagesShow_FindsPageByNumericID(t *testing.T) {
	resetPagesFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigYAML)
	verbose = false
	pagesSpace = "DOCS"
	pagesShowPage = "12345"

	mock := confluence.NewMockClient()
	mock.Pages["12345"] = &confluence.Page{ID: "12345", Title: "Found Page"}
	mock.Ancestors["12345"] = []confluence.PageInfo{}
	mock.Children["12345"] = []confluence.PageInfo{}
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
		return mock
	}

	err := runPagesShow(pagesShowCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPagesShow_FindsPageByTitle(t *testing.T) {
	resetPagesFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigYAML)
	verbose = false
	pagesSpace = "DOCS"
	pagesShowPage = "My Page"

	mock := confluence.NewMockClient()
	page, _ := mock.CreatePage("DOCS", "My Page", "content")
	mock.Ancestors[page.ID] = []confluence.PageInfo{}
	mock.Children[page.ID] = []confluence.PageInfo{}
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
		return mock
	}

	err := runPagesShow(pagesShowCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPagesShow_PageNotFoundError(t *testing.T) {
	resetPagesFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigYAML)
	verbose = false
	pagesSpace = "DOCS"
	pagesShowPage = "NonExistent"

	mock := confluence.NewMockClient()
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
		return mock
	}

	err := runPagesShow(pagesShowCmd, nil)
	if err == nil {
		t.Fatal("expected error for page not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPagesShow_WithDetailsFlag(t *testing.T) {
	resetPagesFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigYAML)
	verbose = false
	pagesSpace = "DOCS"
	pagesShowPage = "Detail Page"
	pagesShowDetails = true

	mock := confluence.NewMockClient()
	page, _ := mock.CreatePage("DOCS", "Detail Page", "some content with ac:name=\"children\" macro")
	mock.Ancestors[page.ID] = []confluence.PageInfo{}
	mock.Children[page.ID] = []confluence.PageInfo{}
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
		return mock
	}

	err := runPagesShow(pagesShowCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShowSpaceOverview_EmptySpaceShowsNoPages(t *testing.T) {
	mock := confluence.NewMockClient()
	mock.SpaceHierarchies["EMPTY"] = []confluence.PageInfo{}

	err := showSpaceOverview(mock, "EMPTY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShowSpaceOverview_WithPagesShowsCountAndTree(t *testing.T) {
	mock := confluence.NewMockClient()
	mock.SpaceHierarchies["FULL"] = []confluence.PageInfo{
		{ID: "1", Title: "Root1", Children: []confluence.PageInfo{
			{ID: "2", Title: "Child1", Children: nil},
		}},
		{ID: "3", Title: "Root2", Children: nil},
	}

	err := showSpaceOverview(mock, "FULL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShowPageDetails_BasicOutput(t *testing.T) {
	mock := confluence.NewMockClient()
	page := &confluence.Page{ID: "123", Title: "Test Page"}
	mock.Ancestors["123"] = []confluence.PageInfo{}
	mock.Children["123"] = []confluence.PageInfo{}

	pagesShowDetails = false
	err := showPageDetails(mock, page, "TEST")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShowPageDetails_WithDetailsFlagShowsContentLength(t *testing.T) {
	mock := confluence.NewMockClient()
	page := &confluence.Page{ID: "456", Title: "Detailed Page"}
	page.Body.Storage.Value = "some content here"
	mock.Ancestors["456"] = []confluence.PageInfo{}
	mock.Children["456"] = []confluence.PageInfo{}

	pagesShowDetails = true
	err := showPageDetails(mock, page, "TEST")
	pagesShowDetails = false
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShowPageDetails_ShowsAncestorsParentChain(t *testing.T) {
	mock := confluence.NewMockClient()
	page := &confluence.Page{ID: "789", Title: "Child Page"}
	mock.Ancestors["789"] = []confluence.PageInfo{
		{ID: "1", Title: "Grandparent"},
		{ID: "2", Title: "Parent"},
	}
	mock.Children["789"] = []confluence.PageInfo{}

	err := showPageDetails(mock, page, "TEST")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShowPageDetails_ShowsChildren(t *testing.T) {
	mock := confluence.NewMockClient()
	page := &confluence.Page{ID: "abc", Title: "Parent Page"}
	mock.Ancestors["abc"] = []confluence.PageInfo{}
	mock.Children["abc"] = []confluence.PageInfo{
		{ID: "c1", Title: "Child One"},
		{ID: "c2", Title: "Child Two"},
		{ID: "c3", Title: "Child Three"},
	}

	err := showPageDetails(mock, page, "TEST")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrintPageTree_NestedStructure(t *testing.T) {
	pages := []confluence.PageInfo{
		{
			ID:    "1",
			Title: "Root",
			Children: []confluence.PageInfo{
				{
					ID:    "2",
					Title: "Level 1",
					Children: []confluence.PageInfo{
						{ID: "3", Title: "Level 2", Children: nil},
					},
				},
			},
		},
	}

	printPageTree(pages, 0, true)
}

func TestPrintPageTree_MultipleRoots(t *testing.T) {
	pages := []confluence.PageInfo{
		{ID: "1", Title: "Root A", Children: nil},
		{ID: "2", Title: "Root B", Children: nil},
		{ID: "3", Title: "Root C", Children: nil},
	}

	printPageTree(pages, 0, true)
}

func TestPrintPageTree_EmptySlice(t *testing.T) {
	pages := []confluence.PageInfo{}
	printPageTree(pages, 0, true)
}

func TestCountTotalPages_RecursiveCounting(t *testing.T) {
	pages := []confluence.PageInfo{
		{
			ID:    "1",
			Title: "Root",
			Children: []confluence.PageInfo{
				{
					ID:    "2",
					Title: "Child",
					Children: []confluence.PageInfo{
						{ID: "3", Title: "Grandchild", Children: nil},
					},
				},
				{ID: "4", Title: "Child 2", Children: nil},
			},
		},
		{ID: "5", Title: "Root 2", Children: nil},
	}

	total := countTotalPages(pages)
	if total != 5 {
		t.Fatalf("expected 5 total pages, got %d", total)
	}
}

func TestCountTotalPages_EmptySlice(t *testing.T) {
	pages := []confluence.PageInfo{}
	total := countTotalPages(pages)
	if total != 0 {
		t.Fatalf("expected 0 total pages, got %d", total)
	}
}

func TestCountTotalPages_SinglePage(t *testing.T) {
	pages := []confluence.PageInfo{
		{ID: "1", Title: "Only", Children: nil},
	}
	total := countTotalPages(pages)
	if total != 1 {
		t.Fatalf("expected 1 total page, got %d", total)
	}
}

func TestIsNumeric_ValidInputs(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"12345", true},
		{"0", true},
		{"999999999", true},
		{"-1", true},
		{"-999", true},
	}

	for _, tc := range tests {
		result := isNumeric(tc.input)
		if result != tc.expected {
			t.Errorf("isNumeric(%q) = %v, expected %v", tc.input, result, tc.expected)
		}
	}
}

func TestIsNumeric_InvalidInputs(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"abc", false},
		{"12.34", false},
		{"12abc", false},
		{"", false},
		{" 123", false},
		{"123 ", false},
		{"Page Title", false},
		{"ID-123", false},
	}

	for _, tc := range tests {
		result := isNumeric(tc.input)
		if result != tc.expected {
			t.Errorf("isNumeric(%q) = %v, expected %v", tc.input, result, tc.expected)
		}
	}
}

func TestRunPages_InvalidProjectReturnsError(t *testing.T) {
	resetPagesFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigWithProjectsYAML)
	verbose = false
	pagesProject = "nonexistent"

	err := runPages(pagesCmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid project")
	}
	if !strings.Contains(err.Error(), "failed to select project") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPagesShow_InvalidProjectReturnsError(t *testing.T) {
	resetPagesFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigWithProjectsYAML)
	verbose = false
	pagesProject = "nonexistent"

	err := runPagesShow(pagesShowCmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid project")
	}
	if !strings.Contains(err.Error(), "failed to select project") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPages_ExplicitSpaceOverridesProjectSpace(t *testing.T) {
	resetPagesFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigWithProjectsYAML)
	verbose = false
	pagesProject = "myproject"
	pagesSpace = "OVERRIDE"

	mock := confluence.NewMockClient()
	mock.SpaceHierarchies["OVERRIDE"] = []confluence.PageInfo{
		{ID: "1", Title: "Override Page", Children: nil},
	}
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
		return mock
	}

	err := runPages(pagesCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPages_InvalidConfigReturnsError(t *testing.T) {
	resetPagesFlags()
	tmp := t.TempDir()
	configFile = filepath.Join(tmp, "nonexistent.yaml")
	verbose = false

	err := runPages(pagesCmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
	if !strings.Contains(err.Error(), "failed to load config") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPagesShow_InvalidConfigReturnsError(t *testing.T) {
	resetPagesFlags()
	tmp := t.TempDir()
	configFile = filepath.Join(tmp, "nonexistent.yaml")
	verbose = false

	err := runPagesShow(pagesShowCmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
	if !strings.Contains(err.Error(), "failed to load config") {
		t.Fatalf("unexpected error: %v", err)
	}
}
