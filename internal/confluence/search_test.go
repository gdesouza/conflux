package confluence

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestBuildCQL_KeywordsDefaultToTextAndAnd(t *testing.T) {
	cql, err := BuildCQL(SearchOptions{Keywords: []string{"robot", "fleet"}, SpaceKey: "DOCS"})
	if err != nil {
		t.Fatalf("BuildCQL: %v", err)
	}
	want := `type = "page" AND space = "DOCS" AND (text ~ "robot" AND text ~ "fleet")`
	if cql != want {
		t.Fatalf("got %q, want %q", cql, want)
	}
}

func TestBuildCQL_MatchAnyUsesOr(t *testing.T) {
	cql, err := BuildCQL(SearchOptions{Keywords: []string{"robot", "fleet"}, MatchAny: true})
	if err != nil {
		t.Fatalf("BuildCQL: %v", err)
	}
	if !strings.Contains(cql, `(text ~ "robot" OR text ~ "fleet")`) {
		t.Fatalf("expected OR-joined keywords, got %q", cql)
	}
}

func TestBuildCQL_SingleKeywordIsNotParenthesized(t *testing.T) {
	cql, err := BuildCQL(SearchOptions{Keywords: []string{"robot"}})
	if err != nil {
		t.Fatalf("BuildCQL: %v", err)
	}
	if cql != `type = "page" AND text ~ "robot"` {
		t.Fatalf("unexpected CQL: %q", cql)
	}
}

func TestBuildCQL_TitleOnlySwitchesField(t *testing.T) {
	cql, err := BuildCQL(SearchOptions{Keywords: []string{"api"}, TitleOnly: true})
	if err != nil {
		t.Fatalf("BuildCQL: %v", err)
	}
	if !strings.Contains(cql, `title ~ "api"`) {
		t.Fatalf("expected title field, got %q", cql)
	}
}

func TestBuildCQL_BlankKeywordsAreDropped(t *testing.T) {
	cql, err := BuildCQL(SearchOptions{Keywords: []string{"  ", "robot", ""}})
	if err != nil {
		t.Fatalf("BuildCQL: %v", err)
	}
	if cql != `type = "page" AND text ~ "robot"` {
		t.Fatalf("unexpected CQL: %q", cql)
	}
}

func TestBuildCQL_EscapesQuotesAndBackslashes(t *testing.T) {
	cql, err := BuildCQL(SearchOptions{Keywords: []string{`say "hi"\n`}})
	if err != nil {
		t.Fatalf("BuildCQL: %v", err)
	}
	if !strings.Contains(cql, `text ~ "say \"hi\"\\n"`) {
		t.Fatalf("value not escaped: %q", cql)
	}
}

func TestBuildCQL_InjectionAttemptStaysInsideLiteral(t *testing.T) {
	cql, err := BuildCQL(SearchOptions{Keywords: []string{`x" OR type = "blogpost`}})
	if err != nil {
		t.Fatalf("BuildCQL: %v", err)
	}
	// The injected quotes must be escaped so the whole payload stays one literal.
	if cql != `type = "page" AND text ~ "x\" OR type = \"blogpost"` {
		t.Fatalf("injection not neutralized: %q", cql)
	}
	if strings.Count(cql, "text ~ ") != 1 {
		t.Fatalf("expected exactly one text clause: %q", cql)
	}
}

func TestBuildCQL_AllFilters(t *testing.T) {
	cql, err := BuildCQL(SearchOptions{
		Keywords:      []string{"deploy"},
		SpaceKey:      "DOCS",
		Labels:        []string{"runbook", "ops"},
		Creator:       "jdoe",
		Contributor:   "asmith",
		AncestorID:    "12345",
		ContentType:   "blogpost",
		ModifiedSince: "2026-01-02",
		OrderBy:       "modified",
	})
	if err != nil {
		t.Fatalf("BuildCQL: %v", err)
	}
	for _, want := range []string{
		`type = "blogpost"`,
		`space = "DOCS"`,
		`text ~ "deploy"`,
		`label = "runbook"`,
		`label = "ops"`,
		`creator = "jdoe"`,
		`contributor = "asmith"`,
		`ancestor = "12345"`,
		`lastmodified >= "2026-01-02"`,
	} {
		if !strings.Contains(cql, want) {
			t.Errorf("missing %q in %q", want, cql)
		}
	}
	if !strings.HasSuffix(cql, " order by lastmodified desc") {
		t.Errorf("missing order clause in %q", cql)
	}
}

func TestBuildCQL_RelativeSinceUsesNowFunction(t *testing.T) {
	cases := map[string]string{
		"7d":  `lastmodified >= now("-7d")`,
		"2w":  `lastmodified >= now("-2w")`,
		"12h": `lastmodified >= now("-12h")`,
		"30m": `lastmodified >= now("-30m")`,
	}
	for input, want := range cases {
		cql, err := BuildCQL(SearchOptions{ModifiedSince: input})
		if err != nil {
			t.Fatalf("BuildCQL(%q): %v", input, err)
		}
		if !strings.Contains(cql, want) {
			t.Errorf("BuildCQL(%q) = %q, want it to contain %q", input, cql, want)
		}
	}
}

func TestBuildCQL_InvalidSinceRejected(t *testing.T) {
	for _, input := range []string{"yesterday", "7y", "01-02-2026", "7 d"} {
		if _, err := BuildCQL(SearchOptions{ModifiedSince: input}); err == nil {
			t.Errorf("expected error for --since %q", input)
		}
	}
}

func TestBuildCQL_InvalidTypeAndSortRejected(t *testing.T) {
	if _, err := BuildCQL(SearchOptions{Keywords: []string{"x"}, ContentType: "widget"}); err == nil {
		t.Error("expected error for invalid content type")
	}
	if _, err := BuildCQL(SearchOptions{Keywords: []string{"x"}, OrderBy: "sideways"}); err == nil {
		t.Error("expected error for invalid sort")
	}
}

func TestBuildCQL_SortOrders(t *testing.T) {
	cases := map[string]string{
		"relevance": "",
		"":          "",
		"created":   " order by created desc",
		"title":     " order by title asc",
	}
	for sort, suffix := range cases {
		cql, err := BuildCQL(SearchOptions{Keywords: []string{"x"}, OrderBy: sort})
		if err != nil {
			t.Fatalf("BuildCQL(sort=%q): %v", sort, err)
		}
		if suffix == "" && strings.Contains(cql, "order by") {
			t.Errorf("sort %q should not add an order clause: %q", sort, cql)
		}
		if suffix != "" && !strings.HasSuffix(cql, suffix) {
			t.Errorf("sort %q = %q, want suffix %q", sort, cql, suffix)
		}
	}
}

func TestBuildCQL_RequiresNarrowingCriteria(t *testing.T) {
	if _, err := BuildCQL(SearchOptions{}); err == nil {
		t.Error("expected error with no criteria at all")
	}
	if _, err := BuildCQL(SearchOptions{SpaceKey: "DOCS"}); err == nil {
		t.Error("expected error when only a space is given")
	}
	// A label alone is a legitimate narrowing filter.
	if _, err := BuildCQL(SearchOptions{SpaceKey: "DOCS", Labels: []string{"adr"}}); err != nil {
		t.Errorf("label-only search should be allowed: %v", err)
	}
}

func TestBuildCQL_RawCQLOverridesEverything(t *testing.T) {
	raw := `type = "page" AND label = "adr"`
	cql, err := BuildCQL(SearchOptions{CQL: "  " + raw + "  ", Keywords: []string{"ignored"}, SpaceKey: "NOPE"})
	if err != nil {
		t.Fatalf("BuildCQL: %v", err)
	}
	if cql != raw {
		t.Fatalf("got %q, want %q", cql, raw)
	}
}

func TestHighlightHelpers(t *testing.T) {
	raw := "the " + highlightStart + "deploy" + highlightEnd + " step"
	if got := StripHighlights(raw); got != "the deploy step" {
		t.Errorf("StripHighlights = %q", got)
	}
	if got := MarkHighlights(raw, "**", "**"); got != "the **deploy** step" {
		t.Errorf("MarkHighlights = %q", got)
	}
}

func TestJoinSearchURL(t *testing.T) {
	cases := []struct{ base, ref, want string }{
		{"https://x.atlassian.net/wiki", "/spaces/DOCS/pages/1", "https://x.atlassian.net/wiki/spaces/DOCS/pages/1"},
		{"https://x.atlassian.net/wiki/", "spaces/DOCS/pages/1", "https://x.atlassian.net/wiki/spaces/DOCS/pages/1"},
		{"https://x.atlassian.net/wiki", "https://other/page", "https://other/page"},
		{"https://x.atlassian.net/wiki", "", ""},
	}
	for _, c := range cases {
		if got := joinSearchURL(c.base, c.ref); got != c.want {
			t.Errorf("joinSearchURL(%q, %q) = %q, want %q", c.base, c.ref, got, c.want)
		}
	}
}

// searchTestServer serves a canned search payload and records the queries it saw.
func searchTestServer(t *testing.T, handler func(q url.Values) (int, any)) (*Client, *[]url.Values) {
	t.Helper()
	var queries []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/search" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		queries = append(queries, r.URL.Query())
		status, body := handler(r.URL.Query())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if str, ok := body.(string); ok {
			fmt.Fprint(w, str)
			return
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(server.Close)
	return New(server.URL, "user", "token"), &queries
}

func TestSearchPages_ParsesResults(t *testing.T) {
	payload := map[string]any{
		"results": []map[string]any{{
			"content": map[string]any{
				"id":      "123",
				"type":    "page",
				"title":   "Deployment Guide",
				"space":   map[string]any{"key": "DOCS", "name": "Docs"},
				"version": map[string]any{"number": 4},
			},
			"title":                 highlightStart + "Deployment" + highlightEnd + " Guide",
			"excerpt":               "the " + highlightStart + "deploy" + highlightEnd + " step",
			"url":                   "/spaces/DOCS/pages/123",
			"lastModified":          "2026-08-12T10:04:00.000Z",
			"resultGlobalContainer": map[string]any{"title": "Docs", "displayUrl": "/spaces/DOCS"},
		}},
		"totalSize": 12,
		"_links":    map[string]any{"base": "https://x.atlassian.net/wiki"},
	}

	empty := map[string]any{"results": []map[string]any{}, "totalSize": 12}
	client, queries := searchTestServer(t, func(q url.Values) (int, any) {
		if q.Get("start") != "0" {
			return http.StatusOK, empty
		}
		return http.StatusOK, payload
	})

	results, err := client.SearchPages(SearchOptions{Keywords: []string{"deploy"}, SpaceKey: "DOCS", Limit: 10})
	if err != nil {
		t.Fatalf("SearchPages: %v", err)
	}

	if results.Total != 12 {
		t.Errorf("Total = %d, want 12", results.Total)
	}
	if len(results.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(results.Results))
	}
	hit := results.Results[0]
	if hit.ID != "123" || hit.Type != "page" {
		t.Errorf("unexpected id/type: %+v", hit)
	}
	// The clean content title wins over the highlighted one.
	if hit.Title != "Deployment Guide" {
		t.Errorf("Title = %q", hit.Title)
	}
	if hit.SpaceKey != "DOCS" || hit.SpaceName != "Docs" {
		t.Errorf("unexpected space: %+v", hit)
	}
	if hit.Version != 4 {
		t.Errorf("Version = %d, want 4", hit.Version)
	}
	if hit.URL != "https://x.atlassian.net/wiki/spaces/DOCS/pages/123" {
		t.Errorf("URL = %q", hit.URL)
	}
	// Excerpt keeps its highlight markers for the caller to render.
	if !strings.Contains(hit.Excerpt, highlightStart) {
		t.Errorf("expected highlight markers in excerpt %q", hit.Excerpt)
	}

	q := (*queries)[0]
	if q.Get("cql") != `type = "page" AND space = "DOCS" AND text ~ "deploy"` {
		t.Errorf("unexpected cql: %q", q.Get("cql"))
	}
	if q.Get("excerpt") != "highlight" {
		t.Errorf("excerpt param = %q", q.Get("excerpt"))
	}
	if q.Get("expand") != "content.space,content.version" {
		t.Errorf("expand param = %q", q.Get("expand"))
	}
}

func TestSearchPages_FallsBackToResultTitleAndContainerSpace(t *testing.T) {
	payload := map[string]any{
		"results": []map[string]any{{
			"title":                 highlightStart + "Orphan" + highlightEnd + " Page",
			"url":                   "/spaces/OPS/pages/9",
			"resultGlobalContainer": map[string]any{"title": "Ops", "displayUrl": "/spaces/OPS/overview"},
		}},
		"totalSize": 1,
	}
	client, _ := searchTestServer(t, func(url.Values) (int, any) { return http.StatusOK, payload })

	results, err := client.SearchPages(SearchOptions{Keywords: []string{"orphan"}})
	if err != nil {
		t.Fatalf("SearchPages: %v", err)
	}
	hit := results.Results[0]
	if hit.Title != "Orphan Page" {
		t.Errorf("Title = %q, want markers stripped", hit.Title)
	}
	if hit.SpaceKey != "OPS" {
		t.Errorf("SpaceKey = %q, want it derived from displayUrl", hit.SpaceKey)
	}
	// Without _links.base the client falls back to its own base URL.
	if !strings.HasSuffix(hit.URL, "/spaces/OPS/pages/9") || !strings.HasPrefix(hit.URL, "http://127.0.0.1") {
		t.Errorf("URL = %q", hit.URL)
	}
}

func TestSearchPages_PaginatesUntilLimit(t *testing.T) {
	const total = 120
	client, queries := searchTestServer(t, func(q url.Values) (int, any) {
		start, _ := strconv.Atoi(q.Get("start"))
		limit, _ := strconv.Atoi(q.Get("limit"))
		var hits []map[string]any
		for i := 0; i < limit && start+i < total; i++ {
			hits = append(hits, map[string]any{
				"content": map[string]any{"id": strconv.Itoa(start + i), "type": "page", "title": "P"},
			})
		}
		return http.StatusOK, map[string]any{"results": hits, "totalSize": total}
	})

	results, err := client.SearchPages(SearchOptions{Keywords: []string{"x"}, Limit: 60})
	if err != nil {
		t.Fatalf("SearchPages: %v", err)
	}
	if len(results.Results) != 60 {
		t.Fatalf("got %d results, want 60", len(results.Results))
	}
	if results.Total != total {
		t.Errorf("Total = %d, want %d", results.Total, total)
	}
	// 60 results with a 50-per-request cap means two round trips.
	if len(*queries) != 2 {
		t.Fatalf("made %d requests, want 2", len(*queries))
	}
	if got := (*queries)[0].Get("limit"); got != "50" {
		t.Errorf("first page limit = %q, want 50", got)
	}
	if got := (*queries)[1].Get("start"); got != "50" {
		t.Errorf("second page start = %q, want 50", got)
	}
	if got := (*queries)[1].Get("limit"); got != "10" {
		t.Errorf("second page limit = %q, want 10", got)
	}
	if results.Results[59].ID != "59" {
		t.Errorf("last result id = %q, want 59", results.Results[59].ID)
	}
}

func TestSearchPages_StopsWhenResultsExhausted(t *testing.T) {
	client, queries := searchTestServer(t, func(q url.Values) (int, any) {
		start, _ := strconv.Atoi(q.Get("start"))
		if start > 0 {
			return http.StatusOK, map[string]any{"results": []map[string]any{}, "totalSize": 999}
		}
		return http.StatusOK, map[string]any{
			"results":   []map[string]any{{"content": map[string]any{"id": "1", "title": "One"}}},
			"totalSize": 999, // server over-reports; the empty page must still stop us
		}
	})

	results, err := client.SearchPages(SearchOptions{Keywords: []string{"x"}, Limit: 100})
	if err != nil {
		t.Fatalf("SearchPages: %v", err)
	}
	if len(results.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(results.Results))
	}
	if len(*queries) != 2 {
		t.Errorf("made %d requests, want 2", len(*queries))
	}
}

func TestSearchPages_DefaultLimitApplied(t *testing.T) {
	client, queries := searchTestServer(t, func(url.Values) (int, any) {
		return http.StatusOK, map[string]any{"results": []map[string]any{}, "totalSize": 0}
	})
	if _, err := client.SearchPages(SearchOptions{Keywords: []string{"x"}}); err != nil {
		t.Fatalf("SearchPages: %v", err)
	}
	if got := (*queries)[0].Get("limit"); got != strconv.Itoa(DefaultSearchLimit) {
		t.Errorf("limit = %q, want %d", got, DefaultSearchLimit)
	}
}

func TestSearchPages_NoMatchesReturnsEmptySlice(t *testing.T) {
	client, _ := searchTestServer(t, func(url.Values) (int, any) {
		return http.StatusOK, map[string]any{"results": []map[string]any{}, "totalSize": 0}
	})
	results, err := client.SearchPages(SearchOptions{Keywords: []string{"nothing"}})
	if err != nil {
		t.Fatalf("SearchPages: %v", err)
	}
	if results.Results == nil {
		t.Error("Results should be an empty slice, not nil, so JSON renders as []")
	}
	if len(results.Results) != 0 || results.Total != 0 {
		t.Errorf("unexpected results: %+v", results)
	}
}

func TestSearchPages_APIErrorIncludesStatusAndBody(t *testing.T) {
	client, _ := searchTestServer(t, func(url.Values) (int, any) {
		return http.StatusBadRequest, `{"message":"Could not parse cql"}`
	})
	_, err := client.SearchPages(SearchOptions{Keywords: []string{"x"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "Could not parse cql") {
		t.Errorf("unhelpful error: %v", err)
	}
}

func TestSearchPages_InvalidOptionsFailBeforeRequest(t *testing.T) {
	client, queries := searchTestServer(t, func(url.Values) (int, any) {
		return http.StatusOK, map[string]any{}
	})
	if _, err := client.SearchPages(SearchOptions{}); err == nil {
		t.Fatal("expected error for empty criteria")
	}
	if len(*queries) != 0 {
		t.Errorf("made %d requests, want none", len(*queries))
	}
}
