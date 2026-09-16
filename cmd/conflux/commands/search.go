package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"conflux/internal/config"
	"conflux/internal/confluence"
	"conflux/pkg/logger"
)

var (
	searchSpace       string
	searchProject     string
	searchAllSpaces   bool
	searchTitleOnly   bool
	searchMatchAny    bool
	searchLabels      []string
	searchAuthor      string
	searchContributor string
	searchAncestor    string
	searchSince       string
	searchType        string
	searchSort        string
	searchLimit       int
	searchCQL         string
	searchOutput      string
	searchShowExcerpt bool
)

var searchCmd = &cobra.Command{
	Use:   "search [keywords...]",
	Short: "Search Confluence pages by keyword",
	Long: `Search Confluence for pages matching keywords and other fields.

Keywords are matched against the full text of the page (title, body and
attachments) using the Confluence search API. Pass --title to match titles only.
Multiple keywords must all match by default; use --any to match any of them.
Quote an argument to search for a phrase.

Results can be narrowed further by space, label, author, contributor, ancestor
page, last-modified date and content type. For anything the flags do not cover,
--cql accepts a raw CQL query.

The search is limited to a single space, selected through --space, --project,
CONFLUX_SPACE_KEY, confluence.space_key, or the first configured project. Pass
--all-spaces to search every space instead, which needs no space configured.`,
	Example: `  conflux search deploy                          # Keyword search in the configured space
  conflux search robot fleet                     # Pages matching BOTH keywords
  conflux search robot fleet --any               # Pages matching EITHER keyword
  conflux search "battery charging"              # Phrase search
  conflux search deploy -s DOCS                  # Search a specific space
  conflux search deploy --all-spaces             # Search every space
  conflux search api --title                     # Match titles only
  conflux search --label runbook --since 7d      # Filter-only search, no keywords
  conflux search deploy --author jdoe --sort modified
  conflux search deploy --show-excerpt           # Include matched text snippets
  conflux search deploy -o json                  # Machine-readable output
  conflux search --cql 'type = "page" AND label = "adr"'`,
	RunE: runSearch,
}

func runSearch(cmd *cobra.Command, args []string) error {
	log := logger.New(verbose)

	if searchOutput != "text" && searchOutput != "json" {
		return fmt.Errorf("invalid output format %q: use text or json", searchOutput)
	}

	runtime, err := resolveSearchRuntime()
	if err != nil {
		return err
	}
	effectiveSpace := runtime.Confluence.SpaceKey

	opts := confluence.SearchOptions{
		Keywords:      args,
		MatchAny:      searchMatchAny,
		TitleOnly:     searchTitleOnly,
		SpaceKey:      effectiveSpace,
		Labels:        searchLabels,
		Creator:       searchAuthor,
		Contributor:   searchContributor,
		AncestorID:    searchAncestor,
		ContentType:   searchType,
		ModifiedSince: searchSince,
		OrderBy:       searchSort,
		Limit:         searchLimit,
		CQL:           searchCQL,
	}

	// Validate the query up front so bad flags are reported as flag errors
	// rather than as a search failure.
	if _, err := confluence.BuildCQL(opts); err != nil {
		return err
	}

	client := newConfluenceClient(runtime.Confluence.BaseURL, runtime.Confluence.Username, runtime.Confluence.APIToken, log)

	results, err := client.SearchPages(opts)
	if err != nil {
		return fmt.Errorf("failed to search Confluence: %w", err)
	}

	return renderSearchResults(os.Stdout, results, searchScope(effectiveSpace, searchCQL), searchOutput, searchShowExcerpt, verbose)
}

// searchScope describes, for display, what the search covered. A raw CQL query
// carries its own scope, so the resolved space is not mentioned there.
func searchScope(spaceKey, rawCQL string) string {
	switch {
	case strings.TrimSpace(rawCQL) != "":
		return "the given CQL query"
	case spaceKey != "":
		return "space '" + spaceKey + "'"
	default:
		return "all spaces"
	}
}

// resolveSearchRuntime resolves credentials and the space to search. Searching
// every space is a legitimate request, so --all-spaces skips the usual space
// resolution, which requires one.
func resolveSearchRuntime() (config.RuntimeConfig, error) {
	if !searchAllSpaces {
		return resolveRuntimeConfig(searchSpace, searchProject)
	}

	if searchSpace != "" || searchProject != "" {
		return config.RuntimeConfig{}, fmt.Errorf("--all-spaces cannot be combined with --space or --project")
	}
	cfg, err := config.LoadRuntime(configFile)
	if err != nil {
		return config.RuntimeConfig{}, fmt.Errorf("failed to load config: %w", err)
	}
	runtime := config.RuntimeConfig{Confluence: cfg.Confluence, SpaceSource: "--all-spaces"}
	runtime.Confluence.SpaceKey = ""
	return runtime, nil
}

// renderSearchResults writes search results as human-readable text or JSON.
func renderSearchResults(w io.Writer, results *confluence.SearchResults, scope, format string, showExcerpt, showCQL bool) error {
	if format == "json" {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(plainSearchResults(results)); err != nil {
			return fmt.Errorf("failed to encode results: %w", err)
		}
		return nil
	}

	if showCQL {
		fmt.Fprintf(w, "CQL: %s\n\n", results.CQL)
	}

	if len(results.Results) == 0 {
		fmt.Fprintf(w, "No matches in %s\n", scope)
		return nil
	}

	fmt.Fprintf(w, "%d match%s in %s\n\n", results.Total, pluralSuffix(results.Total), scope)

	for _, hit := range results.Results {
		// Space | Title | ID, skipping any field the API did not return.
		heading := make([]string, 0, 3)
		if hit.SpaceKey != "" {
			heading = append(heading, hit.SpaceKey)
		}
		heading = append(heading, hit.Title)
		if hit.ID != "" {
			heading = append(heading, hit.ID)
		}
		fmt.Fprintln(w, strings.Join(heading, " | "))

		if date := formatSearchDate(hit.LastModified); date != "" {
			fmt.Fprintf(w, "Last update: %s\n", date)
		}
		if hit.URL != "" {
			fmt.Fprintf(w, "URL: %s\n", hit.URL)
		}
		if showExcerpt {
			if excerpt := formatSearchExcerpt(hit.Excerpt); excerpt != "" {
				fmt.Fprintf(w, "Excerpt: %s\n", excerpt)
			}
		}
		fmt.Fprintln(w)
	}

	if results.Total > len(results.Results) {
		fmt.Fprintf(w, "Showing %d of %d - use --limit to see more\n", len(results.Results), results.Total)
	}

	return nil
}

// plainSearchResults returns a copy with excerpts reduced to plain text, so
// consumers of the JSON output never have to strip highlight markers.
func plainSearchResults(results *confluence.SearchResults) *confluence.SearchResults {
	plain := *results
	plain.Results = make([]confluence.SearchResult, len(results.Results))
	for i, hit := range results.Results {
		hit.Excerpt = strings.Join(strings.Fields(confluence.StripHighlights(hit.Excerpt)), " ")
		plain.Results[i] = hit
	}
	return &plain
}

// formatSearchExcerpt collapses whitespace and turns the search API's
// highlight markers into markdown-style emphasis.
func formatSearchExcerpt(excerpt string) string {
	marked := confluence.MarkHighlights(excerpt, "**", "**")
	return strings.Join(strings.Fields(marked), " ")
}

// formatSearchDate reduces an ISO-8601 timestamp to its date portion.
func formatSearchDate(timestamp string) string {
	if len(timestamp) < 10 {
		return timestamp
	}
	return timestamp[:10]
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "es"
}

func init() {
	rootCmd.AddCommand(searchCmd)

	searchCmd.Flags().StringVarP(&searchSpace, "space", "s", "", "Confluence space key to search (defaults to the configured space)")
	searchCmd.Flags().StringVarP(&searchProject, "project", "P", "", "Project name defined in config to infer space")
	searchCmd.Flags().BoolVar(&searchAllSpaces, "all-spaces", false, "Search every space instead of a single one")
	searchCmd.Flags().BoolVarP(&searchTitleOnly, "title", "t", false, "Match keywords against page titles only")
	searchCmd.Flags().BoolVar(&searchMatchAny, "any", false, "Match any keyword instead of requiring all of them")
	searchCmd.Flags().StringArrayVar(&searchLabels, "label", nil, "Only pages carrying this label (repeatable)")
	searchCmd.Flags().StringVar(&searchAuthor, "author", "", "Only pages created by this username or account id")
	searchCmd.Flags().StringVar(&searchContributor, "contributor", "", "Only pages this username or account id contributed to")
	searchCmd.Flags().StringVar(&searchAncestor, "ancestor", "", "Only pages below this ancestor page ID")
	searchCmd.Flags().StringVar(&searchSince, "since", "", "Only pages modified since YYYY-MM-DD or a duration (30m, 12h, 7d, 2w)")
	searchCmd.Flags().StringVar(&searchType, "type", "page", "Content type: page, blogpost, attachment or comment")
	searchCmd.Flags().StringVar(&searchSort, "sort", "relevance", "Sort order: relevance, modified, created or title")
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", confluence.DefaultSearchLimit, "Maximum number of results")
	searchCmd.Flags().StringVar(&searchCQL, "cql", "", "Raw CQL query (ignores keywords and other filters)")
	searchCmd.Flags().StringVarP(&searchOutput, "output", "o", "text", "Output format: text or json")
	searchCmd.Flags().BoolVar(&searchShowExcerpt, "show-excerpt", false, "Show a highlighted excerpt of the matched text")
}
