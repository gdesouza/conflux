package confluence

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Markers the Confluence search API wraps around matched terms when the
// excerpt strategy is "highlight".
const (
	highlightStart = "@@@hl@@@"
	highlightEnd   = "@@@endhl@@@"
)

const (
	// DefaultSearchLimit is the number of hits returned when no limit is given.
	DefaultSearchLimit = 25
	// maxSearchPageSize is the largest page Confluence will serve per request.
	maxSearchPageSize = 50
)

// SearchOptions describes a keyword search that is translated into CQL.
type SearchOptions struct {
	Keywords      []string // free-text terms; a term containing spaces is matched as a phrase
	MatchAny      bool     // OR the keywords together instead of AND
	TitleOnly     bool     // match keywords against the title instead of the full text
	SpaceKey      string   // restrict to a single space
	Labels        []string // every label must be present
	Creator       string   // username or account id of the page creator
	Contributor   string   // username or account id of any contributor
	AncestorID    string   // restrict to descendants of this page id
	ContentType   string   // page, blogpost, attachment, comment (default: page)
	ModifiedSince string   // YYYY-MM-DD or a relative duration such as 7d, 2w, 12h, 30m
	OrderBy       string   // relevance (default), modified, created, title
	Limit         int      // maximum hits to return
	CQL           string   // raw CQL, bypassing everything above
}

// SearchResult is a single hit returned by the Confluence search API.
type SearchResult struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Title        string `json:"title"`
	SpaceKey     string `json:"spaceKey,omitempty"`
	SpaceName    string `json:"spaceName,omitempty"`
	Excerpt      string `json:"excerpt,omitempty"`
	URL          string `json:"url,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	Version      int    `json:"version,omitempty"`
}

// SearchResults holds the hits for a search, the total number of matches
// Confluence reported, and the CQL that produced them.
type SearchResults struct {
	CQL     string         `json:"cql"`
	Total   int            `json:"total"`
	Results []SearchResult `json:"results"`
}

var (
	absoluteDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}(\s+\d{2}:\d{2})?$`)
	relativeDatePattern = regexp.MustCompile(`^(\d+)([mhdw])$`)
	// Confluence exposes a space key in container URLs as /spaces/<KEY>.
	spaceKeyFromURLPattern = regexp.MustCompile(`/spaces/([^/]+)`)
)

// validContentTypes are the content types the search command accepts.
var validContentTypes = map[string]bool{
	"page":       true,
	"blogpost":   true,
	"attachment": true,
	"comment":    true,
}

// quoteCQLValue renders s as a quoted CQL string literal.
func quoteCQLValue(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + replacer.Replace(s) + `"`
}

// normalizeSince converts a user-supplied date filter into a CQL date value.
// Absolute dates are quoted as-is; relative durations become now("-7d").
func normalizeSince(since string) (string, error) {
	trimmed := strings.TrimSpace(since)
	if absoluteDatePattern.MatchString(trimmed) {
		return quoteCQLValue(trimmed), nil
	}
	if match := relativeDatePattern.FindStringSubmatch(trimmed); match != nil {
		return fmt.Sprintf(`now(%s)`, quoteCQLValue("-"+match[1]+match[2])), nil
	}
	return "", fmt.Errorf("invalid date %q: use YYYY-MM-DD or a duration such as 30m, 12h, 7d, 2w", since)
}

// orderByClause maps the user-facing sort name onto a CQL order by clause.
func orderByClause(orderBy string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(orderBy)) {
	case "", "relevance":
		return "", nil
	case "modified":
		return " order by lastmodified desc", nil
	case "created":
		return " order by created desc", nil
	case "title":
		return " order by title asc", nil
	default:
		return "", fmt.Errorf("invalid sort %q: use relevance, modified, created or title", orderBy)
	}
}

// BuildCQL turns SearchOptions into the CQL query sent to Confluence.
func BuildCQL(opts SearchOptions) (string, error) {
	if raw := strings.TrimSpace(opts.CQL); raw != "" {
		return raw, nil
	}

	contentType := strings.ToLower(strings.TrimSpace(opts.ContentType))
	if contentType == "" {
		contentType = "page"
	}
	if !validContentTypes[contentType] {
		return "", fmt.Errorf("invalid content type %q: use page, blogpost, attachment or comment", contentType)
	}

	clauses := []string{"type = " + quoteCQLValue(contentType)}

	if space := strings.TrimSpace(opts.SpaceKey); space != "" {
		clauses = append(clauses, "space = "+quoteCQLValue(space))
	}

	field := "text"
	if opts.TitleOnly {
		field = "title"
	}
	var keywordClauses []string
	for _, keyword := range opts.Keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		keywordClauses = append(keywordClauses, field+" ~ "+quoteCQLValue(keyword))
	}
	if len(keywordClauses) > 0 {
		separator := " AND "
		if opts.MatchAny {
			separator = " OR "
		}
		joined := strings.Join(keywordClauses, separator)
		if len(keywordClauses) > 1 {
			joined = "(" + joined + ")"
		}
		clauses = append(clauses, joined)
	}

	for _, label := range opts.Labels {
		if label = strings.TrimSpace(label); label != "" {
			clauses = append(clauses, "label = "+quoteCQLValue(label))
		}
	}
	if creator := strings.TrimSpace(opts.Creator); creator != "" {
		clauses = append(clauses, "creator = "+quoteCQLValue(creator))
	}
	if contributor := strings.TrimSpace(opts.Contributor); contributor != "" {
		clauses = append(clauses, "contributor = "+quoteCQLValue(contributor))
	}
	if ancestor := strings.TrimSpace(opts.AncestorID); ancestor != "" {
		clauses = append(clauses, "ancestor = "+quoteCQLValue(ancestor))
	}
	if since := strings.TrimSpace(opts.ModifiedSince); since != "" {
		value, err := normalizeSince(since)
		if err != nil {
			return "", err
		}
		clauses = append(clauses, "lastmodified >= "+value)
	}

	// A space on its own would just list the whole space, which is what
	// `conflux pages` is for, so require a narrowing filter.
	if len(clauses) < 2 || (len(clauses) == 2 && strings.HasPrefix(clauses[1], "space = ")) {
		return "", fmt.Errorf("no search criteria: provide keywords or a filter such as --label, --author or --since")
	}

	order, err := orderByClause(opts.OrderBy)
	if err != nil {
		return "", err
	}

	return strings.Join(clauses, " AND ") + order, nil
}

// StripHighlights removes the search API's highlight markers from text.
func StripHighlights(text string) string {
	return strings.NewReplacer(highlightStart, "", highlightEnd, "").Replace(text)
}

// MarkHighlights replaces the search API's highlight markers with start and
// end delimiters chosen by the caller.
func MarkHighlights(text, start, end string) string {
	return strings.NewReplacer(highlightStart, start, highlightEnd, end).Replace(text)
}

// searchHit mirrors one entry in the results of GET /rest/api/search.
type searchHit struct {
	Content struct {
		ID    string `json:"id"`
		Type  string `json:"type"`
		Title string `json:"title"`
		Space struct {
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"space"`
		Version struct {
			Number int `json:"number"`
		} `json:"version"`
	} `json:"content"`
	Title                 string `json:"title"`
	Excerpt               string `json:"excerpt"`
	URL                   string `json:"url"`
	LastModified          string `json:"lastModified"`
	ResultGlobalContainer struct {
		Title      string `json:"title"`
		DisplayURL string `json:"displayUrl"`
	} `json:"resultGlobalContainer"`
}

// searchResponse mirrors the payload of GET /rest/api/search.
type searchResponse struct {
	Results   []searchHit `json:"results"`
	Start     int         `json:"start"`
	Limit     int         `json:"limit"`
	Size      int         `json:"size"`
	TotalSize int         `json:"totalSize"`
	Links     struct {
		Base string `json:"base"`
	} `json:"_links"`
}

// SearchPages runs a CQL search and returns up to opts.Limit hits, paging
// through the Confluence search API as needed.
func (c *Client) SearchPages(opts SearchOptions) (*SearchResults, error) {
	cql, err := BuildCQL(opts)
	if err != nil {
		return nil, err
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultSearchLimit
	}

	if c.logger != nil {
		c.logger.Debug("Confluence search CQL: %s", cql)
	}

	results := &SearchResults{CQL: cql, Results: []SearchResult{}}

	for start := 0; len(results.Results) < limit; {
		pageSize := min(limit-len(results.Results), maxSearchPageSize)

		payload, err := c.searchPage(cql, start, pageSize)
		if err != nil {
			return nil, err
		}

		results.Total = payload.TotalSize
		base := payload.Links.Base
		if base == "" {
			base = c.baseURL
		}

		for _, hit := range payload.Results {
			results.Results = append(results.Results, newSearchResult(hit, base))
		}

		if len(payload.Results) == 0 {
			break
		}
		start += len(payload.Results)
		if payload.TotalSize > 0 && start >= payload.TotalSize {
			break
		}
	}

	if len(results.Results) > limit {
		results.Results = results.Results[:limit]
	}
	if results.Total < len(results.Results) {
		results.Total = len(results.Results)
	}

	return results, nil
}

// searchPage fetches a single page of search results.
func (c *Client) searchPage(cql string, start, limit int) (*searchResponse, error) {
	params := url.Values{}
	params.Set("cql", cql)
	params.Set("start", strconv.Itoa(start))
	params.Set("limit", strconv.Itoa(limit))
	params.Set("expand", "content.space,content.version")
	params.Set("excerpt", "highlight")

	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/rest/api/search?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	resp, err := c.doAuthenticated(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil, fmt.Errorf("search failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return &payload, nil
}

// newSearchResult flattens one raw search hit into a SearchResult.
func newSearchResult(hit searchHit, base string) SearchResult {
	// The top-level title carries highlight markers; the content title does not.
	title := hit.Content.Title
	if title == "" {
		title = StripHighlights(hit.Title)
	}

	spaceKey := hit.Content.Space.Key
	if spaceKey == "" {
		if match := spaceKeyFromURLPattern.FindStringSubmatch(hit.ResultGlobalContainer.DisplayURL); match != nil {
			spaceKey = match[1]
		}
	}

	return SearchResult{
		ID:           hit.Content.ID,
		Type:         hit.Content.Type,
		Title:        title,
		SpaceKey:     spaceKey,
		SpaceName:    hit.ResultGlobalContainer.Title,
		Excerpt:      strings.TrimSpace(hit.Excerpt),
		URL:          joinSearchURL(base, hit.URL),
		LastModified: hit.LastModified,
		Version:      hit.Content.Version.Number,
	}
}

// joinSearchURL resolves the relative URL a search hit carries against the
// base URL Confluence reports for the result set.
func joinSearchURL(base, reference string) string {
	if reference == "" {
		return ""
	}
	if strings.HasPrefix(reference, "http://") || strings.HasPrefix(reference, "https://") {
		return reference
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(reference, "/")
}
