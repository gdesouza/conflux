package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"conflux/internal/confluence"
	"conflux/pkg/logger"
)

var errTestSearchFailed = errors.New("configured search failure")

func resetSearchFlags() {
	searchSpace = ""
	searchProject = ""
	searchAllSpaces = false
	searchTitleOnly = false
	searchMatchAny = false
	searchLabels = nil
	searchAuthor = ""
	searchContributor = ""
	searchAncestor = ""
	searchSince = ""
	searchType = "page"
	searchSort = "relevance"
	searchLimit = confluence.DefaultSearchLimit
	searchCQL = ""
	searchOutput = "text"
	searchShowExcerpt = false
}

// useSearchMock installs a mock client for the duration of the test.
func useSearchMock(t *testing.T, mock *confluence.MockClient) {
	t.Helper()
	original := newConfluenceClient
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
		return mock
	}
	t.Cleanup(func() { newConfluenceClient = original })
}

func TestResolveSearchRuntime_UsesConfiguredSpaceByDefault(t *testing.T) {
	resetSearchFlags()
	t.Setenv("CONFLUX_SPACE_KEY", "")
	configFile = writePagesTestConfig(t, pagesTestConfigYAML)

	runtime, err := resolveSearchRuntime()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runtime.Confluence.SpaceKey != "DOCS" {
		t.Fatalf("space = %q, want DOCS", runtime.Confluence.SpaceKey)
	}
	if runtime.Confluence.BaseURL != "http://example" {
		t.Errorf("credentials not carried through: %+v", runtime.Confluence)
	}
}

func TestResolveSearchRuntime_FlagOverridesConfig(t *testing.T) {
	resetSearchFlags()
	t.Setenv("CONFLUX_SPACE_KEY", "")
	searchSpace = "OPS"
	configFile = writePagesTestConfig(t, pagesTestConfigYAML)

	runtime, err := resolveSearchRuntime()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runtime.Confluence.SpaceKey != "OPS" {
		t.Fatalf("space = %q, want OPS", runtime.Confluence.SpaceKey)
	}
}

func TestResolveSearchRuntime_AllSpacesClearsSpace(t *testing.T) {
	resetSearchFlags()
	t.Setenv("CONFLUX_SPACE_KEY", "")
	searchAllSpaces = true
	configFile = writePagesTestConfig(t, pagesTestConfigYAML)

	runtime, err := resolveSearchRuntime()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runtime.Confluence.SpaceKey != "" {
		t.Fatalf("space = %q, want empty", runtime.Confluence.SpaceKey)
	}
	// Credentials must survive the space being cleared.
	if runtime.Confluence.BaseURL != "http://example" || runtime.Confluence.APIToken != "t" {
		t.Errorf("credentials lost: %+v", runtime.Confluence)
	}
}

func TestResolveSearchRuntime_AllSpacesWorksWithoutAnyConfiguredSpace(t *testing.T) {
	resetSearchFlags()
	t.Setenv("CONFLUX_SPACE_KEY", "")
	searchAllSpaces = true
	// No space_key and no profiles: resolving a single space would fail here.
	configFile = writePagesTestConfig(t, `confluence:
  base_url: http://example
  username: u
  api_token: t
`)

	runtime, err := resolveSearchRuntime()
	if err != nil {
		t.Fatalf("--all-spaces must not require a resolvable space: %v", err)
	}
	if runtime.Confluence.SpaceKey != "" {
		t.Fatalf("space = %q, want empty", runtime.Confluence.SpaceKey)
	}
}

func TestResolveSearchRuntime_AllSpacesConflictsWithSpace(t *testing.T) {
	resetSearchFlags()
	searchAllSpaces = true
	searchSpace = "DOCS"
	configFile = writePagesTestConfig(t, pagesTestConfigYAML)

	if _, err := resolveSearchRuntime(); err == nil {
		t.Fatal("expected error combining --all-spaces with --space")
	}
}

func TestResolveSearchRuntime_ProjectInfersSpace(t *testing.T) {
	resetSearchFlags()
	t.Setenv("CONFLUX_SPACE_KEY", "")
	searchProject = "myproject"
	configFile = writePagesTestConfig(t, pagesTestConfigWithProjectsYAML)

	runtime, err := resolveSearchRuntime()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runtime.Confluence.SpaceKey != "PROJ" {
		t.Fatalf("space = %q, want PROJ", runtime.Confluence.SpaceKey)
	}
}

func TestResolveSearchRuntime_UnknownProjectErrors(t *testing.T) {
	resetSearchFlags()
	searchProject = "nope"
	configFile = writePagesTestConfig(t, pagesTestConfigWithProjectsYAML)

	if _, err := resolveSearchRuntime(); err == nil {
		t.Fatal("expected error for unknown project")
	}
}

func TestSearchScope(t *testing.T) {
	cases := []struct{ space, cql, want string }{
		{"DOCS", "", "space 'DOCS'"},
		{"", "", "all spaces"},
		{"DOCS", `type = "page"`, "the given CQL query"},
	}
	for _, c := range cases {
		if got := searchScope(c.space, c.cql); got != c.want {
			t.Errorf("searchScope(%q, %q) = %q, want %q", c.space, c.cql, got, c.want)
		}
	}
}

func sampleSearchResults() *confluence.SearchResults {
	return &confluence.SearchResults{
		CQL:   `type = "page" AND text ~ "deploy"`,
		Total: 12,
		Results: []confluence.SearchResult{{
			ID:           "123",
			Type:         "page",
			Title:        "Deployment Guide",
			SpaceKey:     "DOCS",
			Excerpt:      "the @@@hl@@@deploy@@@endhl@@@\n  step runs",
			URL:          "https://x.atlassian.net/wiki/spaces/DOCS/pages/123",
			LastModified: "2026-08-12T10:04:00.000Z",
		}},
	}
}

func TestRenderSearchResults_Text(t *testing.T) {
	var out bytes.Buffer
	if err := renderSearchResults(&out, sampleSearchResults(), "space 'DOCS'", "text", false, false); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := out.String()

	for _, want := range []string{
		"12 matches in space 'DOCS'",
		"DOCS | Deployment Guide | 123",
		"Last update: 2026-08-12",
		"URL: https://x.atlassian.net/wiki/spaces/DOCS/pages/123",
		"Showing 1 of 12",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	// Excerpts are opt-in via --show-excerpt.
	if strings.Contains(got, "Excerpt:") || strings.Contains(got, "step runs") {
		t.Errorf("excerpt should be hidden by default:\n%s", got)
	}
	if strings.Contains(got, "@@@hl@@@") {
		t.Errorf("highlight markers leaked into output:\n%s", got)
	}
	if strings.Contains(got, "CQL:") {
		t.Errorf("CQL should be hidden unless verbose:\n%s", got)
	}
}

func TestRenderSearchResults_TextOmitsMissingFields(t *testing.T) {
	results := &confluence.SearchResults{
		Total:   1,
		Results: []confluence.SearchResult{{Title: "Untitled Space Page"}},
	}

	var out bytes.Buffer
	if err := renderSearchResults(&out, results, "all spaces", "text", false, false); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Untitled Space Page") {
		t.Errorf("title missing:\n%s", got)
	}
	// No empty separators, and no labels for fields the API did not return.
	if strings.Contains(got, "|") {
		t.Errorf("heading should not carry empty fields:\n%s", got)
	}
	if strings.Contains(got, "Last update:") || strings.Contains(got, "URL:") {
		t.Errorf("labels for missing fields should be skipped:\n%s", got)
	}
}

func TestRenderSearchResults_TextShowsCQLWhenVerbose(t *testing.T) {
	var out bytes.Buffer
	if err := renderSearchResults(&out, sampleSearchResults(), "space 'DOCS'", "text", false, true); err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out.String(), `CQL: type = "page" AND text ~ "deploy"`) {
		t.Errorf("expected CQL line:\n%s", out.String())
	}
}

func TestRenderSearchResults_ShowExcerptAddsSnippet(t *testing.T) {
	var out bytes.Buffer
	if err := renderSearchResults(&out, sampleSearchResults(), "space 'DOCS'", "text", true, false); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Excerpt: the **deploy** step runs") {
		t.Errorf("expected highlighted excerpt:\n%s", got)
	}
	if strings.Contains(got, "@@@hl@@@") {
		t.Errorf("highlight markers leaked into output:\n%s", got)
	}
}

func TestRenderSearchResults_SingularAndNoTruncationNotice(t *testing.T) {
	results := sampleSearchResults()
	results.Total = 1

	var out bytes.Buffer
	if err := renderSearchResults(&out, results, "all spaces", "text", false, false); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "1 match in all spaces") {
		t.Errorf("expected singular wording:\n%s", got)
	}
	if strings.Contains(got, "Showing") {
		t.Errorf("no truncation notice expected:\n%s", got)
	}
}

func TestRenderSearchResults_Empty(t *testing.T) {
	var out bytes.Buffer
	empty := &confluence.SearchResults{CQL: "x", Results: []confluence.SearchResult{}}
	if err := renderSearchResults(&out, empty, "space 'DOCS'", "text", false, false); err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out.String(), "No matches in space 'DOCS'") {
		t.Errorf("unexpected output:\n%s", out.String())
	}
}

func TestRenderSearchResults_JSON(t *testing.T) {
	var out bytes.Buffer
	if err := renderSearchResults(&out, sampleSearchResults(), "space 'DOCS'", "json", false, false); err != nil {
		t.Fatalf("render: %v", err)
	}

	var decoded confluence.SearchResults
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if decoded.Total != 12 || len(decoded.Results) != 1 {
		t.Fatalf("unexpected decoded payload: %+v", decoded)
	}
	if decoded.Results[0].ID != "123" || decoded.Results[0].Title != "Deployment Guide" {
		t.Errorf("unexpected hit: %+v", decoded.Results[0])
	}
	if decoded.CQL != `type = "page" AND text ~ "deploy"` {
		t.Errorf("CQL = %q", decoded.CQL)
	}
}

func TestRunSearch_PassesFlagsThroughToClient(t *testing.T) {
	resetSearchFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigYAML)
	verbose = false
	searchTitleOnly = true
	searchMatchAny = true
	searchLabels = []string{"runbook"}
	searchAuthor = "jdoe"
	searchSince = "7d"
	searchSort = "modified"
	searchLimit = 5

	mock := confluence.NewMockClient()
	mock.SearchHits = []confluence.SearchResult{{ID: "1", Title: "Hit", SpaceKey: "DOCS"}}
	useSearchMock(t, mock)

	if err := runSearch(searchCmd, []string{"robot", "fleet"}); err != nil {
		t.Fatalf("runSearch: %v", err)
	}

	opts := mock.LastSearchOpts
	if len(opts.Keywords) != 2 || opts.Keywords[0] != "robot" {
		t.Errorf("keywords = %v", opts.Keywords)
	}
	if !opts.TitleOnly || !opts.MatchAny {
		t.Errorf("title/any flags not forwarded: %+v", opts)
	}
	// space_key from the config is the default scope.
	if opts.SpaceKey != "DOCS" {
		t.Errorf("SpaceKey = %q, want DOCS", opts.SpaceKey)
	}
	if len(opts.Labels) != 1 || opts.Labels[0] != "runbook" {
		t.Errorf("Labels = %v", opts.Labels)
	}
	if opts.Creator != "jdoe" || opts.ModifiedSince != "7d" || opts.OrderBy != "modified" {
		t.Errorf("filters not forwarded: %+v", opts)
	}
	if opts.Limit != 5 {
		t.Errorf("Limit = %d, want 5", opts.Limit)
	}
}

func TestRunSearch_InvalidOutputFormatRejected(t *testing.T) {
	resetSearchFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigYAML)
	searchOutput = "yaml"
	useSearchMock(t, confluence.NewMockClient())

	err := runSearch(searchCmd, []string{"robot"})
	if err == nil {
		t.Fatal("expected error for invalid output format")
	}
	if !strings.Contains(err.Error(), "invalid output format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSearch_NoCriteriaReturnsError(t *testing.T) {
	resetSearchFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigYAML)
	useSearchMock(t, confluence.NewMockClient())

	err := runSearch(searchCmd, nil)
	if err == nil {
		t.Fatal("expected error when no keywords or filters are given")
	}
	if !strings.Contains(err.Error(), "no search criteria") {
		t.Fatalf("unexpected error: %v", err)
	}
	// A bad query is a flag error, not a search failure.
	if strings.Contains(err.Error(), "failed to search Confluence") {
		t.Fatalf("query errors should not be reported as search failures: %v", err)
	}
}

func TestRunSearch_InvalidQueryFlagsReportedAsFlagErrors(t *testing.T) {
	cases := map[string]func(){
		"invalid sort":         func() { searchSort = "sideways" },
		"invalid date":         func() { searchSince = "yesterday" },
		"invalid content type": func() { searchType = "widget" },
	}
	for wantMsg, apply := range cases {
		resetSearchFlags()
		configFile = writePagesTestConfig(t, pagesTestConfigYAML)
		apply()

		mock := confluence.NewMockClient()
		useSearchMock(t, mock)

		err := runSearch(searchCmd, []string{"robot"})
		if err == nil {
			t.Fatalf("expected error for %s", wantMsg)
		}
		if !strings.Contains(err.Error(), wantMsg) {
			t.Errorf("error %q should mention %q", err, wantMsg)
		}
		if strings.Contains(err.Error(), "failed to search Confluence") {
			t.Errorf("%s should not be reported as a search failure: %v", wantMsg, err)
		}
		// The client must not be contacted with an invalid query.
		if mock.LastSearchOpts.Limit != 0 {
			t.Errorf("%s: client was called despite invalid query", wantMsg)
		}
	}
}

func TestRunSearch_SurfacesClientError(t *testing.T) {
	resetSearchFlags()
	configFile = writePagesTestConfig(t, pagesTestConfigYAML)

	mock := confluence.NewMockClient()
	mock.SearchErr = errTestSearchFailed
	useSearchMock(t, mock)

	err := runSearch(searchCmd, []string{"robot"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to search Confluence") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFormatSearchDate(t *testing.T) {
	cases := map[string]string{
		"2026-08-12T10:04:00.000Z": "2026-08-12",
		"2026-08-12":               "2026-08-12",
		"":                         "",
		"bad":                      "bad",
	}
	for input, want := range cases {
		if got := formatSearchDate(input); got != want {
			t.Errorf("formatSearchDate(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRenderSearchResults_JSONExcerptIsPlainText(t *testing.T) {
	var out bytes.Buffer
	results := sampleSearchResults()
	if err := renderSearchResults(&out, results, "space 'DOCS'", "json", false, false); err != nil {
		t.Fatalf("render: %v", err)
	}

	var decoded confluence.SearchResults
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got := decoded.Results[0].Excerpt; got != "the deploy step runs" {
		t.Errorf("Excerpt = %q, want markers stripped and whitespace collapsed", got)
	}
	// The caller's results must not be mutated by rendering.
	if !strings.Contains(results.Results[0].Excerpt, "@@@hl@@@") {
		t.Error("rendering must not mutate the original results")
	}
}
